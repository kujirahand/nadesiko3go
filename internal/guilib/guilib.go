package guilib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/webview/webview_go"
)

// Plugin is the GUI command set for WebView window operations.
type Plugin struct{}

// New creates a new guilib plugin instance.
func New() *Plugin { return &Plugin{} }

type command struct {
	josi       [][]string
	returnNone bool
	fn         stdlib.Impl
}

// FuncList returns the command signatures for parser and lexer.
func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	for name, c := range commands() {
		list[name] = &lexer.FuncItem{
			Name:       name,
			Type:       "func",
			Josi:       c.josi,
			ReturnNone: c.returnNone,
			Pure:       false,
		}
	}
	return list
}

// Impls returns the command implementation map.
func (p *Plugin) Impls() map[string]stdlib.Impl {
	out := map[string]stdlib.Impl{}
	for name, c := range commands() {
		out[name] = c.fn
	}
	return out
}

func commands() map[string]command {
	return map[string]command{
		"ウィンドウ作成": {
			josi:       [][]string{{"から", "で", "を"}},
			returnNone: true,
			fn:         cmdCreateWindow,
		},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

// cmdCreateWindow opens a WebView window for the given URL or HTML string.
// (URLから|URLで|URLを) ウィンドウ作成
func cmdCreateWindow(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if len(args) < 1 {
		return value.Undefined(), nil
	}

	furl := value.ToString(arg(args, 0))
	trimmed := strings.TrimSpace(furl)
	lower := strings.ToLower(trimmed)

	isHTML := strings.HasPrefix(lower, "<html") || strings.HasPrefix(lower, "<!doctype")

	var targetURL string
	if isHTML {
		// HTMLコンテンツの場合は一時HTMLファイルを作成して開く
		tmpFile, err := os.CreateTemp("", "nako_win_*.html")
		if err != nil {
			return value.Undefined(), fmt.Errorf("一時HTMLファイルの作成に失敗しました: %w", err)
		}
		defer tmpFile.Close()

		if _, err := tmpFile.WriteString(furl); err != nil {
			return value.Undefined(), fmt.Errorf("一時HTMLファイルへの書き込みに失敗しました: %w", err)
		}
		absPath, _ := filepath.Abs(tmpFile.Name())
		targetURL = "file://" + absPath
	} else if strings.HasPrefix(furl, "http://") || strings.HasPrefix(furl, "https://") || strings.HasPrefix(furl, "file://") {
		targetURL = furl
	} else {
		// 相対パスまたはローカルファイルパス
		absPath, err := filepath.Abs(furl)
		if err == nil {
			targetURL = "file://" + absPath
		} else {
			targetURL = furl
		}
	}

	// 実行中のバイナリ（gonako-gui または gonako）があるか確認し、別プロセスで起動
	// これによりGUIエディタ内からの呼び出しでもUIメインスレッドがブロックされない
	exe, err := os.Executable()
	if err == nil && (strings.Contains(filepath.Base(exe), "gonako-gui") || strings.Contains(filepath.Base(exe), "gonako")) {
		// gonako-gui -url <targetURL> をバックグラウンドで起動
		guiExe := exe
		if !strings.Contains(filepath.Base(exe), "gonako-gui") {
			// gonako (CUI) の場合は同じディレクトリにある gonako-gui を探す
			cand := filepath.Join(filepath.Dir(exe), "gonako-gui")
			if _, err := os.Stat(cand); err == nil {
				guiExe = cand
			}
		}
		cmd := exec.Command(guiExe, "-url", targetURL)
		if err := cmd.Start(); err == nil {
			return value.Undefined(), nil
		}
	}

	// フォールバック: 現在のプロセスで直接webviewを開く
	w := webview.New(false)
	if w == nil {
		return value.Undefined(), fmt.Errorf("WebViewの作成に失敗しました")
	}
	defer w.Destroy()

	w.SetTitle("なでしこ3")
	w.SetSize(960, 640, webview.HintNone)
	if isHTML {
		w.SetHtml(furl)
	} else {
		w.Navigate(targetURL)
	}
	w.Run()

	return value.Undefined(), nil
}
