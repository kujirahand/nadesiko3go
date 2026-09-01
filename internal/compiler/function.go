package compiler

import (
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
		p := ir.Param{Name: name}
		params = append(params, p)
		fn.slots[name] = fn.numVars
		fn.numVars++
	}

	c.fnStack = append(c.fnStack, c.fn)
	c.fn = fn

	// 引数はスタックに左から積まれているので、右の引数から順に取り出す
	for i := len(params) - 1; i >= 0; i-- {
		c.emit(ir.OpStoreLocal, fn.slots[params[i].Name], 0, n)
	}
	c.emit(ir.OpConst, c.constString(""), 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n) // 『それ』の初期値は空文字列
	c.compileStatement(n.Block(0))
	// 『戻る』がないまま終わったら『それ』を返す
	c.emit(ir.OpLoadLocal, SoreSlot, 0, n)
	c.emit(ir.OpReturn, 1, 0, n)

	c.prog.Funcs[index] = ir.Func{
		Name:    fn.name,
		Params:  params,
		NumVars: fn.numVars,
		Code:    fn.code,
		Async:   n.AsyncFn,
	}

	c.fn = c.fnStack[len(c.fnStack)-1]
	c.fnStack = c.fnStack[:len(c.fnStack)-1]
}
