package nodelib

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// @キー操作

// keyCommands はキーボード操作をエミュレートする命令群（なでしこv1の『キー送信』相当）。
// 実体（実際のキーイベントの組み立てと送信）はOS別の key_*.go にある。
func keyCommands(m map[string]command) {
	m["キー送信"] = command{ // @アクティブなウィンドウへキー操作を送信する // @きーそうしん
		// 助詞は送信内容が第1引数、ウィンドウタイトルが第2引数。
		// 助詞で対応づくため「A(タイトル)へS(送信内容)を」の語順でも書ける。
		// 省略された引数は『それ』(空文字列)で埋まるため、「省略」と
		// 「空文字列の指定」を区別できない。そこで必須の送信内容を先に置き、
		// タイトルだけ渡されても何も送信しないようにしている。
		josi:       [][]string{{"を", "の"}, {"に", "へ"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			keys := str(a, 0)
			// TODO(#173): str(a, 1) のウィンドウタイトル指定は将来の課題。現状は
			// アクティブなウィンドウへの送信のみ対応する。
			if keys == "" {
				return value.Undefined(), nil
			}
			strokes, err := parseKeyStrokes(keys)
			if err != nil {
				return value.Undefined(), fmt.Errorf("キー送信: %w", err)
			}
			if err := sendKeyStrokes(strokes); err != nil {
				return value.Undefined(), fmt.Errorf("キー送信: %w", err)
			}
			return value.Undefined(), nil
		},
	}
}

// 以下の上限は、タイポなどで巨大な繰り返しを指定したときに
// OSへ膨大なイベントを流さないための安全弁。
//
// maxKeyStrokes と maxKeyRepeat は別々の軸の上限なので、この2つだけでは
// 「{F1 100}」を1000個並べたときのように片方の上限をすり抜ける。積の総量を
// maxKeyTaps で抑えることで、実際に送信されるイベント数を制限する。
const (
	maxKeyStrokes  = 1000 // 1回の命令で送るストローク数の上限
	maxKeyRepeat   = 100  // {ENTER 3} の繰り返し回数の上限
	maxKeyTextRune = 1024 // 1ストロークで入力する文字数の上限
	maxKeyTaps     = 5000 // 1回の命令で送信するキー入力の総数（ストローク数×繰り返し数）の上限
)

// keyMods は同時押しする修飾キーを表すビットフラグ。
type keyMods uint

const (
	modShift keyMods = 1 << iota
	modCtrl
	modAlt
	modWin
)

// keyMode は1ストロークの押し方。
type keyMode int

const (
	keyTapMode  keyMode = iota // 押して離す
	keyHoldMode                // 押したままにする（{CTRL DOWN}）
	keyUpMode                  // 離す（{CTRL UP}）
)

// keyStroke は「同時押しの修飾子 + 入力単位」を表すOS非依存の中間表現。
// name が空なら text を文字として入力し、空でなければ name の論理キーを打つ。
type keyStroke struct {
	mods   keyMods
	name   string
	text   string
	repeat int
	mode   keyMode
}

// isTextStroke は文字列入力として扱えるストロークかを返す。
func (s keyStroke) isTextStroke() bool {
	return s.name == "" && s.mods == 0 && s.mode == keyTapMode && s.repeat == 1
}

// parseKeyStrokes は SendKeys 形式の文字列をストローク列へ変換する。
//
// 記法はなでしこv1と同じくVB互換。
//
//   - ^ % &        … Shift / Ctrl / Alt / Windows キー（直後の1キーに適用）
//     ( )            … 修飾キーを押したままにする範囲
//     {ENTER} {F1}   … 特殊キー（{ENTER 3} で繰り返し、{CTRL DOWN} / {CTRL UP} で単体の押下・解放）
//     ~              … Enter
//     {{} {}}        … 「{」「}」そのもの
func parseKeyStrokes(src string) ([]keyStroke, error) {
	rs := []rune(src)
	var out []keyStroke
	mods := keyMods(0)
	count := 0

	// add は上限を確認しながらストロークを追加する。
	add := func(st keyStroke) error {
		count++
		if count > maxKeyStrokes {
			return fmt.Errorf("送信するキーの数が多すぎます（上限%d）", maxKeyStrokes)
		}
		out = appendStroke(out, st)
		return nil
	}

	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '+':
			mods |= modShift
			i++
			continue
		case r == '^':
			mods |= modCtrl
			i++
			continue
		case r == '%':
			mods |= modAlt
			i++
			continue
		case r == '&':
			mods |= modWin
			i++
			continue
		case r == '~':
			if err := add(keyStroke{mods: mods, name: keyNameEnter, repeat: 1}); err != nil {
				return nil, err
			}
			mods = 0
			i++
			continue
		case r == '(':
			end, err := findGroupEnd(rs, i)
			if err != nil {
				return nil, err
			}
			inner, err := parseKeyStrokes(string(rs[i+1 : end]))
			if err != nil {
				return nil, err
			}
			for _, st := range inner {
				st.mods |= mods
				// 修飾キー付きのときはまとめた文字列を1文字ずつに戻す。
				// （^(EC) は Ctrl+E → Ctrl+C であって "EC" という入力ではない）
				if mods != 0 && st.name == "" && utf8.RuneCountInString(st.text) > 1 {
					for _, r := range st.text {
						if err := add(keyStroke{mods: st.mods, text: string(r), repeat: 1}); err != nil {
							return nil, err
						}
					}
					continue
				}
				if err := add(st); err != nil {
					return nil, err
				}
			}
			mods = 0
			i = end + 1
			continue
		case r == '{':
			st, next, err := readBraceStroke(rs, i, mods)
			if err != nil {
				return nil, err
			}
			if err := add(st); err != nil {
				return nil, err
			}
			mods = 0
			i = next
			continue
		default:
			// 通常の1文字。修飾子は直後の1文字だけに掛かる（VB互換）。
			if err := add(keyStroke{mods: mods, text: string(r), repeat: 1}); err != nil {
				return nil, err
			}
			mods = 0
			i++
			continue
		}
	}
	if total := countKeyTaps(out); total > maxKeyTaps {
		return nil, fmt.Errorf("送信するキーの総数が多すぎます（上限%d）", maxKeyTaps)
	}
	return out, nil
}

