package compiler

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// compileDefFunc compiles a named function definition. The body is emitted into
// its own ir.Func; nothing is left on the stack where the definition appeared.
func (c *Compiler) compileDefFunc(n *ast.Node) {
	index, ok := c.userFuncs[n.Name]
	if !ok {
		// declareFuncs が拾えなかった定義。無名扱いで新しい枠を作る。
		c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: n.Name})
		index = len(c.prog.Funcs) - 1
		c.userFuncs[n.Name] = index
	}
	c.compileFuncBody(index, n)
}

// compileFuncObj compiles an anonymous function and pushes it as a value.
func (c *Compiler) compileFuncObj(n *ast.Node) {
	c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: "(無名関数)"})
	index := len(c.prog.Funcs) - 1
	c.compileFuncBody(index, n)
	c.emit(ir.OpMakeFunc, index, 0, n)
}

// nestedBinding は、関数の中に定義された名前付き関数を、その関数の値として
// 定義側のフレームへ束縛するための情報 (#420)。
type nestedBinding struct {
	index int       // prog.Funcs の添字
	slot  int       // 定義側フレームの隠しスロット
	node  *ast.Node // 定義の節点 (位置情報用)
}

// nestedFuncKey は、入れ子関数を定義側フレームのスロットへ束縛するときの
// 内部名を作る。tempName と同じ前提で、$ を含む名前はソースには現れないので
// 利用者の変数と衝突しない。
func nestedFuncKey(name string) string { return "$関数:" + name }

// declareNestedFuncs reserves a hidden local slot in the function being
// compiled for every named function defined directly in its body, without
// descending into a nested function definition, which binds its own children.
//
// 名前付きの入れ子関数は、定義側のフレームで一度だけクロージャにして
// このスロットへ入れる (compileFuncBody)。呼び出し側は通常の変数と同じ
// 経路 (ローカル読み、または captureInto による捕捉) でその値を取り出して
// 呼ぶので、捕捉を持つ関数を OpCallUser で直接呼ぶことがなくなる (#420)。
func (c *Compiler) declareNestedFuncs(body *ast.Node) []nestedBinding {
	var out []nestedBinding
	var walk func(n *ast.Node)
	walk = func(n *ast.Node) {
		if n == nil {
			return
		}
		switch n.Type {
		case ast.FuncObj:
			return
		case ast.DefFunc, ast.DefTest:
			if n.Name != "" {
				key := nestedFuncKey(n.Name)
				if _, seen := c.fn.slots[key]; !seen {
					index, ok := c.userFuncs[n.Name]
					if !ok {
						// declareFuncs が拾えなかった定義。compileDefFunc と
						// 同じく新しい枠を作る。
						c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: n.Name})
						index = len(c.prog.Funcs) - 1
						c.userFuncs[n.Name] = index
					}
					slot := c.slot(key)
					out = append(out, nestedBinding{index: index, slot: slot, node: n})
				}
			}
			return
		}
		for _, b := range n.Blocks {
			walk(b)
		}
	}
	walk(body)
	return out
}

// nestedFuncVisible reports whether the nested function bound under key can
// be reached from the function being compiled: some enclosing function holds
// it in a hidden slot, or already captures it. captureInto can thread it the
// rest of the way, the way it threads any variable.
func (c *Compiler) nestedFuncVisible(key string) bool {
	for depth := 0; depth <= len(c.fnStack); depth++ {
		fn := c.funcAt(depth)
		if _, ok := fn.slots[key]; ok {
			return true
		}
		if _, ok := fn.captureIndex[key]; ok {
			return true
		}
	}
	return false
}

// declareLocals reserves a slot for every name the body assigns to, without
// descending into a nested function definition, which has its own scope.
//
// A name carrying a module prefix was resolved to a global by the parser and
// stays one.
func (c *Compiler) declareLocals(n *ast.Node) {
	if n == nil {
		return
	}
	switch n.Type {
	case ast.DefFunc, ast.DefTest, ast.FuncObj:
		return
	case ast.Let, ast.DefLocalVar:
		c.declareLocalName(n.Name)
	case ast.DefLocalVarList:
		for _, name := range n.Names {
			c.declareLocalName(name.StringValue())
		}
	case ast.For, ast.Foreach:
		c.declareLocalName(n.Word)
	}
	for _, b := range n.Blocks {
		c.declareLocals(b)
	}
}

