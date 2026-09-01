package indent_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

func TestConvertSyntaxAddsBlockEnd(t *testing.T) {
	code := "!インデント構文\nもし1=1ならば\n    「T」と表示\n"
	tokens, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err = indent.ConvertSyntax(tokens)
	if err != nil {
		t.Fatal(err)
	}
	ends := 0
	for _, token := range tokens {
		if token.Type == lexer.TypeKokomade {
			ends++
		}
	}
	if ends != 1 {
		t.Fatalf("ここまで = %d, want 1", ends)
	}
}

func TestConvertInlineColonAddsBlockEnds(t *testing.T) {
	code := "もし、はいならば：\n  「T」と表示\n違えば：\n  「F」と表示\n"
	tokens, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err = indent.ConvertSyntax(tokens)
	if err != nil {
		t.Fatal(err)
	}
	ends, colons := 0, 0
	for _, token := range tokens {
		if token.Type == lexer.TypeKokomade {
			ends++
		}
		if token.Type == ":" {
			colons++
		}
	}
	if ends != 1 || colons != 0 {
		t.Fatalf("ここまで=%d, colon=%d; want 1, 0", ends, colons)
	}
}
