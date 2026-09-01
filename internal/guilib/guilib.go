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
type Plugin struct {
	title  string
	width  int
	height int
	debug  bool
}

// New creates a new guilib plugin instance with default window settings.
func New() *Plugin {
	return &Plugin{
		title:  "なでしこ3",
		width:  960,
		height: 640,
		debug:  false,
	}
}

type command struct {
	josi       [][]string
	returnNone bool
	fn         stdlib.Impl
}

// FuncList returns the command signatures for parser and lexer.
func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	for name, c := range p.commands() {
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
	for name, c := range p.commands() {
		out[name] = c.fn
	}
	return out
}

func (p *Plugin) commands() map[string]command {
	return map[string]command{
		"ウィンドウ作成": {
			josi:       [][]string{{"から", "で", "を"}},
			returnNone: true,
			fn:         p.cmdCreateWindow,
		},
		"ウィンドウ設定": {
			josi:       [][]string{{"の", "で", "を", "に"}},
			returnNone: true,
			fn:         p.cmdSetWindowConfig,
		},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

// cmdSetWindowConfig sets window properties like title, width, height, and debug.
// (設定の|設定で|設定を|設定に) ウィンドウ設定
func (p *Plugin) cmdSetWindowConfig(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if len(args) < 1 {
		return value.Undefined(), nil
	}

	arg0 := arg(args, 0)
	d, ok := arg0.Dict()
	if !ok || d == nil {
		return value.Undefined(), nil
	}

	// タイトル / title
	if v, ok := d.Get("タイトル"); ok {
		p.title = value.ToString(v)
	} else if v, ok := d.Get("title"); ok {
		p.title = value.ToString(v)
	}

	// サイズ: [幅, 高さ] (例: [800, 600])
	if v, ok := d.Get("サイズ"); ok {
		if arr, isArr := v.Array(); isArr && arr != nil && arr.Len() >= 2 {
			if w, ok := arr.Get(0).Number(); ok && w > 0 {
				p.width = int(w)
			}
			if h, ok := arr.Get(1).Number(); ok && h > 0 {
				p.height = int(h)
			}
		}
	} else if v, ok := d.Get("size"); ok {
		if arr, isArr := v.Array(); isArr && arr != nil && arr.Len() >= 2 {
			if w, ok := arr.Get(0).Number(); ok && w > 0 {
				p.width = int(w)
			}
			if h, ok := arr.Get(1).Number(); ok && h > 0 {
				p.height = int(h)
			}
		}
	}

	// 幅 / width
	if v, ok := d.Get("幅"); ok {
		if w, ok := v.Number(); ok && w > 0 {
			p.width = int(w)
		}
	} else if v, ok := d.Get("width"); ok {
		if w, ok := v.Number(); ok && w > 0 {
			p.width = int(w)
		}
	}

	// 高さ / height
	if v, ok := d.Get("高さ"); ok {
		if h, ok := v.Number(); ok && h > 0 {
			p.height = int(h)
		}
	} else if v, ok := d.Get("height"); ok {
		if h, ok := v.Number(); ok && h > 0 {
			p.height = int(h)
		}
	}

	// デバッグ / debug
	if v, ok := d.Get("デバッグ"); ok {
		p.debug = value.ToBool(v)
	} else if v, ok := d.Get("debug"); ok {
		p.debug = value.ToBool(v)
	}

	return value.Undefined(), nil
}

// cmdCreateWindow opens a WebView window for the given URL or HTML string.
// (URLから|URLで|URLを) ウィンドウ作成
func (p *Plugin) cmdCreateWindow(_ stdlib.Context, args []value.Value) (value.Value, error) {
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

	title := p.title
	if title == "" {
		title = "なでしこ3"
	}
	width := p.width
	if width <= 0 {
		width = 960
	}
	height := p.height
	if height <= 0 {
		height = 640
	}

	// 実行中のバイナリ（gonako-gui または gonako）があるか確認し、別プロセスで起動
	// これによりGUIエディタ内からの呼び出しでもUIメインスレッドがブロックされない
	exe, err := os.Executable()
	if err == nil && (strings.Contains(filepath.Base(exe), "gonako-gui") || strings.Contains(filepath.Base(exe), "gonako")) {
		guiExe := exe
		if !strings.Contains(filepath.Base(exe), "gonako-gui") {
			// gonako (CUI) の場合は同じディレクトリにある gonako-gui を探す
			cand := filepath.Join(filepath.Dir(exe), "gonako-gui")
			if _, err := os.Stat(cand); err == nil {
				guiExe = cand
			}
		}
		cmdArgs := []string{
			"-url", targetURL,
			"-title", title,
			"-width", fmt.Sprintf("%d", width),
			"-height", fmt.Sprintf("%d", height),
		}
		if p.debug {
			cmdArgs = append(cmdArgs, "-debug")
		}
		cmd := exec.Command(guiExe, cmdArgs...)
		if err := cmd.Start(); err == nil {
			return value.Undefined(), nil
		}
	}

	// フォールバック: 現在のプロセスで直接webviewを開く
	w := webview.New(p.debug)
	if w == nil {
		return value.Undefined(), fmt.Errorf("WebViewの作成に失敗しました")
	}
	defer w.Destroy()

	w.SetTitle(title)
	w.SetSize(width, height, webview.HintNone)
	if isHTML {
		w.SetHtml(furl)
	} else {
		w.Navigate(targetURL)
	}
	w.Run()

	return value.Undefined(), nil
}
