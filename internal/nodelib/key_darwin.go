//go:build darwin

package nodelib

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
)

// macOS版は purego で CoreGraphics の CGEvent API を直接呼ぶ。
// golang.design/x/clipboard が同じく purego で AppKit を呼んでいるのに合わせ、
// CGOを使わずに実現している（配布用ビルドが CGO_ENABLED=0 のため）。
// CoreGraphics が使えない環境では osascript による System Events へフォールバックする。
const (
	cgHIDEventTap  = 0 // kCGHIDEventTap
	coreGraphics   = "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics"
	coreFoundation = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
)

// CGEventFlags（CGEventTypes.h）。修飾キーの同時押しはフラグで表現する。
const (
	cgEventFlagMaskShift     uint64 = 0x00020000
	cgEventFlagMaskControl   uint64 = 0x00040000
	cgEventFlagMaskAlternate uint64 = 0x00080000
	cgEventFlagMaskCommand   uint64 = 0x00100000
)

// macOSの仮想キーコード（Carbon HIToolbox Events.h の kVK_* 相当）。
const (
	kVKReturn       uint16 = 0x24
	kVKTab          uint16 = 0x30
	kVKSpace        uint16 = 0x31
	kVKDelete       uint16 = 0x33 // Backspace
	kVKEscape       uint16 = 0x35
	kVKCommand      uint16 = 0x37
	kVKShift        uint16 = 0x38
	kVKCapsLock     uint16 = 0x39
	kVKOption       uint16 = 0x3A
	kVKControl      uint16 = 0x3B
	kVKRightShift   uint16 = 0x3C
	kVKRightOption  uint16 = 0x3D
	kVKRightControl uint16 = 0x3E
	kVKHelp         uint16 = 0x72
	kVKHome         uint16 = 0x73
	kVKPageUp       uint16 = 0x74
	kVKForwardDel   uint16 = 0x75
	kVKEnd          uint16 = 0x77
	kVKPageDown     uint16 = 0x79
	kVKLeftArrow    uint16 = 0x7B
	kVKRightArrow   uint16 = 0x7C
	kVKDownArrow    uint16 = 0x7D
	kVKUpArrow      uint16 = 0x7E
)

// macOSFunctionKeyBase は F17 以降を除くファンクションキーのコード表。
var macOSFunctionKeyCodes = map[int]uint16{
	1: 0x7A, 2: 0x78, 3: 0x63, 4: 0x76, 5: 0x60, 6: 0x61, 7: 0x62, 8: 0x64,
	9: 0x65, 10: 0x6D, 11: 0x67, 12: 0x6F, 13: 0x69, 14: 0x6B, 15: 0x71, 16: 0x6A,
}

// macOSKeyCodes は論理キー名からmacOSの仮想キーコードへの変換表。
var macOSKeyCodes = map[string]uint16{
	keyNameEnter:     kVKReturn,
	keyNameTab:       kVKTab,
	keyNameEscape:    kVKEscape,
	keyNameBackspace: kVKDelete,
	keyNameDelete:    kVKForwardDel,
	keyNameHome:      kVKHome,
	keyNameEnd:       kVKEnd,
	keyNamePageUp:    kVKPageUp,
	keyNamePageDown:  kVKPageDown,
	keyNameUp:        kVKUpArrow,
	keyNameDown:      kVKDownArrow,
	keyNameLeft:      kVKLeftArrow,
	keyNameRight:     kVKRightArrow,
	keyNameSpace:     kVKSpace,
	keyNameCapsLock:  kVKCapsLock,
	keyNameHelp:      kVKHelp,
	keyNameCtrl:      kVKControl,
	keyNameRCtrl:     kVKRightControl,
	keyNameAlt:       kVKOption,
	keyNameRAlt:      kVKRightOption,
	keyNameShift:     kVKShift,
	keyNameRShift:    kVKRightShift,
	keyNameWin:       kVKCommand, // Windowsキーの代わりにCommandキー
}

