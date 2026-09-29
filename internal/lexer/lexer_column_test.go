package lexer_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Issue #231: 数値に単位が続くと、単位を消費したのに数値の長さを
// Columnへ加算し、後続トークンのColumnが実際より先に進んでいた。
func TestNumberUnitColumn(t *testing.T) {
	tests := []struct {
		name string
		code string
		at   string // Column を確かめる対象の字句
		want int
	}{
		// A=123円+B: `+` の実位置はoffset6 → Columnは7
		{"単位 円", `A=123円+B`, "+", 7},
		{"単位 円の後のB", `A=123円+B`, "B", 8},
		// A=1px+B: `+` の実位置はoffset5 → Columnは6
		{"CSS単位 px", `A=1px+B`, "+", 6},
		{"CSS単位 pxの後のB", `A=1px+B`, "B", 7},
		// 比較用: 単位なしなら従来通り
		{"単位なし", `A=123+B`, "+", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(tt.code)), 0, "main.nako3")
			if err != nil {
				t.Fatalf("Tokenize(%q) failed: %v", tt.code, err)
			}
			var found *lexer.Token
			for i := range raw {
				if raw[i].StringValue() == tt.at {
					found = &raw[i]
				}
			}
			if found == nil {
				t.Fatalf("%q を含むトークンが無い: %v", tt.at, summarize(raw))
			}
			if found.Column != tt.want {
				t.Errorf("『%s』の Column = %d, want %d", tt.at, found.Column, tt.want)
			}
		})
	}
}
