//go:build windows

package nodelib

import (
	"encoding/binary"
	"fmt"
	"unicode"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows版の実装は user32!SendInput を直接呼ぶ（CGOを使わない）。
// 修飾キーなしの文字は KEYEVENTF_UNICODE で送るため、日本語などの
// 非ASCII文字もキーコード変換なしで入力できる。
// ただし同時押しのときはVK_PACKET（Unicode入力）ではショートカットとして
// 届かないので、仮想キーコードへ変換して送る。
const (
	inputTypeKeyboard = 1
	keyEventFKeyUp    = 0x0002
	keyEventFUnicode  = 0x0004
)

var (
	procSendInput = windows.NewLazySystemDLL("user32.dll").NewProc("SendInput")
	// procVkKeyScan は文字から現在のキーボード配列の仮想キーコードを引くAPI。
	procVkKeyScan = windows.NewLazySystemDLL("user32.dll").NewProc("VkKeyScanW")
)

// virtual key codes (Winuser.h)
const (
	vkBack     = 0x08
	vkTab      = 0x09
	vkReturn   = 0x0D
	vkShift    = 0x10
	vkControl  = 0x11
	vkMenu     = 0x12 // Alt
	vkPause    = 0x13
	vkCapital  = 0x14
	vkEscape   = 0x1B
	vkSpace    = 0x20
	vkPrior    = 0x21 // PageUp
	vkNext     = 0x22 // PageDown
	vkEnd      = 0x23
	vkHome     = 0x24
	vkLeft     = 0x25
	vkUp       = 0x26
	vkRight    = 0x27
	vkDown     = 0x28
	vkSnapshot = 0x2C // PrintScreen
	vkInsert   = 0x2D
	vkDelete   = 0x2E
	vkHelp     = 0x2F
	vkLWin     = 0x5B
	vkRWin     = 0x5C
	vkNumLock  = 0x90
	vkScroll   = 0x91
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLMenu    = 0xA4
	vkRMenu    = 0xA5
	vkCancel   = 0x03 // Ctrl+Break
	vkF1       = 0x70 // F1〜F16 は連番
	vkF16      = 0x7F
)

// windowsKeyCodes は論理キー名から仮想キーコードへの変換表。
var windowsKeyCodes = map[string]uint16{
	keyNameEnter:       vkReturn,
	keyNameTab:         vkTab,
	keyNameEscape:      vkEscape,
	keyNameBackspace:   vkBack,
	keyNameDelete:      vkDelete,
	keyNameInsert:      vkInsert,
	keyNameHome:        vkHome,
	keyNameEnd:         vkEnd,
	keyNamePageUp:      vkPrior,
	keyNamePageDown:    vkNext,
	keyNameUp:          vkUp,
	keyNameDown:        vkDown,
	keyNameLeft:        vkLeft,
	keyNameRight:       vkRight,
	keyNameSpace:       vkSpace,
	keyNameCapsLock:    vkCapital,
	keyNameNumLock:     vkNumLock,
	keyNameScrollLock:  vkScroll,
	keyNamePrintScreen: vkSnapshot,
	keyNamePause:       vkPause,
	keyNameBreak:       vkCancel,
	keyNameHelp:        vkHelp,
	keyNameCtrl:        vkControl,
	keyNameRCtrl:       vkRControl,
	keyNameAlt:         vkMenu,
	keyNameRAlt:        vkRMenu,
	keyNameShift:       vkShift,
	keyNameRShift:      vkRShift,
	keyNameWin:         vkLWin,
}

// windowsModifierKeys は同時押しする修飾キーの仮想キーコード（押す順）。
func windowsModifierKeys(mods keyMods) []uint16 {
	var keys []uint16
	if mods&modCtrl != 0 {
		keys = append(keys, vkControl)
	}
	if mods&modAlt != 0 {
		keys = append(keys, vkMenu)
	}
	if mods&modShift != 0 {
		keys = append(keys, vkShift)
	}
	if mods&modWin != 0 {
		keys = append(keys, vkLWin)
	}
	return keys
}

// sendKeyStrokes はストローク列を SendInput で順番に送信する。
func sendKeyStrokes(strokes []keyStroke) error {
	return forEachStroke(strokes, sendKeyStroke)
}

// held は {CTRL DOWN} などで既に物理的に押されている修飾キー。
// ここで押し直すと最後の解放で解除されてしまうため、新しく押す分だけを処理する。
func sendKeyStroke(st keyStroke, held keyMods) error {
	press := windowsModifierKeys(st.mods &^ held)
	// 文字を仮想キーへ変換する経路の判定には保持中の修飾キーも含める。
	// Unicode(VK_PACKET)のままではショートカットとして届かないため。
	effective := st.mods | held
	switch st.mode {
	case keyHoldMode:
		return sendInputKeys(keysDown(press, modifierOrKeyCode(st)))
	case keyUpMode:
		return sendInputKeys(keysUp(press, modifierOrKeyCode(st)))
	default:
		events := vkDownEvents(press)
		switch {
		case st.name != "":
			code, ok := windowsStrokeCode(st)
			if !ok {
				return fmt.Errorf("この環境では送信できないキーです: 『%s』", st.name)
			}
			events = append(events, vkEvent(code, false), vkEvent(code, true))
		case effective != 0:
			combos, err := windowsCharCombos(st.text, press)
			if err != nil {
				return err
			}
			events = append(events, combos...)
		default:
			events = append(events, unicodeEvents(st.text)...)
		}
		events = append(events, vkUpEvents(press)...)
		return sendInputKeys(events)
	}
}

// releaseOSHeldModifiers は保持中の修飾キーを解放する（ベストエフォート）。
func releaseOSHeldModifiers(held keyMods) {
	codes := windowsModifierKeys(held)
	if len(codes) == 0 {
		return
	}
	var ups []input
	for _, code := range codes {
		ups = append(ups, vkEvent(code, true))
	}
	// 片付けに失敗しても呼び出し元では元のエラーを返すので、結果は見ない。
	_ = sendInputKeys(ups)
}

// windowsCharCombos は同時押しする文字を仮想キーの押下・解放イベント列へ変換する。
// press はこのストロークで新しく押す修飾キー（保持中は含まない）。
func windowsCharCombos(text string, press []uint16) ([]input, error) {
	var events []input
	for _, r := range text {
		code, extra, ok := windowsCharToVK(r)
		if !ok {
			return nil, fmt.Errorf("修飾キーとの同時押しでは送信できない文字です: 『%c』", r)
		}
		// VkKeyScanW が返す追加の修飾子(Shiftなど)はここで押す必要がある
		keys := windowsModifierKeys(extra)
		for _, code := range press {
			if !containsVK(keys, code) {
				keys = append(keys, code)
			}
		}
		events = append(events, keysDown(keys, code)...)
		events = append(events, keysUp(keys, code)...)
	}
	return events, nil
}

// containsVK は仮想キーコードの一覧に code が含まれるかを返す。
func containsVK(codes []uint16, code uint16) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// windowsCharToVK は文字を現在のキーボード配列の仮想キーコードへ変換する。
// 戻り値の追加修飾子は、その文字を打つのに必要なShiftなどの同時押し。
// VkKeyScanW が使えないときはUS配列の表にフォールバックする。
func windowsCharToVK(r rune) (uint16, keyMods, bool) {
	if units := utf16.Encode([]rune{r}); len(units) > 0 {
		ret, _, _ := procVkKeyScan.Call(uintptr(units[0]))
		if scan := int16(ret); scan != -1 {
			vk := uint16(uint8(scan & 0xFF))
			if vk != 0 {
				var extra keyMods
				if state := uint8(scan >> 8); state != 0 {
					if state&1 != 0 {
						extra |= modShift
					}
					if state&2 != 0 {
						extra |= modCtrl
					}
					if state&4 != 0 {
						extra |= modAlt
					}
				}
				return vk, extra, true
			}
		}
	}
	if code, ok := windowsUSCharCodes[unicode.ToLower(r)]; ok {
		return code, modShiftIfUpper(r), true
	}
	return 0, 0, false
}

// modShiftIfUpper は大文字を打つために必要なShiftの同時押しを返す。
func modShiftIfUpper(r rune) keyMods {
	if unicode.IsUpper(r) {
		return modShift
	}
	return 0
}

// windowsUSCharCodes はUS配列の英数字・記号の仮想キーコード表。
// VkKeyScanW が使えない環境のためのフォールバック。英数字は '0'〜'Z' が連番。
var windowsUSCharCodes = map[rune]uint16{
	'0': 0x30, '1': 0x31, '2': 0x32, '3': 0x33, '4': 0x34,
	'5': 0x35, '6': 0x36, '7': 0x37, '8': 0x38, '9': 0x39,
	'a': 0x41, 'b': 0x42, 'c': 0x43, 'd': 0x44, 'e': 0x45, 'f': 0x46,
	'g': 0x47, 'h': 0x48, 'i': 0x49, 'j': 0x4A, 'k': 0x4B, 'l': 0x4C,
	'm': 0x4D, 'n': 0x4E, 'o': 0x4F, 'p': 0x50, 'q': 0x51, 'r': 0x52,
	's': 0x53, 't': 0x54, 'u': 0x55, 'v': 0x56, 'w': 0x57, 'x': 0x58,
	'y': 0x59, 'z': 0x5A,
	';': 0xBA, '=': 0xBB, ',': 0xBC, '-': 0xBD, '.': 0xBE, '/': 0xBF,
	'`': 0xC0, '[': 0xDB, '\\': 0xDC, ']': 0xDD, '\'': 0xDE,
}

// windowsStrokeCode はストロークを仮想キーコードへ変換する。
// 名前付きキーでなければ false を返す（＝ユニコード入力で送る）。
func windowsStrokeCode(st keyStroke) (uint16, bool) {
	if st.name != "" {
		if code, ok := windowsKeyCodes[st.name]; ok {
			return code, true
		}
		// F1〜F16 は連番なので表を持たずに計算する
		if n, ok := functionKeyNumber(st.name); ok {
			return vkF1 + uint16(n-1), true
		}
		return 0, false
	}
	return 0, false
}

// modifierOrKeyCode は {CTRL DOWN} のような単体イベントの仮想キーコードを返す。
func modifierOrKeyCode(st keyStroke) uint16 {
	if code, ok := windowsStrokeCode(st); ok {
		return code
	}
	if keys := windowsModifierKeys(st.mods); len(keys) > 0 {
		return keys[0]
	}
	return 0
}

// unicodeEvents は文字列を KEYEVENTF_UNICODE の押下・解放イベント列へ変換する。
// サロゲートペアはWindows側で1文字として扱われるため、2単位続けて送る。
func unicodeEvents(text string) []input {
	var events []input
	for _, u := range utf16.Encode([]rune(text)) {
		events = append(events,
			input{typ: inputTypeKeyboard, scan: u, flags: keyEventFUnicode},
			input{typ: inputTypeKeyboard, scan: u, flags: keyEventFUnicode | keyEventFKeyUp},
		)
	}
	return events
}

// vkDownEvents は仮想キーコードを押すイベント列を作る。
func vkDownEvents(codes []uint16) []input {
	var events []input
	for _, c := range codes {
		events = append(events, vkEvent(c, false))
	}
	return events
}

// vkUpEvents は仮想キーコードを離すイベント列を作る。
func vkUpEvents(codes []uint16) []input {
	var events []input
	for _, c := range codes {
		events = append(events, vkEvent(c, true))
	}
	return events
}

// vkEvent は仮想キーコード1つの押下・解放イベントを作る。
func vkEvent(code uint16, up bool) input {
	in := input{typ: inputTypeKeyboard, vk: code}
	if up {
		in.flags = keyEventFKeyUp
	}
	return in
}

// keysDown は修飾キーと対象キーをまとめて押すイベント列を作る。
func keysDown(mods []uint16, code uint16) []input {
	return append(vkDownEvents(mods), vkEvent(code, false))
}

// keysUp は修飾キーと対象キーをまとめて離すイベント列を作る。
func keysUp(mods []uint16, code uint16) []input {
	return append(vkUpEvents(mods), vkEvent(code, true))
}

// input は SendInput に渡す INPUT 構造体（共用体は KEYBDINPUT を使う）。
type input struct {
	typ   uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	extra uintptr
}

// inputSize は INPUT 構造体のサイズ。共用体は最大の MOUSEINPUT と同じ幅に
// 揃える必要があるため、ポインタ幅から計算する。
//
//	x64: type(4)+パディング(4)+MOUSEINPUT(32) = 40
//	x86: type(4)+MOUSEINPUT(24)             = 28
var inputSize = func() int {
	ptr := int(unsafe.Sizeof(uintptr(0)))
	roundUp := func(v, a int) int { return (v + a - 1) / a * a }
	// MOUSEINPUT: LONG×2 + DWORD×3 + ULONG_PTR
	union := roundUp(20, ptr) + ptr
	return ptr + union
}()

// bytes は INPUT 構造体のバイト列を作る（リトルエンディアン）。
func (in input) bytes() []byte {
	ptr := int(unsafe.Sizeof(uintptr(0)))
	roundUp := func(v, a int) int { return (v + a - 1) / a * a }
	buf := make([]byte, inputSize)
	o := 0
	put32 := func(v uint32) {
		binary.LittleEndian.PutUint32(buf[o:], v)
		o += 4
	}
	put16 := func(v uint16) {
		binary.LittleEndian.PutUint16(buf[o:], v)
		o += 2
	}
	putPtr := func(v uintptr) {
		for i := 0; i < ptr; i++ {
			buf[o+i] = byte(v >> (8 * i))
		}
		o += ptr
	}
	put32(in.typ)
	o = ptr // 共用体はポインタ幅の境界から始まる
	put16(in.vk)
	put16(in.scan)
	put32(in.flags)
	put32(in.time)
	o = ptr + roundUp(12, ptr) // dwExtraInfo のアライメント
	putPtr(in.extra)
	return buf
}

// sendInputKeys は INPUT 配列をひとまとめで送信する。
func sendInputKeys(events []input) error {
	if len(events) == 0 {
		return nil
	}
	buf := make([]byte, 0, len(events)*inputSize)
	for _, e := range events {
		buf = append(buf, e.bytes()...)
	}
	ret, _, err := procSendInput.Call(
		uintptr(len(events)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(inputSize))
	// SendInput の戻り値は挿入できたイベント数。押下だけが届いて解放が欠けると
	// 修飾キーが押しっぱなしになるため、部分送信はエラーとして扱う。
	if ret != uintptr(len(events)) {
		releaseStuckKeys(events)
		if ret == 0 {
			return fmt.Errorf("キーの送信に失敗しました: %w", err)
		}
		return fmt.Errorf("キーの送信に失敗しました（%d/%d件）: %w", ret, len(events), err)
	}
	return nil
}

// releaseStuckKeys は押したまま取り残された修飾キーを解放する（ベストエフォート）。
func releaseStuckKeys(events []input) {
	var ups []input
	for _, e := range events {
		if e.flags&keyEventFKeyUp != 0 || !isWindowsModifierVK(e.vk) {
			continue
		}
		ups = append(ups, vkEvent(e.vk, true))
	}
	if len(ups) == 0 {
		return
	}
	// 片付けに失敗しても呼び出し元へはエラーを返すので、結果は見ない。
	buf := make([]byte, 0, len(ups)*inputSize)
	for _, e := range ups {
		buf = append(buf, e.bytes()...)
	}
	_, _, _ = procSendInput.Call(
		uintptr(len(ups)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(inputSize))
}

// isWindowsModifierVK は修飾キーの仮想キーコードかを返す。
func isWindowsModifierVK(vk uint16) bool {
	switch vk {
	case vkShift, vkControl, vkMenu, vkLWin, vkRWin, vkLShift, vkRShift, vkLControl, vkRControl, vkLMenu, vkRMenu:
		return true
	}
	return false
}
