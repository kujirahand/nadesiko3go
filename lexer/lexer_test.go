package lexer

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/token"
)

func TestLexer1(t *testing.T) {
	testOkurigana(t, "表示する", "表示")
	testOkurigana(t, "あおい空だ", "あおい空")
	execLex(t, "漢字=32", "漢字 = 32 EOS")
	execLex(t, "32を表示", "32を 表示 EOS")
	execLex(t, "a=1+2*3", "a = 1 + 2 * 3 EOS")
	execLex(t, "3を表示\n4を表示", "3を 表示 LF 4を 表示 EOS")
	execLex(t, "3を表示する", "3を 表示 EOS")
}
func TestLexer2(t *testing.T) {
	execLex(t, "a=32", "a = 32 EOS")
	execLex(t, "a='abc'", "a = 'abc' EOS")
	execLex(t, "a=「abc」", "a = 「abc」 EOS")
	execLex(t, "a=「abc{~}abc」", "a = 「abc\nabc」 EOS")
}

func testOkurigana(t *testing.T, code, expected string) {
	s := DeleteOkurigana(code)
	if s != expected {
		t.Fatalf("okurigana error [%s] != [%s]", s, expected)
	}
}

func execLex(t *testing.T, code, expected string) {
	p := NewLexer(code, 0)
	s := token.TokensToString(p.Split(), " ")
	if s != expected {
		t.Fatalf("lex error [%s] != [%s]", s, expected)
	}
}
