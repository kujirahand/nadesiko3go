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

// Plugin is the GUI command set for WebView and native dialog operations.
type Plugin struct {
	dialogs fileDialogs
}

// New creates a new guilib plugin instance.
func New() *Plugin {
	return &Plugin{dialogs: nativeFileDialogs()}
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
		"ファイル選択": {
			josi: [][]string{{"の"}},
			fn:   p.cmdSelectFile,
		},
		"保存ファイル選択": {
			josi: [][]string{{"の"}},
			fn:   p.cmdSelectSaveFile,
		},
		"フォルダ選択": {
			josi: [][]string{{"で", "から", "の"}},
			fn:   p.cmdSelectFolder,
		},
		"ウィンドウ作成": {
			josi:       [][]string{{"で", "による"}, {"の", "を", "から"}},
			returnNone: true,
			fn:         p.cmdCreateWindow,
		},
	}
}

func (p *Plugin) cmdSelectFile(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	path, err := p.dialogs.open(
		normalizeDefaultDir(contextBaseDir(ctx)),
		normalizeExtension(value.ToString(arg(args, 0))),
	)
	if err != nil {
		return value.String(""), err
	}
	return value.String(path), nil
}

func (p *Plugin) cmdSelectSaveFile(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	extension := normalizeExtension(value.ToString(arg(args, 0)))
	path, err := p.dialogs.save(normalizeDefaultDir(contextBaseDir(ctx)), defaultFileName(extension), extension)
	if err != nil {
		return value.String(""), err
	}
	return value.String(addDefaultExtension(path, extension)), nil
}

func (p *Plugin) cmdSelectFolder(_ stdlib.Context, args []value.Value) (value.Value, error) {
	path, err := p.dialogs.folder(normalizeDefaultDir(value.ToString(arg(args, 0))))
	if err != nil {
		return value.String(""), err
	}
	return value.String(path), nil
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

type windowConfig struct {
	title  string
	width  int
	height int
	debug  bool
}

func parseWindowConfig(v value.Value) windowConfig {
	cfg := windowConfig{
		title:  "なでしこ3",
		width:  960,
		height: 640,
		debug:  false,
	}

	d, ok := v.Dict()
	if !ok || d == nil {
		return cfg
	}

	// タイトル / title
	if tv, ok := d.Get("タイトル"); ok {
		cfg.title = value.ToString(tv)
	} else if tv, ok := d.Get("title"); ok {
		cfg.title = value.ToString(tv)
	}

	// サイズ: [幅, 高さ] (例: [800, 600])
	if sv, ok := d.Get("サイズ"); ok {
		if arr, isArr := sv.Array(); isArr && arr != nil && arr.Len() >= 2 {
			if w, ok := arr.Get(0).Number(); ok && w > 0 {
				cfg.width = int(w)
			}
			if h, ok := arr.Get(1).Number(); ok && h > 0 {
				cfg.height = int(h)
			}
		}
	} else if sv, ok := d.Get("size"); ok {
		if arr, isArr := sv.Array(); isArr && arr != nil && arr.Len() >= 2 {
			if w, ok := arr.Get(0).Number(); ok && w > 0 {
				cfg.width = int(w)
			}
			if h, ok := arr.Get(1).Number(); ok && h > 0 {
				cfg.height = int(h)
			}
		}
	}

	// 幅 / width
	if wv, ok := d.Get("幅"); ok {
		if w, ok := wv.Number(); ok && w > 0 {
			cfg.width = int(w)
		}
	} else if wv, ok := d.Get("width"); ok {
		if w, ok := wv.Number(); ok && w > 0 {
			cfg.width = int(w)
		}
	}

	// 高さ / height
	if hv, ok := d.Get("高さ"); ok {
		if h, ok := hv.Number(); ok && h > 0 {
			cfg.height = int(h)
		}
	} else if hv, ok := d.Get("height"); ok {
		if h, ok := hv.Number(); ok && h > 0 {
			cfg.height = int(h)
		}
	}

	// デバッグ / debug
	if dv, ok := d.Get("デバッグ"); ok {
		cfg.debug = value.ToBool(dv)
	} else if dv, ok := d.Get("debug"); ok {
		cfg.debug = value.ToBool(dv)
	}

	return cfg
}

// cmdCreateWindow opens a WebView window for the given URL or HTML string with options.
// (OPTION_OBJでURLの|OPTION_OBJでURLを) ウィンドウ作成
func (p *Plugin) cmdCreateWindow(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if len(args) < 1 {
		return value.Undefined(), nil
	}

	var furl string
	cfg := windowConfig{
		title:  "なでしこ3",
		width:  960,
		height: 640,
		debug:  false,
	}

	if len(args) >= 2 {
		// args[0] = OPTION_OBJ, args[1] = URL
		optArg := arg(args, 0)
		urlArg := arg(args, 1)

		if optArg.Kind() == value.KindDict {
			cfg = parseWindowConfig(optArg)
			furl = value.ToString(urlArg)
		} else if urlArg.Kind() == value.KindDict {
			cfg = parseWindowConfig(urlArg)
			furl = value.ToString(optArg)
		} else {
			furl = value.ToString(urlArg)
		}
	} else {
		// 1引数の場合
		arg0 := arg(args, 0)
		if arg0.Kind() == value.KindDict {
			cfg = parseWindowConfig(arg0)
			d, _ := arg0.Dict()
			if uv, ok := d.Get("URL"); ok {
				furl = value.ToString(uv)
			} else if uv, ok := d.Get("url"); ok {
				furl = value.ToString(uv)
			} else if uv, ok := d.Get("HTML"); ok {
				furl = value.ToString(uv)
			} else if uv, ok := d.Get("html"); ok {
				furl = value.ToString(uv)
			}
		} else {
			furl = value.ToString(arg0)
		}
	}

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
			"-title", cfg.title,
			"-width", fmt.Sprintf("%d", cfg.width),
			"-height", fmt.Sprintf("%d", cfg.height),
		}
		if cfg.debug {
			cmdArgs = append(cmdArgs, "-debug")
		}
		cmd := exec.Command(guiExe, cmdArgs...)
		if err := cmd.Start(); err == nil {
			return value.Undefined(), nil
		}
	}

	// フォールバック: 現在のプロセスで直接webviewを開く
	w := webview.New(cfg.debug)
	if w == nil {
		return value.Undefined(), fmt.Errorf("WebViewの作成に失敗しました")
	}
	defer w.Destroy()

	w.SetTitle(cfg.title)
	w.SetSize(cfg.width, cfg.height, webview.HintNone)
	if isHTML {
		w.SetHtml(furl)
	} else {
		w.Navigate(targetURL)
	}
	w.Run()

	return value.Undefined(), nil
}
