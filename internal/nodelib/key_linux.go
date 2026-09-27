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
// {CTRL DOWN} のように明示的に押しっぱなしにした修飾キーは held で追跡し、
// 解放されるまでの間のストロークにも適用する。この間の xdotool 呼び出しに
// --clearmodifiers を付けると保持中の修飾キーが解除されてしまうため外す。
func sendKeyStrokes(strokes []keyStroke) error {
	bin, err := exec.LookPath("xdotool")
	if err != nil {
		return errors.New(xdotoolGuide)
	}
	held := keyMods(0)
	for _, st := range strokes {
		commands, err := xdotoolCommands(st, held)
		if err != nil {
			return err
		}
		for i := 0; i < st.repeat; i++ {
			for _, args := range commands {
				out, err := exec.Command(bin, args...).CombinedOutput()
				if err != nil {
					return fmt.Errorf("xdotoolに失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
				}
			}
		}
		held = updateHeldModifiers(held, st)
	}
	return nil
}

// xdotoolCommands はストロークを xdotool の引数列へ変換する。
// held は {CTRL DOWN} などで保持中（物理的に押されている）の修飾キー。
func xdotoolCommands(st keyStroke, held keyMods) ([][]string, error) {
	// 保持中のキーがあるときは --clearmodifiers を付けない（解除してしまうため）
	var clear []string
	if held == 0 {
		clear = []string{"--clearmodifiers"}
	}
	// 名前付きキーはキーシンボル名で指定する
	if st.name != "" {
		name, ok := x11KeyName(st.name)
		if !ok {
			return nil, fmt.Errorf("この環境では送信できないキーです: 『%s』", st.name)
		}
		combo := strings.Join(append(x11ModifierNames(st.mods), name), "+")
		return [][]string{joinArgs([]string{x11KeyCommand(st.mode)}, clear, []string{combo})}, nil
	}
	if st.text == "" {
		return nil, errors.New("送信する文字がありません")
	}
	if st.mode != keyTapMode {
		return nil, errors.New("文字の押しっぱなしには対応していません")
	}
	// 同時押しの文字は1文字ずつキーの組み合わせとして送る。
	// （type でまとめてUnicode入力する経路ではショートカットとして届かない）
	if st.mods != 0 || held != 0 {
		var commands [][]string
		for _, r := range st.text {
			// 保持中の修飾キーは物理的に押されているので、ここでは指定しない。
			// 指定すると xdotool が最後にその修飾キーを解放してしまう。
			var keys []string
			if st.mods != 0 {
				keys = append(x11ModifierNames(st.mods), string(r))
			} else {
				keys = []string{string(r)}
			}
			commands = append(commands, joinArgs([]string{"key"}, clear, []string{strings.Join(keys, "+")}))
		}
		return commands, nil
	}
	// 修飾キーなしの文字列は type でまとめて入力する（日本語もそのまま送れる）
	return [][]string{joinArgs([]string{"type"}, clear, []string{"--", st.text})}, nil
}

// x11KeyCommand は押し方に対応する xdotool のサブコマンドを返す。
func x11KeyCommand(mode keyMode) string {
	switch mode {
	case keyHoldMode:
		return "keydown"
	case keyUpMode:
		return "keyup"
	default:
		return "key"
	}
}

// joinArgs は引数の断片を1つの引数列につなげる（空の断片は無視する）。
func joinArgs(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
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
