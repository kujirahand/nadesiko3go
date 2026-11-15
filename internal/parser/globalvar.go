package parser

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// resolveLateGlobals は、関数定義より後で初めて代入されたグローバル変数を
// 関数の中から参照・代入したとき、名前を完全名（`main__G`）へ直す（#476）。
//
// パーサはソースを先頭から順に読むので、関数本体をパースする時点で未登録の
// 名前はプレーン名のまま残る。コンパイラは名前文字列でグローバルスロットを
// 引くため、プレーン名と完全名が別の変数になってしまう。そこで構文解析の
// 後に、全てのグローバル変数が出揃った状態で名前を解決し直す。
//
// 解決の順序と公開判定は FindVar に合わせる: 自分のモジュールの変数を優先し、
// 次に取り込んだモジュールの公開変数を探す。非公開の変数は他のモジュール
// から見えない。
func (p *Parser) resolveLateGlobals(root *ast.Node) {
	byMod := lateGlobalsByMod(p)
	if len(byMod) == 0 || root == nil {
		return
	}
	rootMod := ""
	if len(p.ModList) > 0 {
		rootMod = p.ModList[0]
	}
	w := &globalResolver{p: p, byMod: byMod}
	w.walk(root, nil, rootMod)
}

// lateGlobalsByMod は モジュール名 → (plain名 → 登録情報) の表を作る。
// 完全名の接頭辞は、既知のモジュール名（ModList）のうち最長一致するものを
// 使う。モジュール名に "_" が含まれていても誤って分割しないため。
func lateGlobalsByMod(p *Parser) map[string]map[string]*lexer.FuncItem {
	byMod := map[string]map[string]*lexer.FuncItem{}
	for full, item := range p.FuncList {
		if item == nil || (item.Type != "var" && item.Type != "const") {
			continue
		}
		mod := ""
		for _, m := range p.ModList {
			if strings.HasPrefix(full, m+"__") && len(m) > len(mod) {
				mod = m
			}
		}
		if mod == "" {
			continue
		}
		plain := full[len(mod)+2:]
		mm := byMod[mod]
		if mm == nil {
			mm = map[string]*lexer.FuncItem{}
			byMod[mod] = mm
		}
		mm[plain] = item
	}
	return byMod
}

type globalResolver struct {
	p     *Parser
	byMod map[string]map[string]*lexer.FuncItem
}

// modOf はノードの所属モジュールをソースファイル名から求める。
// 既知のモジュールでなければ空文字列を返す。
func (w *globalResolver) modOf(n *ast.Node) string {
	if n == nil || n.File == "" {
		return ""
	}
	m := lexer.FilenameToModName(n.File)
	for _, mod := range w.p.ModList {
		if mod == m {
			return m
		}
	}
	return ""
}

// walk は n を走査する。scope が nil のときはトップレベルで、何も書き換えない。
// mod は現在の関数が属するモジュール名。
func (w *globalResolver) walk(n *ast.Node, scope map[string]bool, mod string) {
	if n == nil {
		return
	}
	switch n.Type {
	case ast.DefFunc, ast.DefTest:
		// 名前付き関数は外側のローカルを見ない
		if m := w.modOf(n); m != "" {
			mod = m
		}
		inner := map[string]bool{}
		w.collectLocals(n, inner, true)
		w.walkChildren(n, inner, mod)
		return
	case ast.FuncObj:
		// 無名関数は外側のローカルをクロージャで捕捉する
		if m := w.modOf(n); m != "" {
			mod = m
		}
		inner := map[string]bool{}
		for k := range scope {
			inner[k] = true
		}
		w.collectLocals(n, inner, true)
		w.walkChildren(n, inner, mod)
		return
	case ast.DefLocalVar, ast.DefLocalVarList:
		// 宣言名は書き換えず、初期値だけ走査する
		w.walkChildren(n, scope, mod)
		return
	}
	if scope != nil {
		w.rewrite(n, scope, mod)
	}
	w.walkChildren(n, scope, mod)
}

func (w *globalResolver) walkChildren(n *ast.Node, scope map[string]bool, mod string) {
	for _, b := range n.Blocks {
		w.walk(b, scope, mod)
	}
	for _, b := range n.Index {
		w.walk(b, scope, mod)
	}
}

// collectLocals は関数 fn のローカル名（引数・明示宣言・ループ変数）を集める。
// ネストした関数定義の中には入らない。
// なお、関数内の明示宣言は宣言位置によらず関数全体のローカルとして扱う。
// そのため「暗黙代入の後に同じ名前を明示宣言する」書き方では、代入も
// ローカル側に寄る（定義順で意味が変わる問題がこの形に限って残る）。
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

// full は plain名を完全名へ引く。FindVar と同じ順序・公開判定を使う。
// 書き換え不要なら ok=false。
func (w *globalResolver) full(name string, scope map[string]bool, mod string) (string, bool) {
	if name == "" || strings.Contains(name, "__") || scope[name] {
		return "", false
	}
	if mm := w.byMod[mod]; mm != nil {
		if _, ok := mm[name]; ok {
			return mod + "__" + name, true
		}
	}
	for _, m := range w.p.ModList {
		if m == mod {
			continue
		}
		mm := w.byMod[m]
		if mm == nil {
			continue
		}
		item, ok := mm[name]
		if !ok || !w.p.exported(item, m) {
			continue
		}
		return m + "__" + name, true
	}
	return "", false
}

func (w *globalResolver) rewrite(n *ast.Node, scope map[string]bool, mod string) {
	switch n.Type {
	case ast.Word, ast.Variable:
		s, _ := n.Value.(string)
		if f, ok := w.full(s, scope, mod); ok {
			n.Value = f
			if n.NameToken != nil && n.NameToken.StringValue() == s {
				n.NameToken.Value = f
			}
		}
	case ast.Let, ast.LetArray, ast.LetProp, ast.Inc:
		if f, ok := w.full(n.Name, scope, mod); ok {
			n.Name = f
		}
	case ast.RefArray, ast.RefProp:
		if n.Name != "" {
			if f, ok := w.full(n.Name, scope, mod); ok {
				n.Name = f
			}
		} else if n.NameToken != nil {
			if f, ok := w.full(n.NameToken.StringValue(), scope, mod); ok {
				n.NameToken.Value = f
			}
		}
	}
}
