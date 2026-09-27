package nodelib

import (
	"strings"
	"testing"
)

// TestParseKeyStrokes はSendKeys記法のパーサを確認する（OSに依存しない）。
func TestParseKeyStrokes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []keyStroke
	}{
		{name: "素の文字列はまとめる", src: "abc", want: []keyStroke{
			{text: "abc", repeat: 1},
		}},
		{name: "日本語もまとめる", src: "あいう", want: []keyStroke{
			{text: "あいう", repeat: 1},
		}},
		{name: "チルダは Enter", src: "a~", want: []keyStroke{
			{text: "a", repeat: 1},
			{name: keyNameEnter, repeat: 1},
		}},
		{name: "修飾子は直後の1文字に掛かる", src: "^v", want: []keyStroke{
			{mods: modCtrl, text: "v", repeat: 1},
		}},
		{name: "カッコ内は修飾キーを押しっぱなしにする", src: "^(EC)", want: []keyStroke{
			{mods: modCtrl, text: "E", repeat: 1},
			{mods: modCtrl, text: "C", repeat: 1},
		}},
		{name: "AltとF4", src: "%{F4}", want: []keyStroke{
			{mods: modAlt, name: "F4", repeat: 1},
		}},
		{name: "Windowsキー", src: "&e", want: []keyStroke{
			{mods: modWin, text: "e", repeat: 1},
		}},
		{name: "特殊キーの前後の文字は別ストローク", src: "a{F1}b", want: []keyStroke{
			{text: "a", repeat: 1},
			{name: "F1", repeat: 1},
			{text: "b", repeat: 1},
		}},
		{name: "繰り返し回数", src: "{ENTER 3}", want: []keyStroke{
			{name: keyNameEnter, repeat: 3},
		}},
		{name: "キーの押しっぱなし", src: "{CTRL DOWN}c{CTRL UP}", want: []keyStroke{
			{name: keyNameCtrl, repeat: 1, mode: keyHoldMode},
			{text: "c", repeat: 1},
			{name: keyNameCtrl, repeat: 1, mode: keyUpMode},
		}},
		{name: "中カッコのエスケープ", src: "{{}a{}}", want: []keyStroke{
			{text: "{a}", repeat: 1},
		}},
		{name: "小文字のキー名も正規化する", src: "{enter}", want: []keyStroke{
			{name: keyNameEnter, repeat: 1},
		}},
		{name: "別名を論理キー名へ変換する", src: "{ESC}{DEL}{PGDN}", want: []keyStroke{
			{name: keyNameEscape, repeat: 1},
			{name: keyNameDelete, repeat: 1},
			{name: keyNamePageDown, repeat: 1},
		}},
		{name: "ネストしたカッコ", src: "^(a+(b))", want: []keyStroke{
			{mods: modCtrl, text: "a", repeat: 1},
			{mods: modCtrl | modShift, text: "b", repeat: 1},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKeyStrokes(tt.src)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			assertStrokes(t, got, tt.want)
		})
	}
}

// TestParseKeyStrokesError は不正な記法がエラーになることを確認する。
func TestParseKeyStrokesError(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "閉じカッコなし", src: "{ENTER"},
		{name: "開き丸カッコの対応なし", src: "^(EC"},
		{name: "不明なキー名", src: "{HOGE}"},
		{name: "空のキー名", src: "{}"},
		{name: "繰り返し回数が0", src: "{ENTER 0}"},
		{name: "繰り返し回数が大きすぎる", src: "{ENTER 9999}"},
		{name: "ストローク数が多すぎる", src: strings.Repeat("{F1}", maxKeyStrokes+1)},
		// ストローク数と繰り返し回数はそれぞれ上限内でも、積の総量が多すぎる場合
		{name: "キー入力の総数が多すぎる", src: strings.Repeat("{F1 100}", maxKeyTaps/100+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseKeyStrokes(tt.src); err == nil {
				t.Fatalf("エラーになるべきです: %q", tt.src)
			}
		})
	}
}

