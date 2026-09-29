package stdlib_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// TestNanCheck は issue #223 の回帰テスト。本家では『非数判定』が
// Number.isNaN（値そのものが数値型のNaNのときのみ真）、『NAN判定』が
// isNaN（数値へ変換してから判定）という別の意味を持つ。
func TestNanCheck(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	tests := []struct {
		name string
		arg  value.Value
		want bool
	}{
		// 非数判定: 数値型のNaNにのみ真を返す
		{"非数判定", value.Number(math.NaN()), true},
		{"非数判定", value.Number(12), false},
		{"非数判定", value.Number(math.Inf(1)), false},
		{"非数判定", value.String("12x"), false}, // 文字列は変換せず偽
		{"非数判定", value.String("あ"), false},
		{"非数判定", value.String("NaN"), false},
		{"非数判定", value.Undefined(), false},
		{"非数判定", value.Null(), false},
		{"非数判定", value.Bool(true), false},
		// NAN判定: isNaN相当で、先に数値へ変換してから判定する
		{"NAN判定", value.Number(math.NaN()), true},
		{"NAN判定", value.Number(12), false},
		{"NAN判定", value.String("12x"), true}, // Number("12x") は NaN なので真
		{"NAN判定", value.String("あ"), true},
		{"NAN判定", value.String("12"), false},
		{"NAN判定", value.Undefined(), true},
		{"NAN判定", value.Null(), false}, // Number(null) は 0 なので偽
		{"NAN判定", value.Bool(true), false},
	}
	for _, tt := range tests {
		e, ok := r.Lookup(tt.name)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が実装されていない", tt.name)
		}
		got, err := e.Fn(ctx, []value.Value{tt.arg})
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		b, isBool := got.Bool()
		if !isBool || b != tt.want {
			t.Errorf("%s(%v) = %v, want %v", tt.name, value.ToString(tt.arg), value.ToString(got), tt.want)
		}
	}
}
