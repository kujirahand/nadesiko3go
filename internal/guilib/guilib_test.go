package guilib

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func TestGuilibPlugin(t *testing.T) {
	p := New()
	funcs := p.FuncList()

	if _, ok := funcs["ウィンドウ作成"]; !ok {
		t.Fatalf("ウィンドウ作成 command not found in guilib")
	}

	item := funcs["ウィンドウ作成"]
	if item.Type != "func" {
		t.Errorf("expected type func, got %s", item.Type)
	}

	reg := stdlib.NewRegistry(New())
	if reg.FuncList()["ウィンドウ作成"] == nil {
		t.Errorf("failed to register ウィンドウ作成 in stdlib.Registry")
	}
}
