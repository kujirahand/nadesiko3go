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
	if _, ok := funcs["ウィンドウ設定"]; !ok {
		t.Fatalf("ウィンドウ設定 command not found in guilib")
	}

	reg := stdlib.NewRegistry(New())
	if reg.FuncList()["ウィンドウ設定"] == nil {
		t.Errorf("failed to register ウィンドウ設定 in stdlib.Registry")
	}

	// Test config setting
	dict := value.NewDict()
	dict.Set("タイトル", value.String("マイアプリ"))
	sizeArr := value.NewArray(value.Number(800), value.Number(600))
	dict.Set("サイズ", value.ArrayValue(sizeArr))

	impls := p.Impls()
	_, err := impls["ウィンドウ設定"](nil, []value.Value{value.DictValue(dict)})
	if err != nil {
		t.Fatalf("ウィンドウ設定 failed: %v", err)
	}

	if p.title != "マイアプリ" {
		t.Errorf("expected title 'マイアプリ', got '%s'", p.title)
	}
	if p.width != 800 || p.height != 600 {
		t.Errorf("expected size 800x600, got %dx%d", p.width, p.height)
	}
}