// countKeyTaps はストローク列が実際に送信するキー入力の総数を数える。
// 繰り返し回数を掛け、文字列ストロークは文字数ぶんとして数える。
func countKeyTaps(strokes []keyStroke) int {
	total := 0
	for _, st := range strokes {
		if st.repeat < 1 {
			continue
		}
		units := 1
		if st.name == "" {
			units = utf8.RuneCountInString(st.text)
		}
		total += units * st.repeat
	}
	return total
}

// appendStroke は隣り合う素の文字入力をまとめ、ストローク列へ追加する。
// 日本語など複数文字の入力はOS側でまとめて送ったほうが速く確実なため。
func appendStroke(out []keyStroke, st keyStroke) []keyStroke {
	if st.repeat < 1 {
		st.repeat = 1
	}
	if st.isTextStroke() && utf8.RuneCountInString(st.text) == 1 {
		if n := len(out); n > 0 && out[n-1].isTextStroke() && utf8.RuneCountInString(out[n-1].text) < maxKeyTextRune {
			out[n-1].text += st.text
			return out
		}
	}
	return append(out, st)
}

// findGroupEnd は開きカッコ rs[i] == '(' に対応する閉じカッコの位置を返す。
func findGroupEnd(rs []rune, i int) (int, error) {
	depth := 0
	for j := i; j < len(rs); j++ {
		switch rs[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j, nil
			}
		}
	}
	return 0, fmt.Errorf("「(」に対応する「)」がありません: 『%s』", string(rs))
}

