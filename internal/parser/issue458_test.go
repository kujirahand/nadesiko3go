package parser_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Issue #458: 無引数関数が配列・辞書の値として使えることのパーサー側の固定。
// parse ヘルパーはシステム関数を登録しないので、ここでは stdlib と同じ形
// (助詞なしの無引数関数)で「今」「今日」を登録した専用ヘルパーを使う。
func parseWithZeroArgFuncs(t *testing.T, code string) (*ast.Node, error) {
	t.Helper()
	lx := lexer.NewLexer()
	for _, name := range []string{"今", "今日"} {
		lx.FuncList[name] = &lexer.FuncItem{Name: name, Type: "func"}
	}
	lx.FuncList["表示"] = &lexer.FuncItem{Name: "表示", Type: "func", Josi: [][]string{{"を", "と"}}, ReturnNone: true}
	// 1引数関数 (「それ」補完の対象)
	lx.FuncList["倍"] = &lexer.FuncItem{Name: "倍", Type: "func", Josi: [][]string{{"の"}}}
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

// 代入文の右辺を取り出す。tree は Block で、代入は Blocks[0]、
// その値が Blocks[0].Block(0) に入る(既存テストと同じ形)。
func issue458AssignValue(t *testing.T, code string) *ast.Node {
	t.Helper()
	tree, err := parseWithZeroArgFuncs(t, code)
	if err != nil {
		t.Fatalf("%q: %v", code, err)
	}
	if len(tree.Blocks) == 0 {
		t.Fatalf("%q: 空のツリー", code)
	}
	return tree.Blocks[0].Block(0)
}

func TestIssue458ParseArrayFuncValues(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		wantFuncs []string // 要素が関数である位置の名前(それ以外は "")
	}{
		{"先頭", "A=[今,0]", []string{"今", ""}},
		{"末尾", "A=[0,今]", []string{"", "今"}},
		{"真ん中", "A=[0,今,1]", []string{"", "今", ""}},
		{"単独", "A=[今]", []string{"今"}},
		{"連続", "A=[今,今日]", []string{"今", "今日"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := issue458AssignValue(t, tt.code)
			if arr.Type != ast.JSONArray || len(arr.Blocks) != len(tt.wantFuncs) {
				t.Fatalf("%q: array = %#v", tt.code, arr)
			}
			for i, want := range tt.wantFuncs {
				elem := arr.Blocks[i]
				if want == "" {
					if elem.Type == ast.Func {
						t.Errorf("%q: 要素%d は関数でないはずが %q", tt.code, i, elem.Name)
					}
					continue
				}
				if elem.Type != ast.Func || elem.Name != want {
					t.Errorf("%q: 要素%d = (%q, %q), want 関数 %q", tt.code, i, elem.Type, elem.Name, want)
				}
				if len(elem.Blocks) != 0 {
					t.Errorf("%q: 要素%d の無引数関数に引数が付いている: %#v", tt.code, i, elem.Blocks)
				}
			}
		})
	}
}

func TestIssue458ParseDictFuncValues(t *testing.T) {
	// 辞書は key,value の交互の並びになる
	tests := []struct {
		name     string
		code     string
		wantFunc map[int]string // Blocks の添字 -> 関数名
	}{
		{"単独の値", "A={\"a\":今}", map[int]string{1: "今"}},
		{"先頭の値", "A={\"a\":今,\"b\":1}", map[int]string{1: "今"}},
		{"末尾の値", "A={\"a\":1,\"b\":今}", map[int]string{3: "今"}},
		{"全部が関数", "A={\"a\":今,\"b\":今日}", map[int]string{1: "今", 3: "今日"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := issue458AssignValue(t, tt.code)
			if dict.Type != ast.JSONObj {
				t.Fatalf("%q: dict = %#v", tt.code, dict)
			}
			for idx, want := range tt.wantFunc {
				elem := dict.Blocks[idx]
				if elem.Type != ast.Func || elem.Name != want {
					t.Errorf("%q: Blocks[%d] = (%q, %q), want 関数 %q", tt.code, idx, elem.Type, elem.Name, want)
				}
			}
		})
	}
}

func TestIssue458ParseNestedFuncValues(t *testing.T) {
	arr := issue458AssignValue(t, "A=[[今,0],{\"a\":今日}]")
	if arr.Type != ast.JSONArray || len(arr.Blocks) != 2 {
		t.Fatalf("array = %#v", arr)
	}
	inner := arr.Blocks[0]
	if inner.Type != ast.JSONArray || inner.Blocks[0].Type != ast.Func || inner.Blocks[0].Name != "今" {
		t.Fatalf("inner array = %#v", inner)
	}
	dict := arr.Blocks[1]
	if dict.Type != ast.JSONObj || dict.Blocks[1].Type != ast.Func || dict.Blocks[1].Name != "今日" {
		t.Fatalf("inner dict = %#v", dict)
	}
}

// 1引数関数がカンマや } の直前に来た場合も一語関数分岐に入り、
// 既存処理どおり暗黙引数「それ」が補完されること。
// agy レビュー (低指摘1) を受けて追加した (実行時の「それ」の値は
// 文脈依存のため、ここでは AST の形状だけを固定する)。
func TestIssue458ParseOneArgFuncGetsSore(t *testing.T) {
	// 配列: A=[倍,0] の要素0
	arr := issue458AssignValue(t, "A=[倍,0]")
	elem := arr.Blocks[0]
	if elem.Type != ast.Func || elem.Name != "倍" || len(elem.Blocks) != 1 || elem.Blocks[0].Value != "それ" {
		t.Fatalf("A=[倍,0] の要素 = %#v, want 「それ」を引数に持つ関数 倍", elem)
	}
	// 辞書: A={"a":倍} の値 (Blocks[1])
	dict := issue458AssignValue(t, "A={\"a\":倍}")
	val := dict.Blocks[1]
	if val.Type != ast.Func || val.Name != "倍" || len(val.Blocks) != 1 || val.Blocks[0].Value != "それ" {
		t.Fatalf("辞書の値 = %#v, want 「それ」を引数に持つ関数 倍", val)
	}
}

func TestIssue458ParseStillRejectsUnclosed(t *testing.T) {
	for _, code := range []string{"A=[今,0", "A={\"a\":今"} {
		if _, err := parseWithZeroArgFuncs(t, code); err == nil {
			t.Errorf("%q は文法エラーになるべき", code)
		}
	}
}
