package main

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

type sessionWindowController struct {
	handle   int
	settings guilib.WindowSettings
	info     guilib.WindowInfo
}

func (f *sessionWindowController) Change(handle int, settings guilib.WindowSettings) error {
	f.handle = handle
	f.settings = settings
	return nil
}

func (f *sessionWindowController) Info(handle int) (guilib.WindowInfo, error) {
	f.handle = handle
	return f.info, nil
}

func TestWindowCommandsRunThroughVM(t *testing.T) {
	controller := &sessionWindowController{info: guilib.WindowInfo{
		Width: 640, Height: 480, X: 10, Y: 20,
		State: "最大化", Title: "見本", Resizable: true,
	}}
	session := &guiSession{window: controller}
	result := session.run(`
設定＝{"サイズ":[640,480],"位置":"中央","状態":"最大化"}
設定で母艦のウィンドウ変更。
情報＝母艦のウィンドウ取得。
情報["状態"]を表示。
`, "window.nako3", false, nil, nil)
	if !result.OK {
		t.Fatalf("ウィンドウ命令を実行できません: %s", result.Error)
	}
	if result.Output != "最大化\n" {
		t.Fatalf("ウィンドウ取得の結果が違います: %q", result.Output)
	}
	if controller.handle != guilib.MotherWindowHandle || controller.settings.Width != 640 || !controller.settings.Center {
		t.Fatalf("母艦の設定がコントローラーへ渡されていません: handle=%d settings=%#v", controller.handle, controller.settings)
	}
}
