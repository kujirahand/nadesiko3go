package guilib

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

	// DOMスキン設定だけを使うプログラムでは、VMが『DOMスキン』用の
	// 記憶領域を作らないことがあるため、選択中の名前を控えておく。
	domSkinMu sync.Mutex
	domSkin   string
	// システム変数『DOMスキン』が使えるかどうかの判定結果。
	domSkinProbed bool
	domSkinUsable bool

	// システム変数『DOM親要素』が使えるかどうかの判定結果。→ domParentVarUsable
	domParentMu     sync.Mutex
	domParentProbed bool
	domParentUsable bool
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
	list["DOMスキン"] = &lexer.FuncItem{Name: "DOMスキン", Type: "const", Value: ""}
	list["DOMスキン辞書"] = &lexer.FuncItem{Name: "DOMスキン辞書", Type: "const", Value: value.DictValue(value.NewDict())}
	domOptions := value.NewDict()
	domOptions.Set("自動改行", value.Bool(false))
	domOptions.Set("テーブルヘッダ", value.Bool(true))
	domOptions.Set("テーブル背景色", value.ArrayValue(value.NewArray(
		value.String("#AA4040"), value.String("#ffffff"), value.String("#fff0f0"),
	)))
	domOptions.Set("テーブル数値右寄せ", value.Bool(true))
	list["DOM部品オプション"] = &lexer.FuncItem{Name: "DOM部品オプション", Type: "const", Value: value.DictValue(domOptions)}
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
			fn:   p.cmdSetDOMParentAlias,
		},
		"DOM部品作成": { // @タグ名のDOM部品を現在の親要素へ追加してハンドルを返す // @DOMぶひんさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateDOMPart,
		},
		"DOMスキン設定": { // @「ボタン作成」「エディタ作成」などで適用するスキンを指定する // @DOMすきんせってい
			josi:       [][]string{{"を", "に", "の"}},
			returnNone: true,
			fn:         p.cmdSetDOMSkin,
		},
		"DOM要素作成": { // @画面へ未接続のTAG要素を作成してハンドルを返す // @DOMようそさくせい
			josi: [][]string{{"の", "を"}},
			fn:   p.cmdCreateDOMElement,
		},
		"DOM部品削除": { // @指定したDOM部品と子要素を削除する // @DOMぶひんさくじょ
			josi:       [][]string{{"の", "を"}},
			returnNone: true,
			fn:         p.cmdRemoveDOMPart,
		},
		"ラベル作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateLabel,
		},
		"エディタ作成": {
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateEditor,
		},
		"テキストエリア作成": { // @テキストの値を持つtextarea要素を追加してハンドルを返す // @てきすとえりあさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateTextArea,
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
		"キャンバス作成": { // @大きさ[幅,高さ]のcanvas要素を追加してハンドルを返す // @きゃんばすさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateCanvas,
		},
		"画像作成": { // @URLを指定したimg要素を追加してハンドルを返す // @がぞうさくせい
			josi: [][]string{{"の", "から"}},
			fn:   p.cmdCreateImage,
		},
		"改行作成": { // @br要素を追加してハンドルを返す // @かいぎょうさくせい
			josi: [][]string{},
			fn:   p.cmdCreateBreak,
		},
		"チェックボックス作成": { // @ラベル付きチェックボックスを追加してinput要素のハンドルを返す // @ちぇっくぼっくすさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateCheckbox,
		},
		"セレクトボックス作成": { // @配列の選択肢を持つselect要素を追加してハンドルを返す // @せれくとぼっくすさくせい
			josi: [][]string{{"の"}},
			fn:   p.cmdCreateSelect,
		},
		"セレクトボックスアイテム設定": { // @select要素の選択肢を配列の内容へ差し替える // @せれくとぼっくすあいてむせってい
			josi:       [][]string{{"を"}, {"へ", "に"}},
			returnNone: true,
			fn:         p.cmdSetSelectItems,
		},
		"色選択ボックス作成": { // @input[type=color]要素を追加してハンドルを返す // @いろせんたくぼっくすさくせい
			josi: [][]string{},
			fn:   p.cmdCreateColorInput,
		},
		"日付選択ボックス作成": { // @input[type=date]要素を追加してハンドルを返す // @ひづけせんたくぼっくすさくせい
			josi: [][]string{},
			fn:   p.cmdCreateDateInput,
		},
		"パスワード入力エディタ作成": { // @初期値を持つinput[type=password]要素を追加してハンドルを返す // @ぱすわーどにゅうりょくえでぃたさくせい
			josi: [][]string{{"の", "で"}},
			fn:   p.cmdCreatePasswordInput,
		},
		"値指定バー作成": { // @範囲[最小,最大,値]のinput[type=range]要素を追加してハンドルを返す // @あたいしていばーさくせい
			josi: [][]string{{"の", "で"}},
			fn:   p.cmdCreateRangeInput,
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

// domParentVar は追加先の親要素を保持するシステム変数名。本家と同じく、
// この変数が唯一の正であり、Screen側には親要素の控えを持たない。
const domParentVar = "DOM親要素"

func (p *Plugin) cmdSetDOMParent(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.setDOMParent(ctx, args, "DOM親要素設定")
}

func (p *Plugin) cmdSetDOMParentAlias(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.setDOMParent(ctx, args, "DOM親部品設定")
}

func (p *Plugin) setDOMParent(ctx stdlib.Context, args []value.Value, command string) (value.Value, error) {
	handle, err := p.resolveDOMParent(arg(args, 0), command)
	if err != nil {
		return value.Null(), err
	}
	p.screen.setParent(handle)
	ctx.SetSysVar(domParentVar, value.Number(float64(handle)))
	return value.Number(float64(handle)), nil
}

// currentParent は部品の追加先ハンドルを返す。『DOM親要素』へ直接代入された
// 値も追随したいので、システム変数が使えるならそちらを正とし、使えないときだけ
// Screen側の控えを使う。
func (p *Plugin) currentParent(ctx stdlib.Context, command string) (int, error) {
	if !p.domParentVarUsable(ctx) {
		return p.screen.parent(), nil
	}
	target := ctx.SysVar(domParentVar)
	switch target.Kind() {
	case value.KindUndefined, value.KindNull:
		return 0, nil
	}
	handle, err := p.resolveDOMParent(target, command)
	if err != nil {
		return 0, err
	}
	p.screen.setParent(handle)
	return handle, nil
}

// domParentVarUsable は『DOM親要素』への代入がVMに届くかを一度だけ調べる。
// VMはプログラム中に名前が現れない変数に記憶領域を割り当てないため、
// SetSysVarが黙って捨てられることがある。書いて読み直すことでそれを見分ける。
// 調べたあとは元の値へ必ず戻す。
func (p *Plugin) domParentVarUsable(ctx stdlib.Context) bool {
	p.domParentMu.Lock()
	defer p.domParentMu.Unlock()
	if p.domParentProbed {
		return p.domParentUsable
	}
	p.domParentProbed = true
	saved := ctx.SysVar(domParentVar)
	const probe = -12345
	ctx.SetSysVar(domParentVar, value.Number(probe))
	if n, ok := ctx.SysVar(domParentVar).Number(); ok && n == probe {
		p.domParentUsable = true
	}
	ctx.SetSysVar(domParentVar, saved)
	return p.domParentUsable
}

func (p *Plugin) resolveDOMParent(target value.Value, command string) (int, error) {
	if selector, ok := target.String(); ok {
		if selector == "" {
			return 0, nil
		}
		if handle, found := p.screen.query(selector); found {
			return handle, nil
		}
		if handle, found := p.screen.queryByID(selector); found {
			return handle, nil
		}
		return 0, fmt.Errorf("『%s』で要素『%s』が見つかりません。", command, selector)
	}
	if number, ok := target.Number(); ok && number == 0 {
		return 0, nil
	}
	handle, err := handleValue(target)
	if err != nil {
		return 0, err
	}
	if !p.screen.hasNode(handle) {
		return 0, fmt.Errorf("『%s』で画面部品ハンドル『%d』が見つかりません。", command, handle)
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

func (p *Plugin) cmdSetDOMSkin(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	skin := value.ToString(arg(args, 0))
	usable := p.domSkinVarUsable(ctx)
	p.domSkinMu.Lock()
	p.domSkin = skin
	p.domSkinMu.Unlock()
	if usable {
		ctx.SetSysVar("DOMスキン", value.String(skin))
	}
	return value.Undefined(), nil
}

func (p *Plugin) currentDOMSkin(ctx stdlib.Context) string {
	if p.domSkinVarUsable(ctx) {
		skin, _ := ctx.SysVar("DOMスキン").String()
		return skin
	}
	p.domSkinMu.Lock()
	defer p.domSkinMu.Unlock()
	return p.domSkin
}

// domSkinVarUsable は『DOMスキン』への代入がVMに届くかを一度だけ調べる。
// DOMスキン設定だけを使うソースでは定数の初期値は読めても書き込み先がない。
func (p *Plugin) domSkinVarUsable(ctx stdlib.Context) bool {
	p.domSkinMu.Lock()
	defer p.domSkinMu.Unlock()
	if p.domSkinProbed {
		return p.domSkinUsable
	}
	p.domSkinProbed = true
	saved := ctx.SysVar("DOMスキン")
	probe := value.String("\x00gonako-dom-skin-probe")
	ctx.SetSysVar("DOMスキン", probe)
	if got, ok := ctx.SysVar("DOMスキン").String(); ok && got == "\x00gonako-dom-skin-probe" {
		p.domSkinUsable = true
	}
	ctx.SetSysVar("DOMスキン", saved)
	return p.domSkinUsable
}

// applyDOMSkin は本家と同じく、選択名に対応する関数がある場合だけ呼ぶ。
// コールバックには元のタグ名と、作成した部品の数値ハンドルを渡す。
func (p *Plugin) applyDOMSkin(ctx stdlib.Context, tag string, handle int) error {
	dict, ok := ctx.SysVar("DOMスキン辞書").Dict()
	if !ok || dict == nil {
		return nil
	}
	item, found := dict.Get(p.currentDOMSkin(ctx))
	if !found {
		return nil
	}
	fn, ok := item.Func()
	if !ok || fn == nil {
		return nil
	}
	_, err := ctx.CallFunc(fn, []value.Value{value.String(tag), value.Number(float64(handle))})
	return err
}

func (p *Plugin) autoBreakEnabled(ctx stdlib.Context) bool {
	options, ok := ctx.SysVar("DOM部品オプション").Dict()
	if !ok || options == nil {
		return false
	}
	enabled, found := options.Get("自動改行")
	return found && value.ToBool(enabled)
}

func (p *Plugin) createPartHandle(ctx stdlib.Context, command, tag, text, html, name string) (int, error) {
	parent, err := p.currentParent(ctx, command)
	if err != nil {
		return 0, err
	}
	h := p.screen.create(tag, text, html, name, parent)
	if err := p.finishPart(ctx, tag, h, parent); err != nil {
		return 0, err
	}
	return h, nil
}

func (p *Plugin) finishPart(ctx stdlib.Context, tag string, handle, parent int) error {
	if err := p.applyDOMSkin(ctx, tag, handle); err != nil {
		return err
	}
	// 自動改行はスキンを適用せず、作成した部品と同じ親へ追加する。
	// brにもハンドルを割り当て、Go側モデルとWebView側の要素を一致させる。
	if p.autoBreakEnabled(ctx) {
		p.screen.create("br", "", "", "", parent)
	}
	return nil
}

func (p *Plugin) createPart(ctx stdlib.Context, command, tag, text, html, name string) (value.Value, error) {
	h, err := p.createPartHandle(ctx, command, tag, text, html, name)
	if err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(h)), nil
}

func (p *Plugin) cmdCreateDOMPart(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	target := arg(args, 0)
	if _, isString := target.String(); !isString {
		handle, err := handleValue(target)
		if err != nil {
			return value.Undefined(), err
		}
		parent, err := p.currentParent(ctx, "DOM部品作成")
		if err != nil {
			return value.Undefined(), err
		}
		if err := p.screen.append(handle, parent, "DOM部品作成"); err != nil {
			return value.Undefined(), err
		}
		return value.Number(float64(handle)), nil
	}
	tag := strings.TrimSpace(value.ToString(target))
	if !validDOMTag(tag) {
		return value.Undefined(), fmt.Errorf("『DOM部品作成』のタグ名『%s』が不正です。", tag)
	}
	return p.createPart(ctx, "DOM部品作成", strings.ToLower(tag), "", "", "")
}

func (p *Plugin) cmdCreateDOMElement(_ stdlib.Context, args []value.Value) (value.Value, error) {
	tag := strings.TrimSpace(value.ToString(arg(args, 0)))
	if !validDOMTag(tag) {
		return value.Undefined(), fmt.Errorf("『DOM要素作成』のタグ名『%s』が不正です。", tag)
	}
	handle := p.screen.createDetached(strings.ToLower(tag))
	return value.Number(float64(handle)), nil
}

func (p *Plugin) cmdRemoveDOMPart(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	target := arg(args, 0)
	handle := 0
	if selector, ok := target.String(); ok {
		var found bool
		handle, found = p.screen.query(selector)
		if !found {
			handle, found = p.screen.queryByID(selector)
		}
		if !found {
			return value.Undefined(), fmt.Errorf("『DOM部品削除』で要素『%s』が見つかりません。", selector)
		}
	} else {
		var err error
		handle, err = handleValue(target)
		if err != nil {
			return value.Undefined(), err
		}
	}
	current, _ := p.currentParent(ctx, "DOM部品削除")
	removed, err := p.screen.remove(handle, "DOM部品削除")
	if err != nil {
		return value.Undefined(), err
	}
	if _, ok := removed[current]; ok {
		p.screen.setParent(0)
		ctx.SetSysVar(domParentVar, value.Number(0))
	}
	return value.Undefined(), nil
}

func (p *Plugin) cmdCreateLabel(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart(ctx, "ラベル作成", "span", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateEditor(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart(ctx, "エディタ作成", "input", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateTextArea(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart(ctx, "テキストエリア作成", "textarea", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateButton(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart(ctx, "ボタン作成", "button", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateSubmit(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createPart(ctx, "送信ボタン作成", "submit", value.ToString(arg(args, 0)), "", "")
}

func (p *Plugin) cmdCreateForm(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	form, err := p.createPartHandle(ctx, "フォーム作成", "form", "", "", "")
	if err != nil {
		return value.Undefined(), err
	}
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

func (p *Plugin) cmdCreateCanvas(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	h, err := p.createPartHandle(ctx, "キャンバス作成", "canvas", "", "", "")
	if err != nil {
		return value.Undefined(), err
	}
	width, height := value.Undefined(), value.Undefined()
	if size, ok := arg(args, 0).Array(); ok && size != nil {
		width, height = size.Get(0), size.Get(1)
	}
	dimensions := map[string]string{"width": value.ToString(width), "height": value.ToString(height)}
	if err := p.screen.setAttributes(h, dimensions); err != nil {
		return value.Undefined(), err
	}
	if err := p.screen.setStyles(h, dimensions); err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(h)), nil
}

func (p *Plugin) cmdCreateImage(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createAttributedPart(ctx, "画像作成", "img", "", map[string]string{"src": value.ToString(arg(args, 0))})
}

func (p *Plugin) cmdCreateBreak(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
	return p.createPart(ctx, "改行作成", "br", "", "", "")
}

func (p *Plugin) cmdCreateCheckbox(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	parent, err := p.currentParent(ctx, "チェックボックス作成")
	if err != nil {
		return value.Undefined(), err
	}
	wrapper := p.screen.create("span", "", "", "", parent)
	inputID := fmt.Sprintf("nadesi-dom-%d", wrapper)
	input := p.screen.createWithAttributes("input", "on", "", "", wrapper, map[string]string{
		"type": "checkbox", "id": inputID,
	})
	p.screen.createWithAttributes("label", value.ToString(arg(args, 0)), "", "", wrapper, map[string]string{"for": inputID})
	if err := p.finishPart(ctx, "span", wrapper, parent); err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(input)), nil
}

func valuesFromArray(v value.Value) []string {
	array, ok := v.Array()
	if !ok || array == nil {
		return nil
	}
	items := make([]string, array.Len())
	for i := range items {
		items[i] = value.ToString(array.Get(i))
	}
	return items
}

func (p *Plugin) cmdCreateSelect(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	parent, err := p.currentParent(ctx, "セレクトボックス作成")
	if err != nil {
		return value.Undefined(), err
	}
	h := p.screen.create("select", "", "", "", parent)
	p.screen.appendOptions(h, valuesFromArray(arg(args, 0)))
	if err := p.finishPart(ctx, "select", h, parent); err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(h)), nil
}

func (p *Plugin) cmdSetSelectItems(_ stdlib.Context, args []value.Value) (value.Value, error) {
	target := arg(args, 1)
	var handle int
	if selector, ok := target.String(); ok {
		var found bool
		handle, found = p.screen.query(selector)
		if !found {
			return value.Undefined(), fmt.Errorf("『セレクトボックスアイテム設定』で要素『%s』が見つかりません。", selector)
		}
	} else {
		var err error
		handle, err = handleValue(target)
		if err != nil {
			return value.Undefined(), err
		}
	}
	return value.Undefined(), p.screen.replaceOptions(handle, valuesFromArray(arg(args, 0)))
}

func (p *Plugin) createAttributedPart(ctx stdlib.Context, command, tag, text string, attrs map[string]string) (value.Value, error) {
	h, err := p.createPartHandle(ctx, command, tag, text, "", "")
	if err != nil {
		return value.Undefined(), err
	}
	if len(attrs) > 0 {
		if err := p.screen.setAttributes(h, attrs); err != nil {
			return value.Undefined(), err
		}
	}
	return value.Number(float64(h)), nil
}

func (p *Plugin) cmdCreateColorInput(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
	return p.createAttributedPart(ctx, "色選択ボックス作成", "input", "#000000", map[string]string{"type": "color"})
}

func (p *Plugin) cmdCreateDateInput(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
	return p.createAttributedPart(ctx, "日付選択ボックス作成", "input", "", map[string]string{"type": "date"})
}

func (p *Plugin) cmdCreatePasswordInput(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	return p.createAttributedPart(ctx, "パスワード入力エディタ作成", "input", value.ToString(arg(args, 0)), map[string]string{"type": "password"})
}

func (p *Plugin) cmdCreateRangeInput(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	minimum, maximum, initial := value.Number(0), value.Number(100), value.Number(50)
	if ranges, ok := arg(args, 0).Array(); ok && ranges != nil && ranges.Len() >= 2 {
		minimum, maximum = ranges.Get(0), ranges.Get(1)
		if ranges.Len() >= 3 {
			initial = ranges.Get(2)
		} else {
			initial = value.Number(math.Floor((value.ToNumber(maximum) - value.ToNumber(minimum)) / 2))
		}
	}
	return p.createAttributedPart(ctx, "値指定バー作成", "input", value.ToString(initial), map[string]string{
		"type": "range", "min": value.ToString(minimum), "max": value.ToString(maximum),
	})
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
