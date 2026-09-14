package main

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

// virtualWindowController は実ウィンドウに触れず、辞書の設定を
// メモリ上に保ったまま『母艦のウィンドウ取得』へ返せること（#97）。
func TestVirtualWindowControllerRoundTripsSettings(t *testing.T) {
	controller := newVirtualWindowController()

	before, err := controller.Info(guilib.MotherWindowHandle)
	if err != nil {
		t.Fatalf("初期状態を取得できません: %v", err)
	}
	if before.Width == 0 || before.Height == 0 {
		t.Fatalf("初期状態に既定のサイズがありません: %#v", before)
	}

	err = controller.Change(guilib.MotherWindowHandle, guilib.WindowSettings{
		HasSize: true, Width: 640, Height: 480,
		HasPosition: true, X: 10, Y: 20,
		HasState: true, State: "最大化",
		HasTitle: true, Title: "見本",
	})
	if err != nil {
		t.Fatalf("設定を変更できません: %v", err)
	}

	after, err := controller.Info(guilib.MotherWindowHandle)
	if err != nil {
		t.Fatalf("変更後の状態を取得できません: %v", err)
	}
	if after.Width != 640 || after.Height != 480 || after.X != 10 || after.Y != 20 ||
		after.State != "最大化" || after.Title != "見本" {
		t.Fatalf("変更した設定が反映されていません: %#v", after)
	}
}

// ハンドル0(母艦)以外は、実ウィンドウ用のコントローラーと同じくエラーになること。
func TestVirtualWindowControllerRejectsUnknownHandle(t *testing.T) {
	controller := newVirtualWindowController()
	if _, err := controller.Info(1); err == nil {
		t.Fatal("ハンドル0以外を受け付けてはいけません")
	}
	if err := controller.Change(1, guilib.WindowSettings{}); err == nil {
		t.Fatal("ハンドル0以外を受け付けてはいけません")
	}
}

// エディタ内蔵の「インライン」「コマンドライン」実行は、疑似コントローラーを
// 母艦として使うので、実行しても呼び出し元のコントローラーは変化しない。
func TestInlineRunDoesNotTouchRealWindowController(t *testing.T) {
	real := &sessionWindowController{info: guilib.WindowInfo{Width: 999, Height: 999}}
	session := &guiSession{window: real}

	result := session.runWithWindow(`
設定＝{"サイズ":[320,240]}
設定で母艦のウィンドウ変更。
`, "inline.nako3", false, nil, nil, newVirtualWindowController())
	if !result.OK {
		t.Fatalf("実行に失敗しました: %s", result.Error)
	}
	if real.handle != 0 {
		t.Fatalf("実ウィンドウのコントローラーが呼び出されています: handle=%d", real.handle)
	}
}
