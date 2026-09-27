//go:build linux

package nodelib

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Linux版は X11 の xdotool を呼ぶ。
// Xのプロトコルを純Goで喋る実装も可能だが依存と実装量が増えるため、
// 当面は広く入っている xdotool があれば利用する方式にしている。
// Waylandでは xdotool が使えないため、エラーでその旨を案内する。
const xdotoolGuide = "X11環境の xdotool が必要です（例: sudo apt install xdotool）。Waylandでは利用できません"

// x11KeyNames は論理キー名から X のキーシンボル名への変換表。
var x11KeyNames = map[string]string{
	keyNameEnter:       "Return",
	keyNameTab:         "Tab",
	keyNameEscape:      "Escape",
	keyNameBackspace:   "BackSpace",
	keyNameDelete:      "Delete",
	keyNameInsert:      "Insert",
	keyNameHome:        "Home",
	keyNameEnd:         "End",
	keyNamePageUp:      "Page_Up",
	keyNamePageDown:    "Page_Down",
	keyNameUp:          "Up",
	keyNameDown:        "Down",
	keyNameLeft:        "Left",
	keyNameRight:       "Right",
	keyNameSpace:       "space",
	keyNameCapsLock:    "Caps_Lock",
	keyNameNumLock:     "Num_Lock",
	keyNameScrollLock:  "Scroll_Lock",
	keyNamePrintScreen: "Print",
	keyNamePause:       "Pause",
	keyNameBreak:       "Break",
	keyNameHelp:        "Help",
	keyNameCtrl:        "Control_L",
	keyNameRCtrl:       "Control_R",
	keyNameAlt:         "Alt_L",
	keyNameRAlt:        "Alt_R",
	keyNameShift:       "Shift_L",
	keyNameRShift:      "Shift_R",
	keyNameWin:         "Super_L",
}

// x11ModifierNames は同時押しする修飾キーの xdotool での名前（指定順）。
func x11ModifierNames(mods keyMods) []string {
	var names []string
	if mods&modCtrl != 0 {
		names = append(names, "ctrl")
	}
	if mods&modAlt != 0 {
		names = append(names, "alt")
	}
	if mods&modShift != 0 {
		names = append(names, "shift")
	}
	if mods&modWin != 0 {
		names = append(names, "super")
	}
	return names
}

// sendKeyStrokes は xdotool を呼び出してキー操作を送る。
func sendKeyStrokes(strokes []keyStroke) error {
	bin, err := exec.LookPath("xdotool")
	if err != nil {
		return errors.New(xdotoolGuide)
	}
	for _, st := range strokes {
		args, err := xdotoolArgs(st)
		if err != nil {
			return err
		}
		for i := 0; i < st.repeat; i++ {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("xdotoolに失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
			}
		}
	}
	return nil
}

// xdotoolArgs はストロークを xdotool の引数へ変換する。
func xdotoolArgs(st keyStroke) ([]string, error) {
	mods := x11ModifierNames(st.mods)
	var command string
	switch st.mode {
	case keyHoldMode:
		command = "keydown"
	case keyUpMode:
		command = "keyup"
	default:
		command = "key"
	}
	// 名前付きキーはキーシンボル名で指定する
	if st.name != "" {
		name, ok := x11KeyName(st.name)
		if !ok {
			return nil, fmt.Errorf("この環境では送信できないキーです: 『%s』", st.name)
		}
		return []string{command, "--clearmodifiers", strings.Join(append(mods, name), "+")}, nil
	}
	if st.text == "" {
		return nil, errors.New("送信する文字がありません")
	}
	// 修飾キーなしの文字列は type でまとめて入力する（日本語もそのまま送れる）
	if len(mods) == 0 {
		if command != "key" {
			return nil, errors.New("文字の押しっぱなしには対応していません")
		}
		return []string{"type", "--clearmodifiers", "--", st.text}, nil
	}
	// 修飾キー付きは1文字ずつ key で送る（ショートカットとして扱わせる）
	if command != "key" {
		return nil, errors.New("文字の押しっぱなしには対応していません")
	}
	if len([]rune(st.text)) != 1 {
		return nil, errors.New("修飾キー付きで送れるのは1文字だけです")
	}
	return []string{"key", "--clearmodifiers", strings.Join(append(mods, string([]rune(st.text)[0])), "+")}, nil
}

// x11KeyName は論理キー名を X のキーシンボル名へ変換する。
func x11KeyName(name string) (string, bool) {
	if n, ok := x11KeyNames[name]; ok {
		return n, true
	}
	if fn, ok := functionKeyNumber(name); ok {
		return fmt.Sprintf("F%d", fn), true
	}
	return "", false
}
