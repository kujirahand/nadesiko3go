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
		"DOM要素取得", "DOM要素ID取得", "DOM要素全取得",
		"DOMテキスト取得", "DOMテキスト変更", "HTML取得", "HTML変更", "DOM注目",
	} {
		if _, ok := funcs[name]; !ok {
			t.Fatalf("%s command not found in guilib", name)
		}
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
