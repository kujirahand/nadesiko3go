package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/vm"
	"github.com/webview/webview_go"
)

//go:embed ui/*
var uiFS embed.FS

// RunResult is the JSON structure returned to JavaScript after execution.
type RunResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// AppInfo contains metadata about the running gonako-gui application.
type AppInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func main() {
	// 組み込みUIファイルをHTTPサーバーで配信
	subFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		fmt.Fprintf(os.Stderr, "UIアセットの読み込みに失敗しました: %v\n", err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ローカルサーバーの起動に失敗しました: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{
		Handler: http.FileServer(http.FS(subFS)),
	}
	go func() {
		_ = server.Serve(listener)
	}()

	// WebViewウィンドウの初期化
	debug := os.Getenv("GONAKO_DEBUG") == "1"
	w := webview.New(debug)
	if w == nil {
		fmt.Fprintln(os.Stderr, "WebViewの初期化に失敗しました。OSのWebView2/WebKitGTK/Cocoa環境を確認してください。")
		os.Exit(1)
	}
	defer w.Destroy()

	w.SetTitle("なでしこ3 (gonako-gui)")
	w.SetSize(980, 680, webview.HintNone)

	// Go ↔ JavaScript バインディング: なでしこコードの実行
	err = w.Bind("runNakoCode", func(code string) string {
		var outBuf strings.Builder
		host := vm.NewCUIHost(&outBuf, strings.NewReader(""), nil)

		runErr := vm.RunProgram(code, "gui.nako3", host)
		result := RunResult{
			OK:     runErr == nil,
			Output: outBuf.String(),
		}
		if runErr != nil {
			result.Error = runErr.Error()
		}
		b, _ := json.Marshal(result)
		return string(b)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "runNakoCode バインド失敗: %v\n", err)
	}

	// Go ↔ JavaScript バインディング: アプリケーション情報
	err = w.Bind("getAppInfo", func() string {
		info := AppInfo{
			Version: "3.6.0",
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		}
		b, _ := json.Marshal(info)
		return string(b)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "getAppInfo バインド失敗: %v\n", err)
	}

	// WebViewを開く
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d/index.html", port))
	w.Run()
}
