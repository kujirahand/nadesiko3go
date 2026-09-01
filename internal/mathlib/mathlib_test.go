package mathlib

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestPluginMathCommands(t *testing.T) {
	p := New()
	impls := p.Impls()
	tests := []struct {
		name string
		args []value.Value
		want float64
	}{
		{"SIN", []value.Value{value.Number(1)}, math.Sin(1)},
		{"HYPOT", []value.Value{value.Number(3), value.Number(4)}, 5},
		{"LOGN", []value.Value{value.Number(10), value.Number(10)}, 1},
		{"四捨五入", []value.Value{value.Number(3.5)}, 4},
		{"小数点四捨五入", []value.Value{value.Number(3.15), value.Number(1)}, 3.2},
	}
	for _, tt := range tests {
		got, err := impls[tt.name](nil, tt.args)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		n, _ := got.Number()
		if math.Abs(n-tt.want) > 1e-12 {
			t.Errorf("%s = %v, want %v", tt.name, n, tt.want)
		}
	}
}

func TestEveryMathCommandHasImplementation(t *testing.T) {
	p := New()
	impls := p.Impls()
	for name := range p.FuncList() {
		if impls[name] == nil {
			t.Errorf("数学命令『%s』に実装がない", name)
		}
	}
}
