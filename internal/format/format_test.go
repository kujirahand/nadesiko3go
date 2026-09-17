package format_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/format"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func parse(t *testing.T, code string) *ast.Node {
	t.Helper()
	tree, err := parser.ParseSource(code, "main.nako3", stdlib.ParserFuncList())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return tree
}

func formatSource(t *testing.T, code string) string {
	t.Helper()
	return format.Source(code, "main.nako3", parse(t, code))
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

// リテラルの中身(開始行の引用符から終了行の引用符まで)は書き換えないが、
// 開始行の引用符より前にあるコードは、通常の行と同じくインデントを付け直す。
func TestSourceLeavesMultilineStringAlone(t *testing.T) {
	code := "もし、1=1ならば\n" +
		"A=『あ\nい\nう』\n" +
		"ここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n" +
		"    A=『あ\nい\nう』\n" +
		"ここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// リテラルの前後にコードが同居する場合でも、内部の改行や中身は変えない。
// 開始行はリテラルの前だけ整形し、終了行はリテラルの後ろの末尾空白だけ落とす。
func TestSourceReindentsCodeAroundMultilineString(t *testing.T) {
	code := "もし、1=1ならば\n" +
		"A=『あ\nい』と表示。   \n" +
		"ここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n" +
		"    A=『あ\nい』と表示。\n" +
		"ここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// インデント構文では、デデント先の行にパーサーが『ここまで』を合成する。
// その行は外側の文のものなので、合成された終端に引きずられて深くしてはいけない
// (以前はここで最後の行がループの中に取り込まれていた)。
func TestSourceKeepsDedentedLineOutsideBlock(t *testing.T) {
	code := "!インデント構文\n" +
		"3回\n" +
		"    「A」と表示。\n" +
		"    もし、1=1ならば\n" +
		"        「B」と表示。\n" +
		"「終わり」と表示。\n"
	got := formatSource(t, code)
	if got != code {
		t.Fatalf("got:\n%s\nwant (unchanged):\n%s", got, code)
	}
}

// 行末の『:』でブロックを開くファイルは、合成された『ここまで』が本文の
// 最終行の行番号を持つため、深さを読み取れない。Structureの比較で気づける
// ことを確かめる(呼び出し側はこの結果を捨てる)。
func TestStructureCatchesColonSyntaxReindent(t *testing.T) {
	code := "3回:\n    「A」と表示。\n「終」と表示。\n"
	before := parse(t, code)
	formatted := format.Source(code, "main.nako3", before)
	after := parse(t, formatted)
	if format.Structure(before) == format.Structure(after) {
		t.Fatalf("整形で構造が変わったのに検出できていません:\n%s", formatted)
	}
}

func TestStructureIgnoresIndentationOnly(t *testing.T) {
	code := "もし、1=1ならば\n「やあ」と表示。\nここまで\n"
	before := parse(t, code)
	after := parse(t, format.Source(code, "main.nako3", before))
	if format.Structure(before) != format.Structure(after) {
		t.Fatalf("インデントを変えただけで構造が変わったと判定されました")
	}
}

func TestSourceIndentsCommentLinesWithTheirBlock(t *testing.T) {
	code := "もし、1=1ならば\n// 中のコメント\n「やあ」と表示。\nここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n    // 中のコメント\n    「やあ」と表示。\nここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// 末尾に複数の空行があっても、すべて落として改行を1つだけ付ける。
func TestSourceCollapsesTrailingBlankLines(t *testing.T) {
	code := "A=1\n\n\n\n"
	got := formatSource(t, code)
	want := "A=1\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// 末尾に改行がないファイルも整形でき(EOLがStructureの比較から除かれる)、
// 末尾には改行が1つ付く。
func TestSourceAddsMissingTrailingNewline(t *testing.T) {
	code := "A=1"
	got := formatSource(t, code)
	want := "A=1\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	tree := parse(t, code)
	after := parse(t, got)
	if format.Structure(tree) != format.Structure(after) {
		t.Fatalf("末尾に改行を1つ付けただけで構造が変わったと判定されました")
	}
}

// ファイル末尾にある複数行配列・辞書リテラルの閉じ記号(]・})は、それを
// 含む文と同じ深さになる(本文の続きとして扱われて深くなってはいけない)。
func TestSourceIndentsClosingBracketAtEOF(t *testing.T) {
	code := "もし、1=1ならば\nデータ=[\n1\n]\nここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n    データ=[\n        1\n    ]\nここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// lexerがインデント文字として数える文字(全角中黒など)も、半角スペース・
// タブ・全角スペースと同じく行頭から取り除く。
func TestSourceStripsLexerIndentCharacters(t *testing.T) {
	code := "もし、1=1ならば\n・・「A」と表示。\nここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n    「A」と表示。\nここまで\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// 単独のCRだけを改行として使うファイルも、CRLFやLFと同じく行として認識し、
// 文をまたいで1行に連結してしまわない。
func TestSourceHandlesLoneCRNewlines(t *testing.T) {
	code := "A=1\rB=2\r「{A}{B}」と表示。\r"
	got := formatSource(t, code)
	want := "A=1\nB=2\n「{A}{B}」と表示。\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// CRLFとLFが混在するファイルで、複数行リテラルの内部にある改行は、外側の
// 改行コードに合わせて書き換えられず、元の綴りのまま残る。
func TestSourceKeepsLiteralInteriorNewlineStyle(t *testing.T) {
	code := "もし、1=1ならば\r\nA=『一行目\n二行目』\r\nここまで\r\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\r\n    A=『一行目\n二行目』\r\nここまで\r\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// 1つの行に、手前の範囲コメントの閉じ記号と、次の文字列リテラルの開き
// 記号が両方あるとき(範囲コメントの直後に複数行文字列が始まる、非常に
// 稀なケース)、コメントの中身の一部にインデントを書き込んでしまわない。
func TestSourceHandlesAdjacentProtectedSpansOnSameLine(t *testing.T) {
	code := "もし、1=1ならば\n" +
		"/*コメント\n続き*/A=『あ\nい』\n" +
		"Aを表示。\n" +
		"ここまで\n"
	got := formatSource(t, code)
	want := "もし、1=1ならば\n" +
		"/*コメント\n続き*/A=『あ\nい』\n" +
		"    Aを表示。\n" +
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