// macOSCharKeyCodes は修飾キーと同時押しする文字の仮想キーコード表（US配列）。
// 修飾キー付きの入力はキーコードで組まないとショートカットとして届かないため。
var macOSCharKeyCodes = map[rune]uint16{
	'a': 0x00, 's': 0x01, 'd': 0x02, 'f': 0x03, 'h': 0x04, 'g': 0x05, 'z': 0x06,
	'x': 0x07, 'c': 0x08, 'v': 0x09, 'b': 0x0B, 'q': 0x0C, 'w': 0x0D, 'e': 0x0E,
	'r': 0x0F, 'y': 0x10, 't': 0x11, '1': 0x12, '2': 0x13, '3': 0x14, '4': 0x15,
	'6': 0x16, '5': 0x17, '=': 0x18, '9': 0x19, '7': 0x1A, '-': 0x1B, '8': 0x1C,
	'0': 0x1D, ']': 0x1E, 'o': 0x1F, 'u': 0x20, '[': 0x21, 'i': 0x22, 'p': 0x23,
	'l': 0x25, 'j': 0x26, '\'': 0x27, 'k': 0x28, ';': 0x29, '\\': 0x2A, ',': 0x2B,
	'/': 0x2C, 'n': 0x2D, 'm': 0x2E, '.': 0x2F, '`': 0x32,
}

// darwinKeyFuncs は CoreGraphics から動的に取り出した関数群。
type darwinKeyFuncs struct {
	cgEventCreateKeyboardEvent      func(source uintptr, keyCode uint16, keyDown bool) uintptr
	cgEventKeyboardSetUnicodeString func(event uintptr, length uintptr, text uintptr)
	cgEventSetFlags                 func(event uintptr, flags uint64)
	cgEventPost                     func(tap uint32, event uintptr)
	cfRelease                       func(ref uintptr)
	// cgPreflightPostEventAccess はmacOS 10.14以降でないと存在しないため任意。
	cgPreflightPostEventAccess func() bool
}

var darwinKeys struct {
	once  sync.Once
	err   error
	funcs darwinKeyFuncs
}

// darwinSetup はCoreGraphicsの関数を一度だけ取り出す。
func darwinSetup() error {
	darwinKeys.once.Do(func() {
		darwinKeys.err = setupDarwinKeyFuncs()
	})
	return darwinKeys.err
}

func setupDarwinKeyFuncs() error {
	f := &darwinKeys.funcs
	cg, err := purego.Dlopen(coreGraphics, purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return fmt.Errorf("CoreGraphicsを開けません: %w", err)
	}
	cf, err := purego.Dlopen(coreFoundation, purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		return fmt.Errorf("CoreFoundationを開けません: %w", err)
	}
	if err := bindFunc(&f.cgEventCreateKeyboardEvent, cg, "CGEventCreateKeyboardEvent"); err != nil {
		return err
	}
	if err := bindFunc(&f.cgEventKeyboardSetUnicodeString, cg, "CGEventKeyboardSetUnicodeString"); err != nil {
		return err
	}
	if err := bindFunc(&f.cgEventSetFlags, cg, "CGEventSetFlags"); err != nil {
		return err
	}
	if err := bindFunc(&f.cgEventPost, cg, "CGEventPost"); err != nil {
		return err
	}
	if err := bindFunc(&f.cfRelease, cf, "CFRelease"); err != nil {
		return err
	}
	// アクセシビリティ権限の確認は任意（無い環境では確認を飛ばす）
	_ = bindFunc(&f.cgPreflightPostEventAccess, cg, "CGPreflightPostEventAccess")
	return nil
}

// bindFunc は handle から name の関数を取り出して fptr に結び付ける。
func bindFunc(fptr any, handle uintptr, name string) error {
	sym, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("%s が見つかりません: %w", name, err)
	}
	purego.RegisterFunc(fptr, sym)
	return nil
}

// macOSAccessibilityGuide は権限がないときの案内文。
const macOSAccessibilityGuide = "システム設定の「プライバシーとセキュリティ」→「アクセシビリティ」で、このプログラムを許可してください"

