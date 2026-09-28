//go:build linux

package nodelib

import (
	"testing"
)

// TestUpdateHeldModifiers は押しっぱなしの追跡を確認する（OS非依存の共通処理）。
func TestUpdateHeldModifiers(t *testing.T) {
	strokes, err := parseKeyStrokes("{CTRL DOWN}c{CTRL UP}")
	if err != nil {
		t.Fatalf("記法の解析に失敗: %v", err)
	}
	held := keyMods(0)
	commands := make([][][]string, 0, len(strokes))
	for _, st := range strokes {
		got, err := xdotoolCommands(st, held)
		if err != nil {
			t.Fatalf("引数の組み立てに失敗: %v", err)
		}
		commands = append(commands, got)
		held = updateHeldModifiers(held, st)
	}
	if held != 0 {
		t.Errorf("最後までに修飾キーが解放されていません: %d", held)
	}
	want := [][]string{
		{"keydown", "--clearmodifiers", "Control_L"},
		{"key", "c"}, // 保持中の間は --clearmodifiers を付けない
		{"keyup", "Control_L"},
	}
	if len(commands) != len(want) {
		t.Fatalf("コマンド数 = %d, 期待 = %d: %v", len(commands), len(want), commands)
	}
	for i := range want {
		assertArgs(t, commands[i][0], want[i])
	}
}

// TestXdotoolCommands は xdotool の引数の組み立てを確認する。
func TestXdotoolCommands(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{name: "文字列は type でまとめる", src: "こんにちは", want: []string{"type", "--clearmodifiers", "--", "こんにちは"}},
		{name: "同時押しは key の組み合わせ", src: "^c", want: []string{"key", "--clearmodifiers", "ctrl+c"}},
		{name: "AltとF4", src: "%{F4}", want: []string{"key", "--clearmodifiers", "alt+F4"}},
		{name: "Enter", src: "{ENTER}", want: []string{"key", "--clearmodifiers", "Return"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strokes, err := parseKeyStrokes(tt.src)
			if err != nil {
				t.Fatalf("記法の解析に失敗: %v", err)
			}
			commands, err := xdotoolCommands(strokes[0], 0)
			if err != nil {
				t.Fatalf("引数の組み立てに失敗: %v", err)
			}
			assertArgs(t, commands[0], tt.want)
		})
	}
}

// assertArgs は xdotool への引数が期待どおりかを確認する。
func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("引数 = %v, 期待 = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] 引数 = %q, 期待 = %q", i, got[i], want[i])
		}
	}
}
