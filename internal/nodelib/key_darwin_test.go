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
	f := &darwinKeys.funcs
	if f.cgPreflightPostEventAccess != nil && !f.cgPreflightPostEventAccess() {
		t.Skip(macOSAccessibilityGuide)
	}
	strokes, err := parseKeyStrokes(notation)
	if err != nil {
		t.Fatalf("記法の解析に失敗しました: %v", err)
	}
	if err := sendKeyStrokes(strokes); err != nil {
		t.Fatalf("キーの送信に失敗しました: %v", err)
	}
}