// sendKeyStrokes は CGEvent でキー操作を送る。使えない環境はosascriptへ回す。
func sendKeyStrokes(strokes []keyStroke) error {
	if err := darwinSetup(); err != nil {
		asErr := sendKeysViaAppleScript(strokes)
		if asErr == nil {
			return nil
		}
		return fmt.Errorf("%w（AppleScriptでも失敗しました: %v）", err, asErr)
	}
	f := &darwinKeys.funcs
	if f.cgPreflightPostEventAccess != nil && !f.cgPreflightPostEventAccess() {
		// 初回実行などAccessibility権限が未許可のときが、実際にいちばん多い
		// 失敗経路。System Eventsは別の権限(Automation)で動くことがあるため、
		// CGEventを諦めて先にこちらを試す。
		asErr := sendKeysViaAppleScript(strokes)
		if asErr == nil {
			return nil
		}
		return fmt.Errorf("%s（AppleScriptでも失敗しました: %v）", macOSAccessibilityGuide, asErr)
	}
	for _, st := range strokes {
		for i := 0; i < st.repeat; i++ {
			if err := postKeyStroke(st); err != nil {
				return err
			}
		}
	}
	return nil
}

// darwinKeyTarget は送信する1キー（キーコード、またはユニコード文字列）。
type darwinKeyTarget struct {
	code uint16
	uni  []uint16
}

func postKeyStroke(st keyStroke) error {
	targets := macOSTargets(st)
	if len(targets) == 0 {
		return fmt.Errorf("送信できないキーです: 『%s%s』", st.name, st.text)
	}
	flags := macOSFlags(st.mods)
	modKeys := macOSModifierKeys(st.mods)
	switch st.mode {
	case keyHoldMode:
		pressModifierKeys(modKeys, true)
		for _, t := range targets {
			postKeyEvent(t, flags, true)
		}
	case keyUpMode:
		for _, t := range targets {
			postKeyEvent(t, flags, false)
		}
		pressModifierKeys(modKeys, false)
	default:
		pressModifierKeys(modKeys, true)
		for _, t := range targets {
			postKeyEvent(t, flags, true)
			postKeyEvent(t, flags, false)
		}
		pressModifierKeys(modKeys, false)
	}
	return nil
}

// macOSTargets はストロークを送信対象のキー列へ変換する。
func macOSTargets(st keyStroke) []darwinKeyTarget {
	var targets []darwinKeyTarget
	if st.name != "" {
		if code, ok := macOSKeyCode(st.name); ok {
			targets = append(targets, darwinKeyTarget{code: code})
		}
		return targets
	}
	for _, r := range st.text {
		// 修飾キー付きはショートカットとして届くようキーコードで組む。
		if st.mods != 0 {
			lower := r
			if r >= 'A' && r <= 'Z' {
				lower = r - 'A' + 'a'
			}
			if code, ok := macOSCharKeyCodes[lower]; ok {
				targets = append(targets, darwinKeyTarget{code: code})
				continue
			}
		}
		targets = append(targets, darwinKeyTarget{uni: utf16.Encode([]rune{r})})
	}
	return targets
}

// macOSKeyCode は論理キー名をmacOSの仮想キーコードへ変換する。
func macOSKeyCode(name string) (uint16, bool) {
	if code, ok := macOSKeyCodes[name]; ok {
		return code, true
	}
	if n, ok := functionKeyNumber(name); ok {
		if code, ok := macOSFunctionKeyCodes[n]; ok {
			return code, true
		}
	}
	return 0, false
}

// macOSModifierKeys は同時押しする修飾キーの仮想キーコード（押す順）。
func macOSModifierKeys(mods keyMods) []uint16 {
	var keys []uint16
	if mods&modCtrl != 0 {
		keys = append(keys, kVKControl)
	}
	if mods&modAlt != 0 {
		keys = append(keys, kVKOption)
	}
	if mods&modShift != 0 {
		keys = append(keys, kVKShift)
	}
	if mods&modWin != 0 {
		keys = append(keys, kVKCommand)
	}
	return keys
}

// macOSFlags は同時押しの修飾キーをCGEventFlagsへ変換する。
func macOSFlags(mods keyMods) uint64 {
	var flags uint64
	if mods&modShift != 0 {
		flags |= cgEventFlagMaskShift
	}
	if mods&modCtrl != 0 {
		flags |= cgEventFlagMaskControl
	}
	if mods&modAlt != 0 {
		flags |= cgEventFlagMaskAlternate
	}
	if mods&modWin != 0 {
		flags |= cgEventFlagMaskCommand
	}
	return flags
}

