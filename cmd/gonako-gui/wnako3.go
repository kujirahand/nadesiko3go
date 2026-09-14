package main

// ブラウザ版なでしこ（wnako3.js）を gonako-gui に組み込む（#63）。
//
// ui/wnako3/ には scripts/copy-nadesiko3.sh が本家の release/*.js を取り込む。
// ローカルHTTPサーバーは /__gonako/wnako3/ でそれを配信し、WebViewには
// 全ページの先頭で ui/gonako-loader.js を注入する。ローダーは
// <script type="なでしこ"> のあるページ（または index.json の "wnako3": true）
// でだけ wnako3.js を読み込むので、既存のHTMLアプリの動きは変わらない。

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/webview/webview_go"
)

const (
	// wnako3AssetRoot は取り込んだ本家ブラウザ版の置き場（//go:embed の中）。
	wnako3AssetRoot = "ui/wnako3"
	// gonakoAssetPrefix はgonako-guiが内部資材を配信するURLの接頭辞。
	gonakoAssetPrefix = "/__gonako/"
	// gonakoLoaderAsset はWebViewへ注入するローダー。
	gonakoLoaderAsset = "ui/gonako-loader.js"
)

// WNako3Info は取り込んだwnako3の版と取得元（local / cdn）。
type WNako3Info struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

// readWNako3Info は ui/wnako3/VERSION を読む。取り込まれていなければ空を返す。
func readWNako3Info() WNako3Info {
	var info WNako3Info
	data, err := fs.ReadFile(uiFS, wnako3AssetRoot+"/VERSION")
	if err != nil {
		return info
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "version":
			info.Version = val
		case "source":
			info.Source = val
		}
	}
	return info
}

// wnako3AssetName は、URLのパスが同梱したwnako3の.jsファイルを指すとき、そのファイル名を返す。
func wnako3AssetName(name string) (string, bool) {
	if name == "" || strings.Contains(name, "/") || path.Ext(name) != ".js" {
		return "", false
	}
	if _, err := fs.Stat(uiFS, wnako3AssetRoot+"/"+name); err != nil {
		return "", false
	}
	return name, true
}

// serveWNako3Asset は同梱したwnako3のファイルを1つ返す。
func serveWNako3Asset(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(uiFS, wnako3AssetRoot+"/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

// newSiteHandler はHTMLフォルダを配信し、gonako-guiの内部資材も重ねて配信する。
//
//   - /__gonako/wnako3/<名前>.js は同梱したwnako3を返す
//   - フォルダに無い wnako3.js や plugin_turtle.js などは、どの階層で要求されても
//     同梱版を返す。本家と同じ <script src="wnako3.js"> や
//     『!「plugin_turtle.js」を取り込む』がそのまま動くようにするため。
//     フォルダに実物があれば、そちらを優先する。
func newSiteHandler(site fs.FS) http.Handler {
	files := http.FileServer(http.FS(site))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if rest, ok := strings.CutPrefix(p, gonakoAssetPrefix+"wnako3/"); ok {
			if name, ok := wnako3AssetName(rest); ok {
				serveWNako3Asset(w, r, name)
				return
			}
			http.NotFound(w, r)
			return
		}
		if name, ok := wnako3AssetName(path.Base(p)); ok {
			rel := strings.TrimPrefix(path.Clean(p), "/")
			if _, err := fs.Stat(site, rel); err != nil {
				serveWNako3Asset(w, r, name)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

// wnako3PageConfig はローダーへ渡す設定。
type wnako3PageConfig struct {
	// WNako3 はなでしこスクリプトの有無にかかわらずwnako3.jsを読み込むかどうか。
	WNako3        bool   `json:"wnako3"`
	WNako3Version string `json:"wnako3Version"`
	GonakoVersion string `json:"gonakoVersion"`
}

// wnako3ConfigFromIndexJSON は index.json の "wnako3": true を読む。
// ほかの項目はウィンドウ設定（window_config.go）が扱うので、ここでは見ない。
func wnako3ConfigFromIndexJSON(data []byte) bool {
	var raw struct {
		WNako3 bool `json:"wnako3"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	return raw.WNako3
}

// wnako3InitScript はWebViewの全ページ先頭で実行するJavaScriptを作る。
func wnako3InitScript(cfg wnako3PageConfig) string {
	loader, err := fs.ReadFile(uiFS, gonakoLoaderAsset)
	if err != nil {
		return ""
	}
	cfgJSON, _ := json.Marshal(cfg)
	return "window.__gonakoConfig = " + string(cfgJSON) + ";\n" + string(loader)
}

// newWNako3PageConfig は既定値を埋めた設定を返す。
func newWNako3PageConfig(forceWNako3 bool) wnako3PageConfig {
	return wnako3PageConfig{
		WNako3:        forceWNako3,
		WNako3Version: readWNako3Info().Version,
		GonakoVersion: appVersion,
	}
}

// installWNako3 はwnako3のローダーと、Go側の命令を呼ぶブリッジをWebViewへ組み込む。
// Bind より前、Navigate/SetHtml より前に呼ぶこと。
func installWNako3(w webview.WebView, cfg wnako3PageConfig, windows guilib.WindowController, packed *bundle.Bundle) *commandBridge {
	if script := wnako3InitScript(cfg); script != "" {
		w.Init(script)
	}
	bridge := newCommandBridge(windows, packed, func(js string) {
		w.Dispatch(func() { w.Eval(js) })
	})
	_ = w.Bind("startNakoCommand", bridge.start)
	return bridge
}
