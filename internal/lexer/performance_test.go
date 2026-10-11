package lexer

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTokenizeConsumedPositions(t *testing.T) {
	for _, prefix := range []string{
		"　", "A=1円を表示\n\t", "A=10pxを表示; ",
		"🌿😀あ🌿を表示\n", "「あ{変数}😀」を表示\n",
		"/*あ\n😀*/", "#あ😀\n", "${特別😀}を表示\n",
		"等しい間 ", "変数以上 ", "変数以下を ", "A_\n", "‰",
		strings.Repeat("変数=1を表示\n", 1000),
	} {
		t.Run(fmt.Sprintf("%.30s", prefix), func(t *testing.T) {
			tokens, err := Tokenize(prefix+"終端", 0, "test.nako3")
			if err != nil {
				t.Fatal(err)
			}
			last := tokens[len(tokens)-1]
			if last.StringValue() != "終端" || last.Offset != utf8.RuneCountInString(prefix) || last.Length != 2 {
				t.Fatalf("末尾トークンの位置が不正です: %+v", last)
			}
		})
	}
}

func TestLongWord(t *testing.T) {
	word := "ア" + strings.Repeat("あア", 10000)
	for _, trim := range []bool{false, true} {
		rp, err := cbWordParser(word+"を表示", trim)
		if err != nil {
			t.Fatal(err)
		}
		want := word
		if trim {
			want = strings.ReplaceAll(word, "あ", "")
		}
		if rp.res != want || rp.josi != "を" || rp.src != "表示" {
			t.Fatalf("長い識別子の解析が不正です: 値の長さ=%d, 助詞=%q, 残り=%q", len(rp.res), rp.josi, rp.src)
		}
	}
}

func TestTokenizeManyStatementPositions(t *testing.T) {
	const n = 1000
	tokens, err := Tokenize(strings.Repeat("変数=1を表示\n", n), 0, "test.nako3")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != n*5 {
		t.Fatalf("トークン数 = %d, want %d", len(tokens), n*5)
	}
	positions := []struct{ offset, length int }{{0, 2}, {2, 1}, {3, 2}, {5, 2}, {7, 1}}
	for i, token := range tokens {
		line := i / 5
		want := positions[i%5]
		if token.Offset != line*8+want.offset || token.Length != want.length || token.Line != line || token.Column != want.offset+1 {
			t.Fatalf("トークン%dの位置が不正です: %+v", i, token)
		}
	}
}

func TestTypesString(t *testing.T) {
	if got := TypesString(nil, "、"); got != "" {
		t.Fatalf("空のトークン列: %q", got)
	}
	tokens := []Token{{Type: TypeWord}, {Type: TypeMoshi}, {Type: TypeString}}
	if got := TypesString(tokens, "、"); got != "word、もし、string" {
		t.Fatalf("トークン列: %q", got)
	}
}

// 規模を倍にしたときの実行時間と割り当て量を比較する (#320)。
func BenchmarkTokenizeManyStatements(b *testing.B) {
	for _, n := range []int{1000, 2000, 4000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			code := strings.Repeat("変数=1を表示\n", n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Tokenize(code, 0, "bench.nako3"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLongWord(b *testing.B) {
	for _, n := range []int{1000, 2000, 4000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			code := strings.Repeat("あア", n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := cbWordParser(code, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTypesString(b *testing.B) {
	for _, n := range []int{1000, 2000, 4000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			tokens := make([]Token, n)
			for i := range tokens {
				tokens[i].Type = TypeWord
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if len(TypesString(tokens, ",")) != n*5-1 {
					b.Fatal("トークン数が一致しません")
				}
			}
		})
	}
}