// postKeyEvent は1つのキーイベントを作ってHIDイベントタップへ流す。
func postKeyEvent(t darwinKeyTarget, flags uint64, down bool) {
	f := &darwinKeys.funcs
	ev := f.cgEventCreateKeyboardEvent(0, t.code, down)
	if ev == 0 {
		return
	}
	if len(t.uni) > 0 {
		f.cgEventKeyboardSetUnicodeString(ev, uintptr(len(t.uni)), uintptr(unsafe.Pointer(&t.uni[0])))
	}
	f.cgEventSetFlags(ev, flags)
	f.cgEventPost(cgHIDEventTap, ev)
	f.cfRelease(ev)
}

// pressModifierKeys は修飾キーそのものを押下・解放する。
func pressModifierKeys(codes []uint16, down bool) {
	for _, code := range codes {
		postKeyEvent(darwinKeyTarget{code: code}, 0, down)
	}
}

// --- osascript（System Events）によるフォールバック ---

// sendKeysViaAppleScript は System Events の keystroke / key code でキーを送る。
// CGEventが使えない環境や権限が無い環境向けの経路。低速だが外部依存なしで動く。
func sendKeysViaAppleScript(strokes []keyStroke) error {
	script, err := appleScript(strokes)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascriptによるキー送信に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// appleScript はストローク列から AppleScript のスクリプトを組み立てる。
// 実行は sendKeysViaAppleScript が行う（テストでは生成結果だけを確認する）。
func appleScript(strokes []keyStroke) (string, error) {
	var lines []string
	for _, st := range strokes {
		for i := 0; i < st.repeat; i++ {
			line, err := appleScriptLine(st)
			if err != nil {
				return "", err
			}
			lines = append(lines, "  "+line)
		}
	}
	if len(lines) == 0 {
		return "", errors.New("送信するキーがありません")
	}
	return "tell application \"System Events\"\n" + strings.Join(lines, "\n") + "\nend tell", nil
}

func appleScriptLine(st keyStroke) (string, error) {
	using := appleScriptUsing(st.mods)
	switch {
	case st.name != "":
		code, ok := macOSKeyCode(st.name)
		if !ok {
			return "", fmt.Errorf("この環境では送信できないキーです: 『%s』", st.name)
		}
		if st.mode != keyTapMode {
			return "", fmt.Errorf("AppleScriptではこのキーに対応していません: 『%s』", st.name)
		}
		return fmt.Sprintf("key code %d%v", code, using), nil
	case st.mode != keyTapMode:
		return "", errors.New("AppleScriptではキーの押しっぱなしに対応していません")
	default:
		if st.text == "" {
			return "", errors.New("送信する文字がありません")
		}
		// 修飾キー付きは1文字ずつ指定する必要がある
		if st.mods != 0 {
			var lines []string
			for _, r := range st.text {
				lines = append(lines, fmt.Sprintf("keystroke %s%s", appleScriptKeyString(string(r)), using))
			}
			return strings.Join(lines, "\n  "), nil
		}
		return fmt.Sprintf("keystroke %s", appleScriptKeyString(st.text)), nil
	}
}

// appleScriptUsing は同時押しの修飾キーをAppleScriptの using 句へ変換する。
// 例: key code 9 using {command down}
func appleScriptUsing(mods keyMods) string {
	var items []string
	if mods&modCtrl != 0 {
		items = append(items, "control down")
	}
	if mods&modAlt != 0 {
		items = append(items, "option down")
	}
	if mods&modShift != 0 {
		items = append(items, "shift down")
	}
	if mods&modWin != 0 {
		items = append(items, "command down")
	}
	if len(items) == 0 {
		return ""
	}
	return " using {" + strings.Join(items, ", ") + "}"
}

// appleScriptString はAppleScriptの文字列リテラルを作る。
// 同名の関数が file.go にあるため、ここはキー送信用の別名で持つ。
func appleScriptKeyString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
