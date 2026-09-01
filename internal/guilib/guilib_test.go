package guilib

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestGuilibPlugin(t *testing.T) {
	p := New()
	funcs := p.FuncList()

	if _, ok := funcs["ウィンドウ作成"]; !ok {
		t.Fatalf("ウィンドウ作成 command not found in guilib")
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