func (c *Compiler) declareLocalName(name string) {
	if name == "" || strings.Contains(name, "__") {
		return
	}
	// 外側の関数が持っている名前なら、新しいスロットを作らずに捕捉する。
	// そうしないと、閉じ込めた変数を内側の代入が隠してしまう。
	if _, ok := c.fn.slots[name]; ok {
		return
	}
	if _, ok := c.captureInto(len(c.fnStack), name); ok {
		return
	}
	c.slot(name)
}

// compileFuncBody emits a function body into prog.Funcs[index].
//
// The caller puts the arguments straight into their slots, so the body starts
// with an empty operand stack. 『それ』 is the return value when the function
// ends without a 『戻る』.
func (c *Compiler) compileFuncBody(index int, n *ast.Node) {
	name := n.Name
	if name == "" {
		name = "(無名関数)"
	}
	fn := newFuncCtx(name)

	params := make([]ir.Param, 0, len(n.Args))
	for _, a := range n.Args {
		name := a.StringValue()
		if name == "" {
			continue
		}
		slot := fn.numVars
		fn.numVars++
		fn.slots[name] = slot
		params = append(params, ir.Param{Name: name, Slot: slot})
	}

	c.fnStack = append(c.fnStack, c.fn)
	c.fn = fn
	// 関数の本体は、呼ばれるたび先頭から順に動く。外側の『もし』や
	// ループの中に定義が書かれていても、本体の中では直線として数える。
	outerDepth := c.branchDepth
	c.branchDepth = 0

	// 直下の名前付き関数の隠しスロットを先に予約する。本体の中で
	// 定義より前に書かれた呼び出しからも束縛が見えるようにするため (#420)。
	bindings := c.declareNestedFuncs(n.Block(0))

	// 本体が代入する名前を先にスロットにする。こうしないと関数の中の代入が
	// グローバルになってしまい、入れ子の関数から捕捉することもできない。
	c.declareLocals(n.Block(0))

	// 引数は呼び出し側がスロットへ直接入れるので、本体は空のスタックで始まる
	c.emit(ir.OpLoadConst, c.constString(""), 0, n)
	c.emit(ir.OpStoreSpecial, int(ir.SpecialSore), 0, n) // 『それ』の初期値は空文字列
	// 入れ子関数のクロージャは、本体の文が動く前に一度だけ作って隠し
	// スロットへ入れる。クロージャはセルを共有するので、このあと外側の
	// 変数へ代入しても束縛済みの関数値から新しい値が見える。定義より
	// 前に書かれた呼び出しや、入れ子関数自身からの再帰もこの束縛で動く。
	for _, b := range bindings {
		c.emit(ir.OpMakeFunc, b.index, 0, b.node)
		c.emit(ir.OpStoreLocal, b.slot, 0, b.node)
	}
	c.compileStatement(n.Block(0))
	// 『戻る』がないまま終わったら『それ』を返す
	c.emit(ir.OpLoadSpecial, int(ir.SpecialSore), 0, n)
	c.emit(ir.OpReturn, 1, 0, n)

	code := optimize(fn.code)
	c.prog.Funcs[index] = ir.Func{
		Name:        fn.name,
		Params:      params,
		NumVars:     fn.numVars,
		ConstVars:   sortedSlots(fn.constSlots),
		NumCaptures: len(fn.captures),
		Code:        code,
		Async:       n.AsyncFn,
		Captures:    fn.captures,
		MaxStack:    c.maxStack(index, code),
	}

	c.branchDepth = outerDepth
	c.fn = c.fnStack[len(c.fnStack)-1]
	c.fnStack = c.fnStack[:len(c.fnStack)-1]
}
