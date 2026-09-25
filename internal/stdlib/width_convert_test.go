package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// TestZenkakuRecordSeparatorsNotMangled は本家 TypeScript版 #2479 の
// 回帰テストである。U+FF5F（｟）は英数記号半角変換の対象範囲外であり、
// DEL制御文字(0x7f)へ誤変換してはならない。隣接するU+FF5E（～）は
// これまで通り半角チルダへ変換される。
func TestZenkakuRecordSeparatorsNotMangled(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	e, ok := r.Lookup("英数記号半角変換")
	if !ok || e.Fn == nil {
		t.Fatal("英数記号半角変換 が登録されていない")
	}

	tests := []struct{ in, want string }{
		{"～", "~"}, // ～ → ~ (境界の直前は変換対象)
		{"｟", "｟"}, // ｟ は変換対象外のまま
		{"｠", "｠"}, // ｠ も変換対象外のまま
		{"ＡＢＣ！", "ABC!"},
	}
	for _, tt := range tests {
		got, err := e.Fn(ctx, []value.Value{value.String(tt.in)})
		if err != nil {
			t.Fatalf("英数記号半角変換(%q): %v", tt.in, err)
		}
		if value.ToString(got) != tt.want {
			t.Errorf("英数記号半角変換(%q) = %q, want %q", tt.in, value.ToString(got), tt.want)
		}
	}
}
