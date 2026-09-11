package guilib

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func TestGuilibPlugin(t *testing.T) {
	p := New()
	funcs := p.FuncList()

	if _, ok := funcs["ウィンドウ作成"]; !ok {
		t.Fatalf("ウィンドウ作成 command not found in guilib")
	}
	for _, name := range []string{
		"ファイル選択", "保存ファイル選択", "フォルダ選択",
		"DOM親要素設定", "DOM親部品設定", "DOM部品作成",
		"DOM要素取得", "DOM要素ID取得", "DOM要素全取得",
		"DOMテキスト取得", "DOMテキスト変更", "HTML取得", "HTML変更", "DOM注目",
	} {
		if _, ok := funcs[name]; !ok {
			t.Fatalf("%s command not found in guilib", name)
		}
	}
	if parent := funcs["DOM親要素"]; parent == nil || parent.Type != "const" || parent.Value != 0 {
		t.Fatalf("DOM親要素 = %#v", parent)
	}

	reg := stdlib.NewRegistry(New())
	if reg.FuncList()["ウィンドウ作成"] == nil {
		t.Errorf("failed to register ウィンドウ作成 in stdlib.Registry")
	}

	// Test parseWindowConfig
	dict := value.NewDict()
	dict.Set("タイトル", value.String("マイアプリ"))
	sizeArr := value.NewArray(value.Number(800), value.Number(600))
	dict.Set("サイズ", value.ArrayValue(sizeArr))

	cfg := parseWindowConfig(value.DictValue(dict))
	if cfg.title != "マイアプリ" {
		t.Errorf("expected title 'マイアプリ', got '%s'", cfg.title)
	}
	if cfg.width != 800 || cfg.height != 600 {
		t.Errorf("expected size 800x600, got %dx%d", cfg.width, cfg.height)
	}
}

func TestDOMPartCreateUsesConfiguredParent(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	code := `「<section id="main"></section>」をHTML表示
「#main」にDOM親要素設定
部品=「article」のDOM部品作成
部品に「本文」をDOMテキスト設定
「見出し」のラベル作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}

	// HTML表示のラッパーが1、sectionが2、articleが3、ラベルが4。
	ops := screen.DrainOperations()
	if len(ops) != 4 {
		t.Fatalf("operations = %#v", ops)
	}
	if got := ops[1]; got.Type != "create" || got.Handle != 3 || got.Parent != 2 || got.Tag != "article" {
		t.Fatalf("DOM部品作成 operation = %#v", got)
	}
	if got := ops[3]; got.Type != "create" || got.Handle != 4 || got.Parent != 2 || got.Tag != "span" {
		t.Fatalf("ラベル作成 operation = %#v", got)
	}
	if text, err := screen.text(3); err != nil || text != "本文" {
		t.Fatalf("article text = %q, err = %v", text, err)
	}
}

func TestDOMParentAliasAcceptsIDAndHandle(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `「<div id="main"></div>」をHTML表示
「main」にDOM親部品設定
DOM親要素を表示
親=「section」のDOM部品作成
親にDOM親要素設定
「button」のDOM部品作成
0にDOM親要素設定
「footer」のDOM部品作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 4 || ops[1].Parent != 2 || ops[2].Parent != 3 || ops[3].Parent != 0 {
		t.Fatalf("operations = %#v", ops)
	}
	if got := strings.TrimSpace(out.String()); got != "2" {
		t.Fatalf("DOM親要素 = %q, want 2", got)
	}
}

