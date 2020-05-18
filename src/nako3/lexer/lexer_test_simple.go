package lexer

import (
	"testing"
)

// TestLexerSimple : Test
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

// TestLexerSimple2 : Test
func TestLexerSimple2(t *testing.T) {
	s1 := DeleteOkurigana("足す")
	if s1 != "足" {
		t.Errorf("DeleteOkurigana : 足 != %s", s1)
	}
	s2 := DeleteOkurigana("ありす")
	if s2 != "ありす" {
		t.Errorf("DeleteOkurigana : ありす != %s", s1)
	}
	s3 := DeleteOkurigana("足")
	if s3 != "足" {
		t.Errorf("DeleteOkurigana : 足 != %s", s1)
	}
	s4 := DeleteOkurigana("BLUEしろ")
	if s4 != "BLUE" {
		t.Errorf("DeleteOkurigana : BLUE != %s", s1)
	}
}
