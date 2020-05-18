package lexer

import (
	"nako3/token"
	"testing"
)

func TestLexer1(t *testing.T) {
	execLex(t, "漢字=32", "漢字 = 32")
	execLex(t, "32を表示", "32を 表示")
	execLex(t, "a=1+2*3", "a = 1 + 2 * 3")
	execLex(t, "3を表示\n4を表示", "3を 表示 LF 4を 表示")
}
func TestLexer2(t *testing.T) {
	execLex(t, "a=32", "a = 32")
	execLex(t, "a='abc'", "a = 'abc'")
	execLex(t, "a=「abc」", "a = 「abc」")
	execLex(t, "a=「abc{~}abc」", "a = 「abc\nabc」")
}

func execLex(t *testing.T, code, expected string) {
	p := NewLexer(code, 0)
	s := token.TokensToString(p.Split(), " ")
	if s != expected {
		t.Fatalf("lex error [%s] != [%s]", expected, s)
	}
}
