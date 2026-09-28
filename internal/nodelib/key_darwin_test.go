//go:build darwin

package nodelib

import (
	"os"
	"testing"
)

// TestDarwinKeySetup はCoreGraphicsの関数が取り出せることを確認する。
// 実際のキー送信は行わない（副作用のある操作は TestSendKeysReal に譲る）。
func TestDarwinKeySetup(t *testing.T) {
	if err := darwinSetup(); err != nil {
		t.Fatalf("CoreGraphicsの準備に失敗しました: %v", err)
	}
	f := &darwinKeys.funcs
	if f.cgEventCreateKeyboardEvent == nil || f.cgEventPost == nil || f.cgEventSetFlags == nil {
		t.Fatal("キー操作に必要な関数が取り出せていません")
	}
}

// TestAppleScriptFallback は AppleScript フォールバックのスクリプト生成を確認する。
// 実行は伴わない（実際の送信は TestSendKeysReal に譲る）。
func TestAppleScriptFallback(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "文字列は keystroke",
			src:  "abc",
			want: "tell application \"System Events\"\n  keystroke \"abc\"\nend tell",
		},
		{
			name: "同時押しは using 句",
			src:  "^v",
			want: "tell application \"System Events\"\n  keystroke \"v\" using {control down}\nend tell",
		},
		{
			name: "特殊キーは key code",
			src:  "{ENTER}",
			want: "tell application \"System Events\"\n  key code 36\nend tell",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strokes, err := parseKeyStrokes(tt.src)
			if err != nil {
				t.Fatalf("記法の解析に失敗しました: %v", err)
			}
			got, err := appleScript(strokes)
			if err != nil {
				t.Fatalf("スクリプトの生成に失敗しました: %v", err)
			}
			if got != tt.want {
				t.Errorf("スクリプト = %q, 期待 = %q", got, tt.want)
			}
		})
	}
}

// TestSendKeysReal は実際にキーを送信する確認用のテスト。
// 副作用のある操作なので、既定では実行されない。実行するときは、
// 送信先のアプリケーションを前面に出してから次を実行する。
//
//	GONAKO_TEST_SENDKEY=こんにちは go test ./internal/nodelib -run TestSendKeysReal -v
func TestSendKeysReal(t *testing.T) {
	notation := os.Getenv("GONAKO_TEST_SENDKEY")
	if notation == "" {
		t.Skip("環境変数 GONAKO_TEST_SENDKEY で送信内容を指定したときだけ実行します")
	}
	if err := darwinSetup(); err != nil {
		t.Fatalf("CoreGraphicsの準備に失敗しました: %v", err)
	}
	strokes, err := parseKeyStrokes(notation)
	if err != nil {
		t.Fatalf("記法の解析に失敗しました: %v", err)
	}
	if err := sendKeyStrokes(strokes); err != nil {
		t.Fatalf("キーの送信に失敗しました: %v", err)
	}
}
