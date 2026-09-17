package main

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

// windowThemes はネイティブウィンドウごとに指定中のテーマを覚える。
// OSから「指定されたテーマ」を読み戻せない環境があるため、Go側で保持する。
var windowThemes sync.Map

func windowTheme(window unsafe.Pointer) string {
	if theme, ok := windowThemes.Load(window); ok {
		return theme.(string)
	}
	return guilib.ThemeAuto
}

func themeCSSName(theme string) string {
	switch theme {
	case guilib.ThemeLight:
		return "light"
	case guilib.ThemeDark:
		return "dark"
	default:
		return "auto"
	}
}

// nativeThemeCode はOS層へ渡す番号（0=自動、1=ライト、2=ダーク）。
func nativeThemeCode(theme string) int {
	switch theme {
	case guilib.ThemeLight:
		return 1
	case guilib.ThemeDark:
		return 2
	default:
		return 0
	}
}

// themeScript はページの<html>へdata-gonako-theme（light/dark）を付ける。
// 自動のときはprefers-color-schemeの変化にも追従し、OSの配色変更を
// __gonakoSyncNativeThemeでネイティブ側（Windowsのタイトルバーなど）へ伝える。
// ライト・ダークのときはcolor-schemeも固定し、配色を持たないページでも
// ブラウザ標準の背景色や文字色がテーマに合うようにする。
func themeScript(theme string) string {
	return fmt.Sprintf(`(function(theme){
  window.__gonakoTheme = theme;
  if (!window.__gonakoApplyTheme) {
    var mq = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
    window.__gonakoApplyTheme = function() {
      var root = document.documentElement;
      if (!root) { return; }
      var t = window.__gonakoTheme;
      var resolved = t === 'auto' ? (mq && mq.matches ? 'dark' : 'light') : t;
      root.setAttribute('data-gonako-theme', resolved);
      root.style.colorScheme = t === 'auto' ? '' : t;
      try { window.dispatchEvent(new CustomEvent('gonako-theme-change', { detail: { theme: t, resolved: resolved } })); } catch (e) {}
    };
    if (mq) {
      var onChange = function() {
        if (window.__gonakoTheme !== 'auto') { return; }
        window.__gonakoApplyTheme();
        if (typeof window.__gonakoSyncNativeTheme === 'function') { window.__gonakoSyncNativeTheme(); }
      };
      if (mq.addEventListener) { mq.addEventListener('change', onChange); } else if (mq.addListener) { mq.addListener(onChange); }
    }
    if (!document.documentElement) {
      document.addEventListener('DOMContentLoaded', function() { window.__gonakoApplyTheme(); });
    }
  }
  window.__gonakoApplyTheme();
})(%q);`, themeCSSName(theme))
}

// themeWebView はapplyWindowSettingsが必要とするWebViewの機能だけを表す。
type themeWebView interface {
	Window() unsafe.Pointer
	Init(js string)
	Eval(js string)
	Bind(name string, f interface{}) error
}

// themeSyncBound はBind済みのウィンドウを覚える。同じ名前は二度Bindできない。
var themeSyncBound sync.Map

// bindNativeThemeSync は、自動テーマのウィンドウでOSの配色が変わったときに
// ページから呼ばれる関数を登録する。Bindの呼び出しはUIスレッドで実行される。
func bindNativeThemeSync(w themeWebView) {
	window := w.Window()
	if _, loaded := themeSyncBound.LoadOrStore(window, true); loaded {
		return
	}
	_ = w.Bind("__gonakoSyncNativeTheme", func() {
		if windowTheme(window) == guilib.ThemeAuto {
			_ = platformApplyWindowTheme(window, guilib.ThemeAuto)
		}
	})
}

// applyWindowTheme はUIスレッドから呼ぶ。Initで以降のページ遷移にも適用し、
// Evalで表示中のページへすぐ反映する。
func applyWindowTheme(w themeWebView, theme string) error {
	window := w.Window()
	if err := platformApplyWindowTheme(window, theme); err != nil {
		return err
	}
	windowThemes.Store(window, theme)
	bindNativeThemeSync(w)
	script := themeScript(theme)
	w.Init(script)
	w.Eval(script)
	return nil
}

// nativeWindowInfo はOSから得た情報に、指定中のテーマを加える。
func nativeWindowInfo(window unsafe.Pointer) (guilib.WindowInfo, error) {
	info, err := platformWindowInfo(window)
	if err != nil {
		return info, err
	}
	info.Theme = windowTheme(window)
	return info, nil
}
