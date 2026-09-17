package format_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/format"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func formatSource(t *testing.T, code string) string {
	t.Helper()
	tree, err := parser.ParseSource(code, "main.nako3", stdlib.ParserFuncList())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return format.Source(code, "main.nako3", tree)
}

func TestSourceIndentsIfElse(t *testing.T) {
	code := "結果=[]\n" +
		"Iを1から30まで繰り返す\n" +
		"もし、(I % 15 = 0)ならば\n" +
		"結果に「FizzBuzz」を配列追加。\n" +
		"違えば、もし、(I % 3 = 0)ならば\n" +
		"結果に「Fizz」を配列追加。\n" +
		"違えば\n" +
		"結果にIを配列追加。\n" +
		"ここまで\n" +
		"ここまで\n"
	want := "結果=[]\n" +
		"Iを1から30まで繰り返す\n" +
		"    もし、(I % 15 = 0)ならば\n" +
		"        結果に「FizzBuzz」を配列追加。\n" +
		"    違えば、もし、(I % 3 = 0)ならば\n" +
		"        結果に「Fizz」を配列追加。\n" +
		"    違えば\n" +
		"        結果にIを配列追加。\n" +
		"    ここまで\n" +
		"ここまで\n"
	got := formatSource(t, code)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSourceIsIdempotent(t *testing.T) {
	code := "結果=[]\n" +
		"Iを1から30まで繰り返す\n" +
		"もし、(I % 15 = 0)ならば\n" +
		"結果に「FizzBuzz」を配列追加。\n" +
		"ここまで\n" +
		"ここまで\n"
	once := formatSource(t, code)
	twice := formatSource(t, once)
	if once != twice {
		t.Fatalf("not idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestSourceTrimsTrailingWhitespaceAndBlankLines(t *testing.T) {
	code := "A=1  \n\t\nもし、A=1ならば\n表示。   \nここまで\n"
	got := formatSource(t, code)
	want := "A=1\n\nもし、A=1ならば\n    表示。\nここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSourceLeavesMultilineStringAlone(t *testing.T) {
	code := "もし、1=1ならば\n" +
		"A=『あ\nい\nう』\n" +
		"ここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n" +
		"A=『あ\nい\nう』\n" +
		"ここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSourceKeepsMultilineArrayAsContinuation(t *testing.T) {
	code := "データ = [\n" +
		"1,\n" +
		"2\n" +
		"]\n" +
		"表示。\n"
	got := formatSource(t, code)
	want := "データ = [\n" +
		"    1,\n" +
		"    2\n" +
		"]\n" +
		"表示。\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}