func TestDOMPartCreateDefaultsToRoot(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	if err := vm.RunWithHostAndRegistry(`「DIV」のDOM部品作成`, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 1 || ops[0].Parent != 0 || ops[0].Tag != "div" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestExistingCreateCommandsUseConfiguredParent(t *testing.T) {
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	code := `「<div id="parts"></div>」をHTML表示
「#parts」にDOM親要素設定
「見出し」のラベル作成
「入力」のエディタ作成
「実行」のボタン作成
「送信」の送信ボタン作成
{}で「名前=太郎」をフォーム作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}

	// HTML表示内のdivが2。直後に作る5部品はすべてその子になる。
	for handle := 3; handle <= 7; handle++ {
		if got := screen.nodes[handle].parent; got != 2 {
			t.Fatalf("handle %d parent = %d, want 2", handle, got)
		}
	}
	// フォームが作るラベル、入力欄、送信ボタンはフォーム自身の子になる。
	for handle := 8; handle <= 10; handle++ {
		if got := screen.nodes[handle].parent; got != 7 {
			t.Fatalf("form child %d parent = %d, want 7", handle, got)
		}
	}
}

func TestDOMParentSysVarAssignmentIsHonored(t *testing.T) {
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	// 『DOM親要素』へ直接代入しても、実際の追加先が追随すること。
	code := `「<div id="main"></div>」をHTML表示
2をDOM親要素に代入
「p」のDOM部品作成
0をDOM親要素に代入
「footer」のDOM部品作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 3 || ops[1].Parent != 2 || ops[2].Parent != 0 {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestDOMParentSysVarAssignmentWinsOverSetCommand(t *testing.T) {
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	// 設定命令のあとに代入した場合も、あとから代入した値が優先されること。
	code := `「<div id="main"><span id="sub"></span></div>」をHTML表示
「#main」にDOM親要素設定
3をDOM親要素に代入
「p」のDOM部品作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if got := ops[len(ops)-1]; got.Parent != 3 {
		t.Fatalf("operation = %#v", got)
	}
}

func TestDOMParentSysVarRejectsUnknownHandle(t *testing.T) {
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	err := vm.RunWithHostAndRegistry("99をDOM親要素に代入\n「p」のDOM部品作成", "gui.nako3", registry, host)
	if err == nil || !strings.Contains(err.Error(), "DOM部品作成") || !strings.Contains(err.Error(), "見つかりません") {
		t.Fatalf("error = %v", err)
	}
	_ = screen
}

func TestDOMParentAliasReportsOwnName(t *testing.T) {
	registry := stdlib.NewRegistry(NewWithScreen(NewScreen()))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	err := vm.RunWithHostAndRegistry(`「#missing」にDOM親部品設定`, "gui.nako3", registry, host)
	if err == nil || !strings.Contains(err.Error(), "『DOM親部品設定』") {
		t.Fatalf("error = %v", err)
	}
}

func TestDOMElementIDLookupIgnoresEmptyID(t *testing.T) {
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	// 空文字はid無しのノードに一致してはいけない。
	code := `「<div id="main"></div>」をHTML表示
S=「」
SにDOM親要素設定
「p」のDOM部品作成`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if got := ops[len(ops)-1]; got.Parent != 0 {
		t.Fatalf("operation = %#v", got)
	}
	if handle, found := screen.queryByID(""); found {
		t.Fatalf("queryByID(\"\") = %d, want not found", handle)
	}
}

func TestDOMPartCreateRejectsInvalidTargets(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{name: "missing parent", code: `「#missing」にDOM親要素設定`, want: "見つかりません"},
		{name: "invalid tag", code: `「div script」のDOM部品作成`, want: "タグ名"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen := NewScreen()
			registry := stdlib.NewRegistry(NewWithScreen(screen))
			host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
			err := vm.RunWithHostAndRegistry(tc.code, "gui.nako3", registry, host)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDOMElementQueryCommands(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `A=「最初」のラベル作成
Aに{"id":"first","class":"item hot","data-kind":"label"}をDOM属性一括設定
B=「二番目」のボタン作成
Bに{"id":"second","class":"item"}をDOM属性一括設定
「first」をDOM要素ID取得してテキスト取得して表示
「button.item」をDOM要素取得してテキスト取得して表示
一覧=「.item」をDOM要素全取得
一覧の要素数を表示`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out.String()), "最初\n二番目\n2"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	if got := screen.queryAll(`[data-kind="label"]`); len(got) != 1 || got[0] != 1 {
		t.Fatalf("attribute selector result = %#v, want [1]", got)
	}
	missing, err := p.cmdGetElementByID(nil, []value.Value{value.String("missing")})
	if err != nil || missing.Kind() != value.KindNull {
		t.Fatalf("missing element = %#v, err=%v; want null", missing, err)
	}
	handle := value.Number(2)
	passedThrough, err := p.cmdGetElement(nil, []value.Value{handle})
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := passedThrough.Number(); !ok || number != 2 {
		t.Fatalf("handle pass-through = %#v", passedThrough)
	}
}

func TestDOMTextHTMLAndFocusCommands(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `A=「最初」のラベル作成
Aに「変更後」をDOMテキスト変更
AのDOMテキスト取得を表示
Aに「<b>太字</b>」をHTML変更
AのHTML取得を表示
AをDOM注目`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out.String()), "変更後\n<b>太字</b>"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	ops := screen.DrainOperations()
	if len(ops) != 4 || ops[1].Type != "text" || ops[2].Type != "html" || ops[3].Type != "focus" {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestDOMQueryFindsElementsInsideDisplayedHTML(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<section><input id="name" class="field main" value="太郎"><button class="main">送信</button></section>`)

	input, ok := screen.query("input#name.field")
	if !ok {
		t.Fatal("input inside HTML表示 was not found")
	}
	buttons := screen.queryAll("button.main")
	if len(buttons) != 1 {
		t.Fatalf("button query result = %#v", buttons)
	}
	if text, err := screen.text(buttons[0]); err != nil || text != "送信" {
		t.Fatalf("button text = %q, err = %v", text, err)
	}
	if err := screen.focus(input); err != nil {
		t.Fatal(err)
	}
	p := NewWithScreen(screen)
	if _, err := p.cmdFocus(nil, []value.Value{value.String("button.main")}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.cmdFocus(nil, []value.Value{value.String("#missing")}); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 3 || !strings.Contains(ops[0].HTML, `data-gonako-handle=`) || ops[1].Type != "focus" || ops[2].Type != "focus" || ops[2].Handle != buttons[0] {
		t.Fatalf("operations = %#v", ops)
	}
}

func TestFileDialogCommands(t *testing.T) {
	var openDir, openExtension string
	var saveDir, saveName, saveExtension string
	var folderDir string
	p := &Plugin{dialogs: fileDialogs{
		open: func(defaultDir, extension string) (string, error) {
			openDir, openExtension = defaultDir, extension
			return "/tmp/opened.txt", nil
		},
		save: func(defaultDir, defaultName, extension string) (string, error) {
			saveDir, saveName, saveExtension = defaultDir, defaultName, extension
			return "/tmp/saved.txt", nil
		},
		folder: func(defaultDir string) (string, error) {
			folderDir = defaultDir
			return "/tmp/selected", nil
		},
	}}
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `# オープンダイアログ
「.txt」のファイル選択
それを表示
# 保存ダイアログ
「.txt」の保存ファイル選択
それを表示
# フォルダ選択ダイアログ
母艦パスでフォルダ選択
それを表示`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out.String()), "/tmp/opened.txt\n/tmp/saved.txt\n/tmp/selected"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if openDir == "" || openExtension != ".txt" {
		t.Errorf("open args = (%q, %q)", openDir, openExtension)
	}
	if saveDir == "" || saveName != "新規ファイル.txt" || saveExtension != ".txt" {
		t.Errorf("save args = (%q, %q, %q)", saveDir, saveName, saveExtension)
	}
	if folderDir == "" {
		t.Error("folder dialog did not receive 母艦パス")
	}
}

func TestScreenCommandsAndClickEvent(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `エディタ=「初期値」のエディタ作成
ボタン=「送信」のボタン作成
ボタンをクリックした時には
　エディタのテキスト取得
　それを表示
ここまで`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 3 || ops[0].Tag != "input" || ops[1].Tag != "button" || ops[2].Type != "listen" {
		t.Fatalf("operations = %#v", ops)
	}
	if err := screen.DispatchEvent(2, "click", map[string]string{"1": "更新後"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "更新後" {
		t.Fatalf("callback output = %q, want 更新後", got)
	}
}

func TestScreenStyleAttributeAndInvalidHandle(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	host := vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)
	code := `ラベル=「名前」のラベル作成
ラベルに{"color":"red","fontWeight":"bold"}をDOMスタイル一括設定
ラベルに{"title":"説明","name":"label1"}をDOM属性一括設定
ラベルに「変更後」をテキスト設定`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	ops := screen.DrainOperations()
	if len(ops) != 4 {
		t.Fatalf("operation count = %d, want 4: %#v", len(ops), ops)
	}
	if got, err := screen.text(1); err != nil || got != "変更後" {
		t.Fatalf("text = %q, err = %v", got, err)
	}
	if err := screen.setText(99, "x"); err == nil || !strings.Contains(err.Error(), "99") {
		t.Fatalf("invalid handle error = %v", err)
	}
}

func TestFormSubmitProvidesNamedValues(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	registry := stdlib.NewRegistry(p)
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	code := `設定={"method":"POST"}
フォーム=設定で「名前=太郎」をフォーム作成
フォームをフォーム送信した時には
　フォーム値["名前"]を表示
ここまで`
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	// form=1, label=2, editor=3, submit=4
	if err := screen.DispatchEvent(1, "submit", map[string]string{"3": "花子"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "花子" {
		t.Fatalf("callback output = %q, want 花子", got)
	}
}
