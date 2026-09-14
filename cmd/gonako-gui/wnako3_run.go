package main

// エディタの「ブラウザ(wnako3)」実行モード（#63）。
//
// エディタは gonako-gui 自身を --run-wnako3 付きの子プロセスとして起動する
// （launchChildRunProcess）。子プロセスはプログラムのフォルダを配信する
// ローカルサーバーを立て、ui/wnako3run.html の実行画面でプログラムを
// ブラウザ版なでしこ（wnako3）として動かす。タートル（plugin_turtle.js）は
// 実行画面が最初から読み込んでおく。

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/webview/webview_go"
)

const (
	// wnako3RunPagePath は実行画面のURL。フォルダの直下に置いたように見せ、
	// 『取り込む』や画像の相対パスがプログラムのフォルダから解決されるようにする。
	wnako3RunPagePath = "/__gonako-run.html"
	// wnako3RunProgramPath は実行するプログラム（JSON）を返すURL。
	wnako3RunProgramPath = "/__gonako-program.json"
	wnako3RunPageAsset   = "ui/wnako3run.html"
)

// launchWNako3WindowProcess はwnako3で実行する子プロセスを起動する。
func launchWNako3WindowProcess(code, filePath string) NewWindowRunResult {
	return launchChildRunProcess(code, filePath, "--run-wnako3")
}

// wnako3RunProgram は実行画面へ渡すプログラム。
type wnako3RunProgram struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// newWNako3RunHandler はフォルダの配信に、実行画面とプログラムを重ねる。
func newWNako3RunHandler(site fs.FS, program wnako3RunProgram) http.Handler {
	base := newSiteHandler(site)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case wnako3RunPagePath:
			page, err := fs.ReadFile(uiFS, wnako3RunPageAsset)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(page)
		case wnako3RunProgramPath:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(program)
		default:
			base.ServeHTTP(w, r)
		}
	})
}

// bindAsyncRunAPI は window.gonako.run（GONAKO実行）が使う非同期実行APIをBindする。
func bindAsyncRunAPI(w webview.WebView, session *guiSession, args []string, packed *bundle.Bundle) {
	_ = w.Bind("startNakoCode", func(code string) uint64 {
		return session.start(code, "gui.nako3", false, args, packed)
	})
	_ = w.Bind("pollNakoRun", func(runID uint64) string {
		b, _ := json.Marshal(session.poll(runID))
		return string(b)
	})
	_ = w.Bind("resolveNakoDialog", func(runID, dialogID uint64, text string, accepted bool) bool {
		return session.resolveDialog(runID, dialogID, text, accepted)
	})
}

// runWNako3Window は launchWNako3WindowProcess が起動した子プロセスの入口。
// 作業フォルダ（元のファイルのフォルダ）を配信し、wnako3でプログラムを動かす。
func runWNako3Window(sourcePath, title, sourceName string) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("プログラムを読み込めません: %v", err))
		return
	}
	os.Remove(sourcePath)

	if title == "" {
		title = "なでしこ3"
	}
	if sourceName == "" {
		sourceName = "main.nako3"
	}
	workDir, err := os.Getwd()
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("作業フォルダを取得できません: %v", err))
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("ローカルサーバーを起動できません: %v", err))
		return
	}
	defer listener.Close()
	program := wnako3RunProgram{Name: filepath.Base(sourceName), Code: string(data)}
	server := &http.Server{Handler: newWNako3RunHandler(os.DirFS(workDir), program)}
	go func() { _ = server.Serve(listener) }()

	w := newAppWindow(defaultWindowSettings(title, 960, 640))
	if w == nil {
		return
	}
	defer w.Destroy()

	session := &guiSession{window: newNativeWindowController(w)}
	bindAsyncRunAPI(w, session, nil, nil)
	installWNako3(w, newWNako3PageConfig(false), session.window, nil)

	port := listener.Addr().(*net.TCPAddr).Port
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d%s", port, wnako3RunPagePath))
	w.Run()
}
