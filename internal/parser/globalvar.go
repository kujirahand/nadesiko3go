package parser

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
)

// resolveLateGlobals は、関数定義より後で初めて代入されたグローバル変数を
// 関数の中から参照・代入したとき、名前を完全名（`main__G`）へ直す（#476）。
//
// パーサはソースを先頭から順に読むので、関数本体をパースする時点で未登録の
// 名前はプレーン名のまま残る。コンパイラは名前文字列でグローバルスロットを
// 引くため、プレーン名と完全名が別の変数になってしまう。そこで構文解析の
// 後に、全てのグローバル変数が出揃った状態で名前を解決し直す。
func (p *Parser) resolveLateGlobals(root *ast.Node) {
	globals := lateGlobalTable(p)
	if len(globals) == 0 || root == nil {
		return
	}
	w := &globalResolver{globals: globals}
	w.walk(root, nil)
}

// lateGlobalTable は plain名 → 完全名 の表を作る。同じ plain名に異なる完全名が
// 対応するもの（モジュール違いなど）は曖昧なので除外する。
func lateGlobalTable(p *Parser) map[string]string {
	table := map[string]string{}
	ambiguous := map[string]bool{}
	for full, item := range p.FuncList {
		if item == nil || (item.Type != "var" && item.Type != "const") {
			continue
		}
		i := strings.Index(full, "__")
		if i < 0 {
			continue
		}
		plain := full[i+2:]
		if prev, ok := table[plain]; ok && prev != full {
			ambiguous[plain] = true
			continue
		}
		table[plain] = full
	}
	for plain := range ambiguous {
		delete(table, plain)
	}
	return table
}

type globalResolver struct {
	globals map[string]string
}

// walk は n を走査する。scope が nil のときはトップレベルで、何も書き換えない。
func (w *globalResolver) walk(n *ast.Node, scope map[string]bool) {
	if n == nil {
		return
	}
	switch n.Type {
	case ast.DefFunc, ast.DefTest:
		// 名前付き関数は外側のローカルを見ない
		inner := map[string]bool{}
		w.collectLocals(n, inner, true)
		w.walkChildren(n, inner)
		return
	case ast.FuncObj:
		// 無名関数は外側のローカルをクロージャで捕捉する
		inner := map[string]bool{}
		for k := range scope {
			inner[k] = true
		}
		w.collectLocals(n, inner, true)
		w.walkChildren(n, inner)
		return
	case ast.DefLocalVar, ast.DefLocalVarList:
		// 宣言名は書き換えず、初期値だけ走査する
		w.walkChildren(n, scope)
		return
	}
	if scope != nil {
		w.rewrite(n, scope)
	}
	w.walkChildren(n, scope)
}

func (w *globalResolver) walkChildren(n *ast.Node, scope map[string]bool) {
	for _, b := range n.Blocks {
		w.walk(b, scope)
	}
	for _, b := range n.Index {
		w.walk(b, scope)
	}
}

// collectLocals は関数 fn のローカル名（引数・明示宣言・ループ変数）を集める。
// ネストした関数定義の中には入らない。
func (w *globalResolver) collectLocals(fn *ast.Node, set map[string]bool, top bool) {
	if top {
		for _, a := range fn.Args {
			if s := a.StringValue(); s != "" {
				set[s] = true
			}
		}
		for _, b := range fn.Blocks {
			w.collectLocals(b, set, false)
		}
		return
	}
	if fn == nil {
		return
	}
	switch fn.Type {
	case ast.DefFunc, ast.DefTest, ast.FuncObj:
		return
	case ast.DefLocalVar:
		if fn.Name != "" {
			set[fn.Name] = true
		}
	case ast.DefLocalVarList:
		for _, nm := range fn.Names {
			if s := nm.StringValue(); s != "" {
				set[s] = true
			}
		}
	case ast.For, ast.Foreach:
		if fn.Word != "" {
			set[fn.Word] = true
		}
	}
	for _, b := range fn.Blocks {
		w.collectLocals(b, set, false)
	}
	for _, b := range fn.Index {
		w.collectLocals(b, set, false)
	}
}

// full は plain名を完全名へ引く。書き換え不要なら ok=false。
func (w *globalResolver) full(name string, scope map[string]bool) (string, bool) {
	if name == "" || strings.Contains(name, "__") || scope[name] {
		return "", false
	}
	f, ok := w.globals[name]
	return f, ok
}

func (w *globalResolver) rewrite(n *ast.Node, scope map[string]bool) {
	switch n.Type {
	case ast.Word, ast.Variable:
		s, _ := n.Value.(string)
		if f, ok := w.full(s, scope); ok {
			n.Value = f
			if n.NameToken != nil && n.NameToken.StringValue() == s {
				n.NameToken.Value = f
			}
		}
	case ast.Let, ast.LetArray, ast.LetProp, ast.Inc:
		if f, ok := w.full(n.Name, scope); ok {
			n.Name = f
		}
	case ast.RefArray, ast.RefProp:
		if n.Name != "" {
			if f, ok := w.full(n.Name, scope); ok {
				n.Name = f
			}
		} else if n.NameToken != nil {
			if f, ok := w.full(n.NameToken.StringValue(), scope); ok {
				n.NameToken.Value = f
			}
		}
	}
}
