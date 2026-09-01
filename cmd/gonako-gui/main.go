package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

const usage = `gonako-gui - なでしこ3 GUI (軽量WebView)

使い方:
  gonako-gui [オプション] [HTMLフォルダまたはファイル]

例:
  gonako-gui                        内蔵の標準エディタUIを起動
  gonako-gui ./my-app/              ./my-app フォルダ内の index.html を表示
  gonako-gui ./my-app/index.html    指定したHTMLファイルを表示
  gonako-gui -dir ./web/ -title "マイアプリ"

オプション:
  -dir string      配信・表示するHTMLフォルダのパス
  -url string      直接開くURL (http:// または https://)
  -title string    ウィンドウタイトル (既定: "なでしこ3 (gonako-gui)")
  -width int       ウィンドウの幅 (既定: 980)
  -height int      ウィンドウの高さ (既定: 680)
  -debug           開発者ツール（デバッグモード）を有効化
  -help, --help    このヘルプを表示
`

func main() {
	flags := flag.NewFlagSet("gonako-gui", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Print(usage)
	}

	dirFlag := flags.String("dir", "", "配信するHTMLフォルダのパス")
	urlFlag := flags.String("url", "", "開くURL")
	titleFlag := flags.String("title", "なでしこ3 (gonako-gui)", "ウィンドウタイトル")
	widthFlag := flags.Int("width", 980, "ウィンドウの幅")
	heightFlag := flags.Int("height", 680, "ウィンドウの高さ")
	debugFlag := flags.Bool("debug", os.Getenv("GONAKO_DEBUG") == "1", "デバッグモード有効化")

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	targetDir := *dirFlag
	targetURL := *urlFlag
	startPage := "index.html"

	// 位置引数の処理
	if flags.NArg() > 0 && targetDir == "" && targetURL == "" {
		arg := flags.Arg(0)
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			targetURL = arg
		} else {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "指定されたパス『%s』が見つかりません: %v\n", arg, err)
				os.Exit(1)
			}
			if info.IsDir() {
				targetDir = arg
			} else {
				targetDir = filepath.Dir(arg)
				startPage = filepath.Base(arg)
			}
		}
	}

	var handler http.Handler
	if targetDir != "" {
		absDir, err := filepath.Abs(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "パスの解決に失敗しました: %v\n", err)
			os.Exit(1)
		}
		handler = http.FileServer(http.Dir(absDir))
	} else if targetURL == "" {
		// 組み込みUIファイルをHTTPサーバーで配信
		subFS, err := fs.Sub(uiFS, "ui")
		if err != nil {
			fmt.Fprintf(os.Stderr, "UIアセットの読み込みに失敗しました: %v\n", err)
			os.Exit(1)
		}
		handler = http.FileServer(http.FS(subFS))
	}

	var finalURL string
	if targetURL != "" {
		finalURL = targetURL
	} else {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ローカルサーバーの起動に失敗しました: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		port := listener.Addr().(*net.TCPAddr).Port
		server := &http.Server{Handler: handler}
		go func() {
			_ = server.Serve(listener)
		}()
		finalURL = fmt.Sprintf("http://127.0.0.1:%d/%s", port, startPage)
	}

	// WebViewウィンドウの初期化
	w := webview.New(*debugFlag)
	if w == nil {
		fmt.Fprintln(os.Stderr, "WebViewの初期化に失敗しました。OSのWebView2/WebKitGTK/Cocoa環境を確認してください。")
		os.Exit(1)
	}
	defer w.Destroy()

	w.SetTitle(*titleFlag)
	w.SetSize(*widthFlag, *heightFlag, webview.HintNone)

	// Go ↔ JavaScript バインディング: なでしこコードの実行
	err := w.Bind("runNakoCode", func(code string) string {
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
	w.Navigate(finalURL)
	w.Run()
}