// TestKeyCommandsRegistered は命令がレジストリに載っていることを確認する。
func TestKeyCommandsRegistered(t *testing.T) {
	m := commands()
	c, ok := m["キー送信"]
	if !ok {
		t.Fatal("命令『キー送信』が登録されていません")
	}
	if len(c.josi) != 2 {
		t.Fatalf("助詞は2組であるべきです: %v", c.josi)
	}
	if c.fn == nil {
		t.Fatal("実装が登録されていません")
	}
	// 第1引数は送信内容（必須）、第2引数がウィンドウタイトル。
	// 省略された引数は空文字列になるため、空の送信内容と区別できるよう
	// 必須の送信内容を先に置いている。
	if !containsParticle(c.josi[0], "を") || !containsParticle(c.josi[1], "に") {
		t.Fatalf("助詞の順序が想定と違います: %v", c.josi)
	}
}

// TestUpdateHeldModifiersSentinel は修飾キーの押しっぱなしの追跡を確認する。
// {CTRL DOWN} と {CTRL UP} の間の操作にCtrlを適用できるようにするための処理。
func TestUpdateHeldModifiersSentinel(t *testing.T) {
	if bit, ok := modifierBitForKey(keyNameCtrl); !ok || bit != modCtrl {
		t.Errorf("CTRLのビット = %d, %v", bit, ok)
	}
	if _, ok := modifierBitForKey("ENTER"); ok {
		t.Error("ENTERは修飾キーではありません")
	}
	held := updateHeldModifiers(0, keyStroke{name: keyNameCtrl, repeat: 1, mode: keyHoldMode})
	if held != modCtrl {
		t.Fatalf("押しっぱなしの追跡に失敗: %d", held)
	}
	held = updateHeldModifiers(held, keyStroke{text: "c", repeat: 1})
	if held != modCtrl {
		t.Fatalf("文字の送信で解除されてはいけません: %d", held)
	}
	held = updateHeldModifiers(held, keyStroke{name: keyNameCtrl, repeat: 1, mode: keyUpMode})
	if held != 0 {
		t.Fatalf("解放できていません: %d", held)
	}
}

// containsParticle は助詞の一覧に p が含まれるかを返す。
func containsParticle(josi []string, p string) bool {
	for _, j := range josi {
		if j == p {
			return true
		}
	}
	return false
}

// TestKeyTapsLimit は上限ぎりぎりの記法が通ることを確認する。
func TestKeyTapsLimit(t *testing.T) {
	src := strings.Repeat("{F1 100}", maxKeyTaps/100)
	strokes, err := parseKeyStrokes(src)
	if err != nil {
		t.Fatalf("上限内の記法が通りません: %v", err)
	}
	if got := countKeyTaps(strokes); got != maxKeyTaps {
		t.Errorf("総数 = %d, 期待 = %d", got, maxKeyTaps)
	}
}

// TestCountKeyTaps はキー入力の総数の数え上げを確認する。
func TestCountKeyTaps(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{name: "素の文字列は文字数", src: "あいう", want: 3},
		{name: "繰り返しを掛ける", src: "{ENTER 3}", want: 3},
		{name: "文字と繰り返しの合計", src: "ab{F1 2}c", want: 5},
		{name: "同時押しも1ストローク1回", src: "^v", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strokes, err := parseKeyStrokes(tt.src)
			if err != nil {
				t.Fatalf("記法の解析に失敗: %v", err)
			}
			if got := countKeyTaps(strokes); got != tt.want {
				t.Errorf("総数 = %d, 期待 = %d", got, tt.want)
			}
		})
	}
}

// TestFunctionKeyNumber はファンクションキー名の番号の取り出しを確認する。
func TestFunctionKeyNumber(t *testing.T) {
	if n, ok := functionKeyNumber("F12"); !ok || n != 12 {
		t.Errorf("F12 = %d, %v", n, ok)
	}
	for _, name := range []string{"F0", "F17", "F", "X1", ""} {
		if _, ok := functionKeyNumber(name); ok {
			t.Errorf("%q はファンクションキーではありません", name)
		}
	}
}

// assertStrokes はストローク列が期待どおりかを確認する。
func assertStrokes(t *testing.T, got, want []keyStroke) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ストローク数 = %d, 期待 = %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] 取得 = %+v, 期待 = %+v", i, got[i], want[i])
		}
	}
}
