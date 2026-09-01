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
