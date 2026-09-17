package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/webview/webview_go"
)

const windowDispatchTimeout = 5 * time.Second

// nativeWindowController は、なでしこの論理ハンドルと現在のWebViewを結び付ける。
// 現時点で操作できるのは論理ハンドル0の「母艦」だけである。
type nativeWindowController struct {
	webview webview.WebView
}

// directNativeWindowController はWebViewのBindコールバック専用である。
// BindはUIスレッド上で呼ばれるため、そこから再度Dispatchすると相互待ちになる。
type directNativeWindowController struct {
	webview webview.WebView
}

func newNativeWindowController(w webview.WebView) *nativeWindowController {
	return &nativeWindowController{webview: w}
}

func newDirectNativeWindowController(w webview.WebView) *directNativeWindowController {
	return &directNativeWindowController{webview: w}
}

func checkMotherWindowHandle(handle int) error {
	if handle != guilib.MotherWindowHandle {
		return fmt.Errorf("ウィンドウハンドル%dは見つかりません", handle)
	}
	return nil
}

// virtualWindowController は、エディタ内蔵実行（インライン・コマンドライン）
// の「母艦のウィンドウ」を扱う。実際に触れるとエディタ自身のウィンドウが
// 動いてしまう（#97）ため、実ウィンドウには触れず疑似的な状態だけを
// メモリ上に保持する。「ウィンドウ(GUI)」モードだけが別プロセスの実ウィンドウ
// （nativeWindowController）を母艦として使う。
type virtualWindowController struct {
	mu    sync.Mutex
	state guilib.WindowInfo
}

func newVirtualWindowController() *virtualWindowController {
	return &virtualWindowController{state: guilib.WindowInfo{
		Width: 1080, Height: 720, State: "通常", Title: "なでしこ3", Resizable: true,
		Theme: guilib.ThemeAuto,
	}}
}

func (c *virtualWindowController) Change(handle int, settings guilib.WindowSettings) error {
	if err := checkMotherWindowHandle(handle); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if settings.HasSize {
		c.state.Width = settings.Width
		c.state.Height = settings.Height
	}
	if settings.HasPosition {
		if settings.Center {
			c.state.X = 0
			c.state.Y = 0
		} else {
			c.state.X = settings.X
			c.state.Y = settings.Y
		}
	}
	if settings.HasState {
		c.state.State = settings.State
	}
	if settings.HasTitle {
		c.state.Title = settings.Title
	}
	if settings.HasResizable {
		c.state.Resizable = settings.Resizable
	}
	if settings.HasTheme {
		c.state.Theme = settings.Theme
	}
	return nil
}

func (c *virtualWindowController) Info(handle int) (guilib.WindowInfo, error) {
	if err := checkMotherWindowHandle(handle); err != nil {
		return guilib.WindowInfo{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state, nil
}

// visibleWindowState は同時に残り得るOS側フラグから利用者向けの状態を選ぶ。
// 最小化中は画面に表示されていないため、保存中の全画面状態より優先する。
func visibleWindowState(minimized, fullscreen, maximized bool) string {
	if minimized {
		return "最小化"
	}
	if fullscreen {
		return "全画面"
	}
	if maximized {
		return "最大化"
	}
	return "通常"
}

func (c *nativeWindowController) Change(handle int, settings guilib.WindowSettings) error {
	if err := checkMotherWindowHandle(handle); err != nil {
		return err
	}
	done := make(chan error, 1)
	c.webview.Dispatch(func() {
		done <- applyWindowSettings(c.webview, settings, true)
	})
	select {
	case err := <-done:
		return err
	case <-time.After(windowDispatchTimeout):
		return fmt.Errorf("ウィンドウ変更がタイムアウトしました")
	}
}

func (c *nativeWindowController) Info(handle int) (guilib.WindowInfo, error) {
	if err := checkMotherWindowHandle(handle); err != nil {
		return guilib.WindowInfo{}, err
	}
	type result struct {
		info guilib.WindowInfo
		err  error
	}
	done := make(chan result, 1)
	c.webview.Dispatch(func() {
		info, err := nativeWindowInfo(c.webview.Window())
		done <- result{info: info, err: err}
	})
	select {
	case got := <-done:
		return got.info, got.err
	case <-time.After(windowDispatchTimeout):
		return guilib.WindowInfo{}, fmt.Errorf("ウィンドウ取得がタイムアウトしました")
	}
}

func (c *directNativeWindowController) Change(handle int, settings guilib.WindowSettings) error {
	if err := checkMotherWindowHandle(handle); err != nil {
		return err
	}
	return applyWindowSettings(c.webview, settings, true)
}

func (c *directNativeWindowController) Info(handle int) (guilib.WindowInfo, error) {
	if err := checkMotherWindowHandle(handle); err != nil {
		return guilib.WindowInfo{}, err
	}
	return nativeWindowInfo(c.webview.Window())
}

// applyWindowSettings はWebViewのUIスレッドから呼び出す。
// 起動前の初期設定にも同じ処理を使うことで、表示後のちらつきを避ける。
func applyWindowSettings(w webview.WebView, settings guilib.WindowSettings, preservePosition bool) error {
	var before guilib.WindowInfo
	if settings.HasSize && (preservePosition || !settings.HasResizable) {
		info, err := platformWindowInfo(w.Window())
		if err != nil {
			return err
		}
		before = info
	}
	if settings.HasTitle {
		w.SetTitle(settings.Title)
	}
	if settings.HasSize {
		var hint webview.Hint = webview.HintNone
		if (settings.HasResizable && !settings.Resizable) || (!settings.HasResizable && !before.Resizable) {
			hint = webview.HintFixed
		}
		w.SetSize(settings.Width, settings.Height, hint)
		if preservePosition && !settings.HasPosition {
			settings.HasPosition = true
			settings.Center = false
			settings.X = before.X
			settings.Y = before.Y
		}
	}
	if settings.HasTheme {
		if err := applyWindowTheme(w, settings.Theme); err != nil {
			return err
		}
	}
	return platformApplyWindowSettings(w.Window(), settings)
}
