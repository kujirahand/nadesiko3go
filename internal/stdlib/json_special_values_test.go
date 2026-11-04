package stdlib_test

// Issue #374: JSONエンコードの特殊値テスト。
// 無限大と NaN は、JavaScript の JSON.stringify と同じく null になる。

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestJSONEncodeSpecialValues(t *testing.T) {
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("JSONエンコード")
	if !ok {
		t.Fatal("JSONエンコード が見つかりません")
	}
	tests := []struct {
		name string
		in   float64
	}{
		{"正の無限大", math.Inf(1)},
		{"負の無限大", math.Inf(-1)},
		{"NaN", math.NaN()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := e.Fn(newContext(), []value.Value{value.Number(tt.in)})
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got := value.ToString(v); got != "null" {
				t.Errorf("JSONエンコード(%v) = %q, want %q", tt.in, got, "null")
			}
		})
	}
}
