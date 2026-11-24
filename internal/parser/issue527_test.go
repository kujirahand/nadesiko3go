package parser_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Issue #527: 『もし』の条件に「〜の関数」形式の助詞付き関数呼び出しと
// 比較演算子や助詞『が』を組み合わせた式を直接書けることのテスト。
func parseIssue527(t *testing.T, code string) (*ast.Node, error) {
	t.Helper()
	lx := lexer.NewLexer()
	lx.FuncList["表示"] = &lexer.FuncItem{Name: "表示", Type: "func", Josi: [][]string{{"を", "と"}}, ReturnNone: true}
	lx.FuncList["要素数"] = &lexer.FuncItem{Name: "要素数", Type: "func", Josi: [][]string{{"の"}}}
	lx.FuncList["足"] = &lexer.FuncItem{Name: "足", Type: "func", Josi: [][]string{{"と", "に"}, {"を"}}}
	lx.FuncList["以上"] = &lexer.FuncItem{Name: "以上", Type: "func", Josi: [][]string{{"が"}, {""}}}
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

func TestIssue527IfCondWithParticleFuncCall(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantType ast.NodeType
		wantOp   string
	}{
		{
			name:     "基本形_等価比較",
			code:     "もし、Aの要素数 = 2 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "不等価比較",
			code:     "もし、Aの要素数 != 3 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "noteq",
		},
		{
			name:     "算術演算子と比較演算子",
			code:     "もし、Aの要素数 + 1 = 3 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "否定形_でなければ",
			code:     "もし、Aの要素数 = 2 でなければ\n「OK」を表示\nここまで",
			wantType: ast.Not,
			wantOp:   "not",
		},
		{
			name:     "論理演算子との組み合わせ",
			code:     "もし、Aの要素数 = 2 かつ B = 1 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "and",
		},
		{
			name:     "複数引数の関数呼び出し",
			code:     "もし、AとBを足す = 3 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "助詞がによる等価比較",
			code:     "もし、Aの要素数が2ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "助詞がによる否定比較",
			code:     "もし、Aの要素数が2でなければ\n「OK」を表示\nここまで",
			wantType: ast.Not,
			wantOp:   "not",
		},
		{
			name:     "算術演算子と助詞が",
			code:     "もし、Aの要素数 + 1 が 3 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "既存の関数連鎖_以上",
			code:     "もし、Aの要素数が1以上ならば\n「OK」を表示\nここまで",
			wantType: ast.Func,
			wantOp:   "",
		},
		{
			name:     "もし省略形",
			code:     "Aの要素数 = 2 ならば\n「OK」を表示\nここまで",
			wantType: ast.Op,
			wantOp:   "eq",
		},
		{
			name:     "一行もし文",
			code:     "もし、Aの要素数 = 2 ならば「OK」を表示",
			wantType: ast.Op,
			wantOp:   "eq",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := parseIssue527(t, tt.code)
			if err != nil {
				t.Fatalf("parse(%q) error = %v", tt.code, err)
			}
			if len(tree.Blocks) == 0 {
				t.Fatalf("parse(%q): 空のツリー", tt.code)
			}
			ifNode := tree.Blocks[0]
			if ifNode.Type != ast.If {
				t.Fatalf("parse(%q): node.Type = %q, want %q", tt.code, ifNode.Type, ast.If)
			}
			cond := ifNode.Block(0)
			if cond == nil {
				t.Fatalf("parse(%q): 条件ノードが nil", tt.code)
			}
			if cond.Type != tt.wantType || cond.Operator != tt.wantOp {
				t.Errorf("parse(%q): cond = (Type=%q, Operator=%q), want (Type=%q, Operator=%q)",
					tt.code, cond.Type, cond.Operator, tt.wantType, tt.wantOp)
			}
		})
	}
}

func TestIssue527IfExprAndNestedFunc(t *testing.T) {
	codes := []string{
		"X = もし、Aの要素数 = 2 ならば 10 違えば 20",
		"X = もし、Aの要素数が2ならば 10 違えば 20",
		"F = 関数()\nもし、Aの要素数 = 2 ならば\n「OK」を表示\nここまで\nここまで",
	}
	for _, code := range codes {
		if _, err := parseIssue527(t, code); err != nil {
			t.Errorf("parse(%q) error = %v", code, err)
		}
	}
}

func TestIssue527InvalidSyntaxStillErrors(t *testing.T) {
	codes := []string{
		"もし、Aの要素数 = 2をならば\n「OK」を表示\nここまで",
		"もし、Aの要素数 2 ならば\n「OK」を表示\nここまで",
	}
	for _, code := range codes {
		if _, err := parseIssue527(t, code); err == nil {
			t.Errorf("parse(%q): 文法エラーになるべきだが成功した", code)
		}
	}
}
