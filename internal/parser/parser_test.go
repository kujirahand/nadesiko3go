package parser_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

func parse(t *testing.T, code string) (*ast.Node, error) {
	t.Helper()
	lx := lexer.NewLexer()
	lx.FuncList["表示"] = &lexer.FuncItem{Name: "表示", Type: "func", Josi: [][]string{{"を", "と"}}}
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, "main.nako3")
	if err != nil {
		return nil, err
	}
	tokens, err := lx.ReplaceTokens(raw, true, "main.nako3")
	if err != nil {
		return nil, err
	}
	p := parser.New()
	p.SetFuncList(lx.FuncList)
	p.SetModuleExport(lx.ModuleExport)
	return p.Parse(tokens, "main.nako3")
}

func TestParseLiteralAssignmentAndCall(t *testing.T) {
	tree, err := parse(t, "A=30\nAを表示")
	if err != nil {
		t.Fatal(err)
	}
	if tree.Type != ast.Block || len(tree.Blocks) < 3 {
		t.Fatalf("tree = %#v", tree)
	}
	let := tree.Blocks[0]
	if let.Type != ast.Let || let.Name != "main__A" || let.Block(0).Value != float64(30) {
		t.Fatalf("assignment = %#v", let)
	}
	call := tree.Blocks[2]
	if call.Type != ast.Func || call.Name != "表示" || call.Block(0).StringValue() != "main__A" {
		t.Fatalf("call = %#v", call)
	}
}

func TestParseOperatorPrecedence(t *testing.T) {
	tree, err := parse(t, "(1+2*3)を表示")
	if err != nil {
		t.Fatal(err)
	}
	call := tree.Blocks[0]
	expr := call.Block(0)
	if expr.Type != ast.Op || expr.Operator != "+" || expr.Block(1).Operator != "*" {
		t.Fatalf("expression = %#v", expr)
	}
}

func TestParseArrayAndDictLiteral(t *testing.T) {
	tree, err := parse(t, "A=[1,2,3]\nB={\"x\":1,\"y\":2}")
	if err != nil {
		t.Fatal(err)
	}
	array := tree.Blocks[0].Block(0)
	if array.Type != ast.JSONArray || len(array.Blocks) != 3 {
		t.Fatalf("array = %#v", array)
	}
	dict := tree.Blocks[2].Block(0)
	if dict.Type != ast.JSONObj || len(dict.Blocks) != 4 {
		t.Fatalf("dict = %#v", dict)
	}
}

func TestParseArrayReferenceAndAssignment(t *testing.T) {
	tree, err := parse(t, "A=[1,2]\nA[0]=9\nA[0]を表示")
	if err != nil {
		t.Fatal(err)
	}
	set := tree.Blocks[2]
	if set.Type != ast.LetArray || set.Name != "main__A" || len(set.Index) != 1 {
		t.Fatalf("array assignment = %#v", set)
	}
	ref := tree.Blocks[4].Block(0)
	if ref.Type != ast.RefArray || ref.Name != "main__A" || len(ref.Index) != 1 {
		t.Fatalf("array reference = %#v", ref)
	}
}

func TestParseReportsMissingCloseParen(t *testing.T) {
	_, err := parse(t, "A=(1+2\nAを表示")
	if err == nil {
		t.Fatal("expected syntax error")
	}
	nakoErr, ok := err.(*errs.NakoError)
	if !ok || nakoErr.Kind != errs.Syntax {
		t.Fatalf("error = %#v", err)
	}
}

func TestParseControlAndFunctionSyntax(t *testing.T) {
	cases := map[string]string{
		"if":        "もし1=1ならば\n『T』と表示\n違えば\n『F』と表示\nここまで",
		"repeat":    "3回\n『x』と表示\nここまで",
		"while":     "A=0\nA<3の間\nA=A+1\nここまで",
		"foreach":   "[10,20]を反復\n対象を表示\nここまで",
		"function":  "●(AとBを)足すとは\nA+Bで戻る\nここまで",
		"anonymous": "F=関数(A)\nA*2で戻る\nここまで",
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(t, code); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The expected messages come from NakoCompiler.parse in nako3.mts. The compat
// fixture 09_error pins the same four cases.
func TestParseErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		code string
		kind errs.Kind
		want string
	}{
		{
			// 代入の右辺で起きたエラーは、どの変数への代入かを添えて
			// 元の文面ごと report し直すので2行になる
			name: "閉じ括弧なし",
			code: "A=(1+2\nAを表示",
			kind: errs.Syntax,
			want: "[文法エラー]main.nako3(1行目): 単語『A』への代入文で計算式に以下の書き間違いがあります。\n" +
				"[文法エラー]main.nako3(1行目): (...)の解析エラー。『数値1と数値2に演算子『+』を適用した式』の近く",
		},
		{
			// メインモジュールの接頭辞 main__ は文面から省かれる #1223
			name: "未解決の単語",
			code: "未定義関数呼ぶ",
			kind: errs.Syntax,
			want: "[文法エラー]main.nako3(1行目): 不完全な文です。単語『未定義関数呼』が解決していません。",
		},
		{
			name: "ここまでの不足",
			code: "もし1=1ならば\n「T」と表示",
			kind: errs.Syntax,
			want: "[文法エラー]main.nako3(1行目): 『もし』文で『ここまで』がありません。",
		},
		{
			name: "文字列の入れ子",
			code: "「あ「い」を表示",
			kind: errs.Lexer,
			want: "[字句解析エラー]main.nako3(1行目): 『「』で始めた文字列の中に『「』を含めることはできません。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(t, tt.code)
			if err == nil {
				t.Fatal("エラーになるはずが成功した")
			}
			nakoErr, ok := err.(*errs.NakoError)
			if !ok {
				t.Fatalf("エラー型 = %T, want *errs.NakoError", err)
			}
			if nakoErr.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", nakoErr.Kind, tt.kind)
			}
			if got := nakoErr.Error(); got != tt.want {
				t.Errorf("Error()\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}
