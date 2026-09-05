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
	for _, name := range []string{"ファイル選択", "保存ファイル選択", "フォルダ選択"} {
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