// readBraceStroke は '{' の位置 i から1ストローク分を読み、
// 次の読み取り位置とストロークを返す。
func readBraceStroke(rs []rune, i int, mods keyMods) (keyStroke, int, error) {
	st := keyStroke{mods: mods, repeat: 1}
	// エスケープ: {{} → 「{」、{}} → 「}」（VBのSendKeysと同じ）
	if i+2 < len(rs) && rs[i+1] == '{' && rs[i+2] == '}' {
		st.text = "{"
		return st, i + 3, nil
	}
	if i+2 < len(rs) && rs[i+1] == '}' && rs[i+2] == '}' {
		st.text = "}"
		return st, i + 3, nil
	}
	end := -1
	for j := i + 1; j < len(rs); j++ {
		if rs[j] == '}' {
			end = j
			break
		}
	}
	if end < 0 {
		return keyStroke{}, 0, fmt.Errorf("「{」に対応する「}」がありません: 『%s』", string(rs))
	}
	content := strings.TrimSpace(string(rs[i+1 : end]))
	if content == "" {
		return keyStroke{}, 0, fmt.Errorf("キー名が空です: 『%s』", string(rs[i:end+1]))
	}
	// {CTRL DOWN} / {CTRL UP} は修飾キー単体の押下・解放
	upper := strings.ToUpper(content)
	switch {
	case strings.HasSuffix(upper, " DOWN"):
		st.mode = keyHoldMode
		content = strings.TrimSpace(content[:len(content)-len(" DOWN")])
	case strings.HasSuffix(upper, " UP"):
		st.mode = keyUpMode
		content = strings.TrimSpace(content[:len(content)-len(" UP")])
	}
	// {ENTER 3} のような繰り返し回数の指定
	if fields := strings.Fields(content); len(fields) > 1 {
		if n, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
			if n < 1 || n > maxKeyRepeat {
				return keyStroke{}, 0, fmt.Errorf("繰り返し回数は1〜%dで指定してください: 『%s』", maxKeyRepeat, content)
			}
			st.repeat = n
			content = strings.Join(fields[:len(fields)-1], " ")
		}
	}
	name, ok := normalizeKeyName(content)
	if !ok {
		return keyStroke{}, 0, fmt.Errorf("不明なキー名です: 『%s』", content)
	}
	st.name = name
	return st, end + 1, nil
}

// 論理キー名。OS別の実装はこの名前をネイティブのキーコードへ変換する。
const (
	keyNameEnter       = "ENTER"
	keyNameTab         = "TAB"
	keyNameEscape      = "ESCAPE"
	keyNameBackspace   = "BACKSPACE"
	keyNameDelete      = "DELETE"
	keyNameInsert      = "INSERT"
	keyNameHome        = "HOME"
	keyNameEnd         = "END"
	keyNamePageUp      = "PAGEUP"
	keyNamePageDown    = "PAGEDOWN"
	keyNameUp          = "UP"
	keyNameDown        = "DOWN"
	keyNameLeft        = "LEFT"
	keyNameRight       = "RIGHT"
	keyNameSpace       = "SPACE"
	keyNameCapsLock    = "CAPSLOCK"
	keyNameNumLock     = "NUMLOCK"
	keyNameScrollLock  = "SCROLLLOCK"
	keyNamePrintScreen = "PRINTSCREEN"
	keyNamePause       = "PAUSE"
	keyNameBreak       = "BREAK"
	keyNameHelp        = "HELP"
	keyNameCtrl        = "CTRL"
	keyNameRCtrl       = "RCTRL"
	keyNameAlt         = "ALT"
	keyNameRAlt        = "RALT"
	keyNameShift       = "SHIFT"
	keyNameRShift      = "RSHIFT"
	keyNameWin         = "WIN"
	keyNameIME         = "IME"
)

// knownKeyNames は送信できる論理キー名の一覧。
var knownKeyNames = func() map[string]bool {
	m := map[string]bool{
		keyNameEnter: true, keyNameTab: true, keyNameEscape: true,
		keyNameBackspace: true, keyNameDelete: true, keyNameInsert: true,
		keyNameHome: true, keyNameEnd: true, keyNamePageUp: true, keyNamePageDown: true,
		keyNameUp: true, keyNameDown: true, keyNameLeft: true, keyNameRight: true,
		keyNameSpace: true, keyNameCapsLock: true, keyNameNumLock: true,
		keyNameScrollLock: true, keyNamePrintScreen: true, keyNamePause: true,
		keyNameBreak: true, keyNameHelp: true,
		keyNameCtrl: true, keyNameRCtrl: true, keyNameAlt: true, keyNameRAlt: true,
		keyNameShift: true, keyNameRShift: true, keyNameWin: true, keyNameIME: true,
	}
	for i := 1; i <= 16; i++ {
		m[fmt.Sprintf("F%d", i)] = true
	}
	return m
}()

