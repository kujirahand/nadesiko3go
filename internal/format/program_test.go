package format_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/format"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func parseFunc(code, filename string) (*ast.Node, error) {
	return parser.ParseSource(code, filename, stdlib.ParserFuncList())
}

func formatProgram(t *testing.T, code string, opts format.Options) string {
	t.Helper()
	got, err := format.Program(code, "main.nako3", parseFunc, opts)
	if err != nil {
		t.Fatalf("Program: %v", err)
	}
	// 整形結果をもう一度整形しても変わらない(冪等)。
	again, err := format.Program(got, "main.nako3", parseFunc, opts)
	if err != nil {
		t.Fatalf("Program(2回目): %v", err)
	}
	if again != got {
		t.Fatalf("2回目の整形で結果が変わりました:\n1回目:\n%s\n2回目:\n%s", got, again)
	}
	return got
}

func TestProgramNormalizesSpacesAndSymbols(t *testing.T) {
	tests := []struct{ code, want string }{
		// #120 の例
		{"1   + 2 * 3 を表示\n", "1 + 2 × 3を表示\n"},
		{"（1   + ２） * 3 を表示\n", "(1 + 2) × 3を表示\n"},
		{"10 / 4 を表示\n", "10 ÷ 4を表示\n"},
		// 半角の演算子の前後には空白を置く
		{"値＝50\n値を表示\n", "値 = 50\n値を表示\n"},
		{"(1+2)* 3 を表示\n", "(1 + 2) × 3を表示\n"},
		// 比較演算子は『≧』『≦』『≠』に揃え、前後に空白を置く
		{"A=1\nもしA>=1ならば「はい」を表示\n", "A = 1\nもしA ≧ 1ならば「はい」を表示\n"},
		{"A=1\nもしA=>1ならば「はい」を表示\n", "A = 1\nもしA ≧ 1ならば「はい」を表示\n"},
		{"A=1\nもしA<=1ならば「はい」を表示\n", "A = 1\nもしA ≦ 1ならば「はい」を表示\n"},
		{"A=1\nもしA!=2ならば「はい」を表示\n", "A = 1\nもしA ≠ 2ならば「はい」を表示\n"},
		{"A=1\nもしA<>2ならば「はい」を表示\n", "A = 1\nもしA ≠ 2ならば「はい」を表示\n"},
		{"A=1\nもしA≧1ならば「はい」を表示\n", "A = 1\nもしA ≧ 1ならば「はい」を表示\n"},
		{"A=1\nもしA==1ならば「はい」を表示\n", "A = 1\nもしA == 1ならば「はい」を表示\n"},
		// 全角記号に挟まれた演算子の前後には空白を置かない
		{"「あ」 ＆ 「い」を表示\n", "「あ」&「い」を表示\n"},
		{"A=「あ」\nA ＆ 「い」を表示\n", "A = 「あ」\nA & 「い」を表示\n"},
		// 単項のマイナスの後ろには空白を置かない
		{"A=- 1\n-1を表示\n(- 2)を表示\nAに -3を足して表示\n2*-1を表示\n", "A = -1\n-1を表示\n(-2)を表示\nAに -3を足して表示\n2 × -1を表示\n"},
		{"A=5\nA-1を表示\n", "A = 5\nA - 1を表示\n"},
		// 括弧の内側・読点の前後の空白
		{"A = [ 1 ,  2 ]\nA[ 0 ] を表示\n", "A = [1, 2]\nA[0]を表示\n"},
		{"「あ」 と 「い」 を 連結して表示\n", "「あ」と 「い」を 連結して表示\n"},
		// 展開あり文字列・文字列の中身は変えない
		{"B = 1\n「値は{B}  *  2です」  を表示\n", "B = 1\n「値は{B}  *  2です」を表示\n"},
		{"「1   *  ２」を表示\n", "「1   *  ２」を表示\n"},
		// 行末コメントの前の空白とコメントの中身は変えない
		{"1 * 2を表示   # 1  *  2\n", "1 × 2を表示   # 1  *  2\n"},
	}
	for _, tt := range tests {
		if got := formatProgram(t, tt.code, format.Options{}); got != tt.want {
			t.Errorf("Program(%q)\n got: %q\nwant: %q", tt.code, got, tt.want)
		}
	}
}

