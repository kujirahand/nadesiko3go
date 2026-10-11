package parser

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// 長い文字列リテラル・コメント・埋め込み式の解析が、長さの2乗で遅くならない
// ことを確かめるベンチマーク (#315)。
// 実行: go test ./internal/parser -run xxx -bench LongSource
// 2乗に戻ると、規模を2倍にしたときの ns/op が4倍近くになる。
func BenchmarkLongSource(b *testing.B) {
	cases := []struct{ name, code string }{
		{"literal_100k", "S=「" + strings.Repeat("あ", 100000) + "」\nU=S[5]"},
		{"literal_400k", "S=「" + strings.Repeat("あ", 400000) + "」\nU=S[5]"},
		{"literal_ex_400k", "S=『" + strings.Repeat("あ", 400000) + "』\nU=S[5]"},
		{"block_comment_400k", "S=1\n/*" + strings.Repeat("あ", 400000) + "*/\nU=S"},
		{"line_comment_400k", "S=1\n#" + strings.Repeat("あ", 400000) + "\nU=S"},
		{"interpolation_3k", "S=1\nT=「" + strings.Repeat("あ{S}", 3000) + "」\nU=T[5]"},
		{"interpolation_12k", "S=1\nT=「" + strings.Repeat("あ{S}", 12000) + "」\nU=T[5]"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ParseSource(c.code, "a.nako3", lexer.FuncList{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