// keyNameAliases は SendKeys で書ける別名を論理キー名へ正規化する表。
var keyNameAliases = map[string]string{
	"RETURN":       keyNameEnter,
	"ESC":          keyNameEscape,
	"BS":           keyNameBackspace,
	"BKSP":         keyNameBackspace,
	"DEL":          keyNameDelete,
	"INS":          keyNameInsert,
	"PGUP":         keyNamePageUp,
	"PGDN":         keyNamePageDown,
	"PAGE_UP":      keyNamePageUp,
	"PAGE_DOWN":    keyNamePageDown,
	"PAGEUP":       keyNamePageUp,
	"PAGEDOWN":     keyNamePageDown,
	"PRTSC":        keyNamePrintScreen,
	"PRINT":        keyNamePrintScreen,
	"PRINTSCRN":    keyNamePrintScreen,
	"PRINT_SCREEN": keyNamePrintScreen,
	"CTRLBREAK":    keyNameBreak,
	"CONTROL":      keyNameCtrl,
	"RCONTROL":     keyNameRCtrl,
	"OPTION":       keyNameAlt,
	"ROPTION":      keyNameRAlt,
	"ALTGR":        keyNameRAlt,
	"COMMAND":      keyNameWin,
	"CMD":          keyNameWin,
	"META":         keyNameWin,
	"LWIN":         keyNameWin,
	"RWIN":         keyNameWin,
	"SUPER":        keyNameWin,
	"NUMLK":        keyNameNumLock,
	"CAPS":         keyNameCapsLock,
	"CAPS_LOCK":    keyNameCapsLock,
	"SCROLLLOCK":   keyNameScrollLock,
	"SCROLL_LOCK":  keyNameScrollLock,
}

// modifierBitForKey は修飾キーの論理キー名に対応するビットを返す。
// {CTRL DOWN} のような明示的な押しっぱなしを追跡するために使う。
func modifierBitForKey(name string) (keyMods, bool) {
	switch name {
	case keyNameCtrl, keyNameRCtrl:
		return modCtrl, true
	case keyNameAlt, keyNameRAlt:
		return modAlt, true
	case keyNameShift, keyNameRShift:
		return modShift, true
	case keyNameWin:
		return modWin, true
	}
	return 0, false
}

// updateHeldModifiers は明示的な押下・解放に合わせて保持中の修飾キーを更新する。
// 押しっぱなしにした修飾キーは、解放されるまでの間のストロークにも適用する。
func updateHeldModifiers(held keyMods, st keyStroke) keyMods {
	bit, ok := modifierBitForKey(st.name)
	if !ok {
		return held
	}
	switch st.mode {
	case keyHoldMode:
		return held | bit
	case keyUpMode:
		return held &^ bit
	default:
		return held
	}
}

// normalizeKeyName は記法のキー名を論理キー名へ正規化する。
func normalizeKeyName(name string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	upper = strings.ReplaceAll(upper, "-", "_")
	if upper == "" {
		return "", false
	}
	if knownKeyNames[upper] {
		return upper, true
	}
	if canonical, ok := keyNameAliases[upper]; ok {
		return canonical, true
	}
	return "", false
}

// functionKeyNumber は "F12" のような論理キー名から番号を取り出す。
func functionKeyNumber(name string) (int, bool) {
	if len(name) < 2 || name[0] != 'F' {
		return 0, false
	}
	n, err := strconv.Atoi(name[1:])
	if err != nil || n < 1 || n > 16 {
		return 0, false
	}
	return n, true
}
