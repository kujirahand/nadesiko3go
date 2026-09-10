package guilib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/webview/webview_go"
)

// Plugin is the GUI command set for WebView and native dialog operations.
type Plugin struct {
	dialogs fileDialogs
	screen  *Screen
}

// New creates a new guilib plugin instance.
func New() *Plugin {
	return &Plugin{dialogs: nativeFileDialogs(), screen: NewScreen()}
}

// NewWithScreen creates a plugin that writes GUI commands to screen.
func NewWithScreen(screen *Screen) *Plugin {
	if screen == nil {
		screen = NewScreen()
	}
	return &Plugin{dialogs: nativeFileDialogs(), screen: screen}
}

type command struct {
	josi       [][]string
	returnNone bool
	pure       bool
	fn         stdlib.Impl
}

// FuncList returns the command signatures for parser and lexer.
func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	list["フォーム値"] = &lexer.FuncItem{Name: "フォーム値", Type: "const", Value: ""}
	list["DOM親要素"] = &lexer.FuncItem{Name: "DOM親要素", Type: "const", Value: 0}
	for name, c := range p.commands() {
		list[name] = &lexer.FuncItem{
			Name:       name,
			Type:       "func",
			Josi:       c.josi,
			ReturnNone: c.returnNone,
			Pure:       c.pure,
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
		"HTML表示": { // @HTML文字列をウィンドウ画面に追加する // @HTMLひょうじ
			josi:       [][]string{{"を", "と"}},
			returnNone: true,
			fn:         p.cmdDisplayHTML,
		},
		"二択": {
			josi: [][]string{{"で", "の", "と", "を"}},
			fn:   p.cmdConfirm,
		},
		"DOM親要素設定": { // @DOM部品を追加する親要素を指定して、その要素を返す // @DOMおやようそせってい
			josi: [][]string{{"に", "へ"}},
			fn:   p.cmdSetDOMParent,
		},
		"DOM親部品設定": { // @DOM親要素設定と同じ // @DOMおやぶひんせってい
			josi: [][]string{{"に", "へ"}},
			fn:   p.cmdSetDOMParent,
		},
		"DOM部品作成": { // @タグ名のDOM部品を現在の親要素へ追加してハンドルを返す // @DOMぶひんさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateDOMPart,
		},
		"ラベル作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateLabel,
		},
		"エディタ作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateEditor,
		},
		"ボタン作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateButton,
		},
		"送信ボタン作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateSubmit,
		},
		"フォーム作成": {
			josi: [][]string{{"で", "の"}, {"を"}},
			fn:   p.cmdCreateForm,
		},
		"テキスト設定": {
			josi:       [][]string{{"に", "の", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetText,
		},
		"テキスト取得": {
			josi: [][]string{{"の", "から"}},
			fn:   p.cmdGetText,
		},
		"DOMスタイル一括設定": {
			josi:       [][]string{{"に", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetStyles,
		},
		"DOM属性設定": {
			josi:       [][]string{{"の"}, {"に", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetAttribute,
		},
		"DOM属性一括設定": { // @画面部品に辞書で指定した属性を一括設定する // @DOMぞくせいいっかつせってい
			josi:       [][]string{{"に", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetAttributes,
		},
		"DOM要素ID取得": {
			josi: [][]string{{"の", "を"}},
			pure: true,
			fn:   p.cmdGetElementByID,
		},
		"DOM要素取得": {
			josi: [][]string{{"の", "を"}},
			pure: true,
			fn:   p.cmdGetElement,
		},
		"DOM要素全取得": {
			josi: [][]string{{"の", "を"}},
			pure: true,
			fn:   p.cmdGetAllElements,
		},
		"DOMテキスト取得": {
			josi: [][]string{{"の", "から"}},
			pure: true,
			fn:   p.cmdGetText,
		},
		"DOMテキスト変更": { // @画面部品のテキストを変更する // @DOMてきすとへんこう
			josi:       [][]string{{"に", "の", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetText,
		},
		"DOMテキスト設定": {
			josi:       [][]string{{"に", "の", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetText,
		},
		"HTML取得": {
			josi: [][]string{{"の", "から"}},
			pure: true,
			fn:   p.cmdGetHTML,
		},
		"HTML変更": { // @画面部品のHTMLを変更する // @HTMLへんこう
			josi:       [][]string{{"に", "の", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetHTML,
		},
		"HTML設定": {
			josi:       [][]string{{"に", "の", "へ"}, {"を"}},
			returnNone: true,
			fn:         p.cmdSetHTML,
		},
		"DOM注目": { // @画面部品にフォーカスしてカーソルを移動する // @DOMちゅうもく
			josi:       [][]string{{"を", "へ", "に"}},
			returnNone: true,
			fn:         p.cmdFocus,
		},
		"注目": {
			josi:       [][]string{{"を", "へ", "に"}},
			returnNone: true,
			fn:         p.cmdFocus,
		},
		"クリック時": {
			josi:       [][]string{{"で"}, {"を", "の"}},
			returnNone: true,
			fn:         p.cmdOnClick,
		},
		"変更時": {
			josi:       [][]string{{"で"}, {"を", "の"}},
			returnNone: true,
			fn:         p.cmdOnChange,
		},
		"フォーム送信時": {
			josi:       [][]string{{"で"}, {"を", "の"}},
			returnNone: true,
			fn:         p.cmdOnSubmit,
		},
		"ファイル選択": { // @指定した拡張子のファイルをOS標準ダイアログで選択してパスを返す // @ふぁいるせんたく
			josi: [][]string{{"の"}},
			fn:   p.cmdSelectFile,
		},
		"保存ファイル選択": { // @指定した拡張子の保存先をOS標準ダイアログで選択してパスを返す // @ほぞんふぁいるせんたく
			josi: [][]string{{"の"}},
			fn:   p.cmdSelectSaveFile,
		},
		"フォルダ選択": { // @指定したフォルダを開始位置としてOS標準ダイアログでフォルダを選択しパスを返す // @ふぉるだせんたく
			josi: [][]string{{"で", "から", "の"}},
			fn:   p.cmdSelectFolder,
		},
		"ウィンドウ作成": { // @オプション設定（タイトル・サイズ等）とURLまたはHTMLコードからWebViewウィンドウを作成して表示する // @うぃんどうさくせい
			josi:       [][]string{{"で", "による"}, {"の", "を", "から"}},
			returnNone: true,
			fn:         p.cmdCreateWindow,
		},
	}
}

func (p *Plugin) cmdConfirm(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	if dialogs, ok := ctx.(stdlib.DialogContext); ok {
		_, accepted, supported, err := dialogs.ShowDialog("confirm", value.ToString(arg(args, 0)))
		if err != nil {
			return value.Undefined(), err
		}
		if supported {
			return value.Bool(accepted), nil
		}
	}
	return value.Bool(false), nil
}

func (p *Plugin) cmdDisplayHTML(_ stdlib.Context, args []value.Value) (value.Value, error) {
	p.screen.DisplayHTML(value.ToString(arg(args, 0)))
	return value.Undefined(), nil
}

func (p *Plugin) cmdSetDOMParent(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	target := arg(args, 0)
	handle, err := p.resolveDOMParent(target)
	if err != nil {
		return value.Null(), err
	}
	if err := p.screen.setParent(handle); err != nil {
		return value.Null(), err
	}
	ctx.SetSysVar("DOM親要素", value.Number(float64(handle)))
	return value.Number(float64(handle)), nil
}

func (p *Plugin) resolveDOMParent(target value.Value) (int, error) {
	if selector, ok := target.String(); ok {
		if handle, found := p.screen.query(selector); found {
			return handle, nil
		}
		if handle, found := p.screen.queryByID(selector); found {
			return handle, nil
		}
		return 0, fmt.Errorf("『DOM親要素設定』で要素『%s』が見つかりません。", selector)
	}
	if number, ok := target.Number(); ok && number == 0 {
		return 0, nil
	}
	handle, err := handleValue(target)
	if err != nil {
		return 0, err
	}
	if !p.screen.hasNode(handle) {
		return 0, fmt.Errorf("『DOM親要素設定』で画面部品ハンドル『%d』が見つかりません。", handle)
	}
	return handle, nil
}

func validDOMTag(tag string) bool {
	if tag == "" {
		return false
	}
	for i, r := range tag {
		if i == 0 {
			if !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
			return false
		}
	}
	return true
}

func (p *Plugin) createPart(tag, text, html, name string) (value.Value, error) {
	parent := p.screen.parent()
	h := p.screen.create(tag, text, html, name, parent)
	return value.Number(float64(h)), nil
}

func (p *Plugin) cmdCreateDOMPart(_ stdlib.Context, args []value.Value) (value.Value, error) {
	tag := strings.TrimSpace(value.ToString(arg(args, 0)))
	if !validDOMTag(tag) {
		return value.Undefined(), fmt.Errorf("『DOM部品作成』のタグ名『%s』が不正です。", tag)
	}
	return p.createPart(strings.ToLower(tag), "", "", "")
}

func (p *Plugin) cmdCreateLabel(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart("span", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateEditor(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart("input", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateButton(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart("button", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateSubmit(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart("submit", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateForm(_ stdlib.Context, args []value.Value) (value.Value, error) {
	parent := p.screen.parent()
	form := p.screen.create("form", "", "", "", parent)
	if attrs, ok := arg(args, 0).Dict(); ok && attrs != nil {
		values, err := stringMap(arg(args, 0))
		if err == nil {
			_ = p.screen.setAttributes(form, values)
		}
	}
	for _, row := range formRows(arg(args, 1)) {
		p.screen.create("label", row[0], "", "", form)
		editor := p.screen.create("input", row[1], "", row[0], form)
		_ = p.screen.setAttributes(editor, map[string]string{"name": row[0]})
	}
	p.screen.create("submit", "送信", "", "", form)
	return value.Number(float64(form)), nil
}

func (p *Plugin) cmdSetText(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.setText(h, value.ToString(arg(args, 1)))
}

func (p *Plugin) cmdGetText(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.String(""), err
	}
	text, err := p.screen.text(h)
	return value.String(text), err
}

func (p *Plugin) cmdSetStyles(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	styles, err := stringMap(arg(args, 1))
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.setStyles(h, styles)
}

func (p *Plugin) cmdSetAttribute(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	attrs := map[string]string{value.ToString(arg(args, 1)): value.ToString(arg(args, 2))}
	return value.Undefined(), p.screen.setAttributes(h, attrs)
}

func (p *Plugin) cmdSetAttributes(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	attrs, err := stringMap(arg(args, 1))
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.setAttributes(h, attrs)
}

func (p *Plugin) cmdGetElementByID(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if handle, ok := p.screen.queryByID(value.ToString(arg(args, 0))); ok {
		return value.Number(float64(handle)), nil
	}
	return value.Null(), nil
}

func (p *Plugin) cmdGetElement(_ stdlib.Context, args []value.Value) (value.Value, error) {
	query := arg(args, 0)
	selector, isString := query.String()
	if !isString {
		// TypeScript版と同じく、DOMハンドルが渡された場合はそのまま返す。
		return query, nil
	}
	if handle, ok := p.screen.query(selector); ok {
		return value.Number(float64(handle)), nil
	}
	return value.Null(), nil
}

func (p *Plugin) cmdGetAllElements(_ stdlib.Context, args []value.Value) (value.Value, error) {
	handles := p.screen.queryAll(value.ToString(arg(args, 0)))
	values := make([]value.Value, len(handles))
	for i, handle := range handles {
		values[i] = value.Number(float64(handle))
	}
	return value.ArrayValue(value.NewArray(values...)), nil
}

func (p *Plugin) cmdSetHTML(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.setHTML(h, value.ToString(arg(args, 1)))
}

func (p *Plugin) cmdGetHTML(_ stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := handleValue(arg(args, 0))
	if err != nil {
		return value.String(""), err
	}
	html, err := p.screen.html(h)
	return value.String(html), err
}

func (p *Plugin) cmdFocus(_ stdlib.Context, args []value.Value) (value.Value, error) {
	target := arg(args, 0)
	if selector, ok := target.String(); ok {
		h, found := p.screen.query(selector)
		if !found {
			return value.Undefined(), nil
		}
		return value.Undefined(), p.screen.focus(h)
	}
	if target.Kind() == value.KindNull || target.Kind() == value.KindUndefined {
		return value.Undefined(), nil
	}
	h, err := handleValue(target)
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.focus(h)
}

func (p *Plugin) bindEvent(ctx stdlib.Context, args []value.Value, event string) (value.Value, error) {
	fn, err := callable(ctx, arg(args, 0))
	if err != nil {
		return value.Undefined(), err
	}
	h, err := handleValue(arg(args, 1))
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), p.screen.bind(h, event, eventBinding{ctx: ctx, fn: fn})
}

func (p *Plugin) cmdOnClick(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.bindEvent(ctx, args, "click")
}

func (p *Plugin) cmdOnChange(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.bindEvent(ctx, args, "change")
}

func (p *Plugin) cmdOnSubmit(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.bindEvent(ctx, args, "submit")
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
