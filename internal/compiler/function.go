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
	if _, ok := c.resolveLocal(name); ok {
		return
	}
	c.slot(name)
}

// compileFuncBody emits a function body into prog.Funcs[index].
//
// Arguments arrive on the stack in source order, so the body pops them into
// their slots in reverse. Slot 0 is 『それ』, which is also the return value
// when the function ends without a 『戻る』.
func (c *Compiler) compileFuncBody(index int, n *ast.Node) {
	fn := &funcCtx{name: n.Name, slots: map[string]int{"それ": SoreSlot}, numVars: 1}
	if fn.name == "" {
		fn.name = "(無名関数)"
	}

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

	// 本体が代入する名前を先にスロットにする。こうしないと関数の中の代入が
	// グローバルになってしまい、入れ子の関数から捕捉することもできない。
	c.declareLocals(n.Block(0))

	// 引数は呼び出し側がスロットへ直接入れるので、本体は空のスタックで始まる
	c.emit(ir.OpConst, c.constString(""), 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n) // 『それ』の初期値は空文字列
	c.compileStatement(n.Block(0))
	// 『戻る』がないまま終わったら『それ』を返す
	c.emit(ir.OpLoadLocal, SoreSlot, 0, n)
	c.emit(ir.OpReturn, 1, 0, n)

	c.prog.Funcs[index] = ir.Func{
		Name:     fn.name,
		Params:   params,
		NumVars:  fn.numVars,
		Code:     fn.code,
		Async:    n.AsyncFn,
		Captures: fn.captures,
		MaxStack: c.maxStack(index, fn.code),
	}

	c.fn = c.fnStack[len(c.fnStack)-1]
	c.fnStack = c.fnStack[:len(c.fnStack)-1]
}
