package lexer

import (
	"testing"
)

func TestLexerSimple(t *testing.T) {
	p := NewLexer("0123", 0)
	s := p.peekStr(2)
	// 1
	if s != "01" {
		t.Fatalf("lexer.peekstr=%s", s)
	}

	// 2
	c := p.next()
	if c != '0' {
		t.Fatal("lexer.next 0")
	}
	c2 := p.next()
	if c2 != '1' {
		t.Fatalf("lexer.next 1")
	}
	// 3
	s = p.peekStr(3)
	if s != "23" {
		t.Fatalf("lexer.peekstr=%s", s)
	}
}