// インデント構文・コロン記法のファイルも整形できる(#120)。
func TestProgramFormatsColonSyntax(t *testing.T) {
	code := "3回:\n" +
		"  もし、はいならば：\n" +
		"      「T」と表示\n" +
		"  違えば：\n" +
		"   「F」と表示\n" +
		"「end」を表示\n"
	want := "3回:\n" +
		"    もし、はいならば:\n" +
		"        「T」と表示\n" +
		"    違えば:\n" +
		"        「F」と表示\n" +
		"「end」を表示\n"
	if got := formatProgram(t, code, format.Options{}); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProgramConvertsBlocksToColon(t *testing.T) {
	code := "もし1=1ならば\n" +
		"「x」を表示  # 行末\n" +
		"  もし2>1ならば\n" +
		"「y」を表示\n" +
		"  違えば\n" +
		"  「z」を表示\n" +
		"ここまで\n" +
		"違えば\n" +
		"「w」を表示\n" +
		"ここまで\n" +
		"3回\n" +
		"「繰」を表示\n" +
		"ここまで\n" +
		"●（Aを）倍とは\n" +
		"それは A×2\n" +
		"ここまで\n" +
		"エラー監視\n" +
		"「try」を表示\n" +
		"エラーならば\n" +
		"「err」を表示\n" +
		"ここまで\n"
	want := "もし1 = 1ならば:\n" +
		"    「x」を表示  # 行末\n" +
		"    もし2 > 1ならば:\n" +
		"        「y」を表示\n" +
		"    違えば:\n" +
		"        「z」を表示\n" +
		"違えば:\n" +
		"    「w」を表示\n" +
		"3回:\n" +
		"    「繰」を表示\n" +
		"●(Aを)倍とは:\n" +
		"    それは A × 2\n" +
		"エラー監視:\n" +
		"    「try」を表示\n" +
		"エラーならば:\n" +
		"    「err」を表示\n"
	if got := formatProgram(t, code, format.Options{Colon: true}); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// 書き換えられないブロック(本文が空、『ここまで』の行にコメントがある、
// 条件分岐)はそのまま残す。
func TestProgramKeepsBlocksThatCannotBeColon(t *testing.T) {
	code := "もし1=1ならば\n" +
		"ここまで\n" +
		"3回\n" +
		"    「A」を表示\n" +
		"ここまで # 終わり\n" +
		"「a」で条件分岐\n" +
		"    「a」ならば\n" +
		"        「A」を表示\n" +
		"    ここまで\n" +
		"    違えば\n" +
		"        「else」を表示\n" +
		"    ここまで\n" +
		"ここまで\n"
	want := strings.Replace(code, "もし1=1", "もし1 = 1", 1)
	if got := formatProgram(t, code, format.Options{Colon: true}); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// 『!インデント構文』のファイルはコロン記法に書き換えない。
func TestProgramDoesNotConvertIndentSyntax(t *testing.T) {
	code := "!インデント構文\n3回\n    「A」と表示\n「終わり」と表示\n"
	if got := formatProgram(t, code, format.Options{Colon: true}); got != code {
		t.Fatalf("got:\n%s\nwant:\n%s", got, code)
	}
}

func TestProgramIndentsSwitch(t *testing.T) {
	code := "「a」で条件分岐\n" +
		"「a」ならば\n" +
		"「A」を表示\n" +
		"ここまで\n" +
		"違えば\n" +
		"「else」を表示\n" +
		"ここまで\n" +
		"ここまで\n" +
		"「終」を表示\n"
	want := "「a」で条件分岐\n" +
		"    「a」ならば\n" +
		"        「A」を表示\n" +
		"    ここまで\n" +
		"    違えば\n" +
		"        「else」を表示\n" +
		"    ここまで\n" +
		"ここまで\n" +
		"「終」を表示\n"
	if got := formatProgram(t, code, format.Options{}); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// コメントだけの行は、前後のコードの行のうち深い方に揃える。
func TestProgramIndentsCommentLines(t *testing.T) {
	code := "# 先頭\n" +
		"もし1=1ならば\n" +
		"「A」を表示\n" +
		"# 本文の末尾\n" +
		"ここまで\n" +
		"# ブロックの間\n" +
		"3回\n" +
		"# 本文の先頭\n" +
		"「B」を表示\n" +
		"ここまで\n"
	want := "# 先頭\n" +
		"もし1 = 1ならば\n" +
		"    「A」を表示\n" +
		"    # 本文の末尾\n" +
		"ここまで\n" +
		"# ブロックの間\n" +
		"3回\n" +
		"    # 本文の先頭\n" +
		"    「B」を表示\n" +
		"ここまで\n"
	if got := formatProgram(t, code, format.Options{}); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProgramReturnsSyntaxError(t *testing.T) {
	_, err := format.Program("もし「A」ならば\n「B」と表示。", "main.nako3", parseFunc, format.Options{})
	if err == nil {
		t.Fatal("文法エラーなのにエラーになりませんでした")
	}
	if errors.Is(err, format.ErrStructureChanged) {
		t.Fatalf("文法エラーが構造変化として報告されました: %v", err)
	}
}

// 整形結果の構文構造が元と変わる場合は、書き換えずにErrStructureChangedを
// 返す。構文解析関数を差し替えて、2回目以降の解析結果だけを変えて確かめる。
func TestProgramRefusesStructureChange(t *testing.T) {
	calls := 0
	parse := func(code, filename string) (*ast.Node, error) {
		calls++
		if calls == 1 {
			return parseFunc(code, filename)
		}
		return parseFunc("「別の」と表示\n", filename)
	}
	_, err := format.Program("3回\n「A」と表示\nここまで\n", "main.nako3", parse, format.Options{})
	if !errors.Is(err, format.ErrStructureChanged) {
		t.Fatalf("err = %v, want ErrStructureChanged", err)
	}
}
