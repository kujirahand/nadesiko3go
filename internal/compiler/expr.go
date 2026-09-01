package compiler

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// binaryOps maps the parser's operator names to IR operators.
var binaryOps = map[string]ir.BinaryOp{
	"+": ir.BinAdd, "-": ir.BinSub, "*": ir.BinMul,
	"/": ir.BinDiv, "÷": ir.BinDiv, "÷÷": ir.BinIntDiv,
	"%": ir.BinMod, "^": ir.BinPow, "**": ir.BinPow,
	"&":  ir.BinConcat,
	"eq": ir.BinEq, "noteq": ir.BinNotEq,
	"===": ir.BinStrictEq, "!==": ir.BinStrictNotEq,
	"lt": ir.BinLt, "lteq": ir.BinLtEq, "gt": ir.BinGt, "gteq": ir.BinGtEq,
	"shift_l": ir.BinShiftL, "shift_r": ir.BinShiftR, "shift_r0": ir.BinShiftR0,
}

// compileExpr compiles a node that produces exactly one value on the stack.
func (c *Compiler) compileExpr(n *ast.Node) {
	if n == nil {
		c.emit(ir.OpConst, c.constant(ir.Const{Kind: ir.ConstUndefined}), 0, nil)
		return
	}
	switch n.Type {
	case ast.Nop, ast.EOL, ast.Comment:
		// 値が要る場所に来たら『それ』を使う。引数の省略がこの形になる。
		c.emit(ir.OpLoadLocal, SoreSlot, 0, n)
		return

	case ast.Number:
		c.emit(ir.OpConst, c.constNumber(n.NumberValue()), 0, n)
		return

	case ast.String:
		c.emit(ir.OpConst, c.constString(n.StringValue()), 0, n)
		return

	case ast.Bool:
		b, _ := n.Value.(bool)
		c.emit(ir.OpConst, c.constant(ir.Const{Kind: ir.ConstBool, Bool: b}), 0, n)
		return

	case ast.Null:
		c.emit(ir.OpConst, c.constant(ir.Const{Kind: ir.ConstNull}), 0, n)
		return

	case ast.Word, ast.Variable:
		c.loadVar(n.StringValue(), n)
		return

	case ast.Op:
		c.compileOp(n)
		return

	case ast.Not:
		c.compileExpr(n.Block(0))
		c.emit(ir.OpUnary, int(ir.UnaryNot), 0, n)
		return

	case ast.JSONArray:
		for _, item := range n.Blocks {
			c.compileExpr(item)
		}
		c.emit(ir.OpMakeArray, 0, len(n.Blocks), n)
		return

	case ast.JSONObj:
		for _, item := range n.Blocks {
			c.compileExpr(item)
		}
		c.emit(ir.OpMakeDict, 0, len(n.Blocks)/2, n)
		return

	case ast.RefArray, ast.RefProp:
		c.compileRefIndex(n)
		return

	case ast.RefArrayValue:
		// index[0] は対象の値、残りが添字
		if len(n.Index) == 0 {
			c.fail("配列アクセスの指定がありません。", n)
		}
		c.compileExpr(n.Index[0])
		for _, idx := range n.Index[1:] {
			c.compileExpr(idx)
		}
		c.emit(ir.OpIndexGet, 0, len(n.Index)-1, n)
		return

	case ast.Func, ast.CalcFunc:
		c.compileCall(n)
		return

	case ast.CallValue:
		c.compileCallValue(n)
		return

	case ast.FuncObj:
		c.compileFuncObj(n)
		return

	case ast.FuncPointer:
		c.compileFuncPointer(n)
		return

	case ast.Renbun:
		// 左を評価して捨て、右の値を式の値にする
		c.compileStatement(n.Block(0))
		c.compileExpr(n.Block(1))
		return

	case ast.Block:
		// 括弧の中に文が書かれた場合。最後の『それ』が値になる。
		c.compileBlockValue(n)
		c.emit(ir.OpLoadLocal, SoreSlot, 0, n)
		return
	}

	c.fail(fmt.Sprintf("『%s』はまだ実行に対応していません。", n.Type), n)
}

// compileOp compiles a binary operator. 『かつ』 and 『または』 short-circuit and
// yield the operand rather than a boolean, as they do in JavaScript.
func (c *Compiler) compileOp(n *ast.Node) {
	switch n.Operator {
	case "and", "or":
		c.compileExpr(n.Block(0))
		c.emit(ir.OpDup, 0, 0, n)
		var skip int
		if n.Operator == "and" {
			skip = c.emit(ir.OpJumpIfFalse, 0, 0, n)
		} else {
			skip = c.emit(ir.OpJumpIfTrue, 0, 0, n)
		}
		c.emit(ir.OpPop, 0, 0, n) // 判定に使わなかった左辺を捨てる
		c.compileExpr(n.Block(1))
		c.patch(skip, c.here())
		return
	}

	op, ok := binaryOps[n.Operator]
	if !ok {
		c.fail(fmt.Sprintf("演算子『%s』はまだ実行に対応していません。", n.Operator), n)
	}
	c.compileExpr(n.Block(0))
	c.compileExpr(n.Block(1))
	c.emit(ir.OpBinary, int(op), 0, n)
}

// compileRefIndex compiles 『A[1]』 and 『A$名前』.
func (c *Compiler) compileRefIndex(n *ast.Node) {
	c.compileRefBase(n)
	for _, idx := range n.Index {
		c.compileExpr(idx)
	}
	c.emit(ir.OpIndexGet, 0, len(n.Index), n)
}

// compileRefBase pushes the container a reference reads from.
func (c *Compiler) compileRefBase(n *ast.Node) {
	if n.Name != "" {
		c.loadVar(n.Name, n)
		return
	}
	if n.NameToken != nil {
		c.loadVar(n.NameToken.StringValue(), n)
		return
	}
	c.fail("配列アクセスの対象が分かりません。", n)
}

// compileCall compiles a command or user function call. A command that returns
// a value also assigns it to 『それ』, which is how the next statement can use
// it without naming it.
func (c *Compiler) compileCall(n *ast.Node) {
	for _, a := range n.Blocks {
		c.compileExpr(a)
	}
	argc := len(n.Blocks)

	if index, ok := c.userFuncs[n.Name]; ok {
		c.emit(ir.OpCallUser, index, argc, n)
		c.emit(ir.OpDup, 0, 0, n)
		c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
		return
	}

	entry, ok := c.registry.Lookup(n.Name)
	if !ok {
		// 命令にも利用者定義関数にもない名前。変数に入った関数とみなして呼ぶ。
		// 『F=関数(...)…ここまで』のあとの『F()』がこの形になる。
		c.compileCallVariable(n)
		return
	}
	c.emit(ir.OpCallStd, entry.ID, argc, n)
	if entry.Item.ReturnNone {
		// 戻り値のない命令。呼び出しの結果は捨て、『それ』も変えない。
		c.emit(ir.OpPop, 0, 0, n)
		c.emit(ir.OpConst, c.constant(ir.Const{Kind: ir.ConstUndefined}), 0, n)
		return
	}
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
}

// compileCallVariable calls a function held in a variable. The arguments are
// already on the stack, so the callee has to go underneath them; it is easier
// to re-emit both in the right order.
func (c *Compiler) compileCallVariable(n *ast.Node) {
	// さきほど積んだ引数を捨ててから、呼び出す値と引数を積み直す
	for range n.Blocks {
		c.emit(ir.OpPop, 0, 0, n)
	}
	c.loadVar(n.Name, n)
	for _, a := range n.Blocks {
		c.compileExpr(a)
	}
	c.emit(ir.OpCallValue, 0, len(n.Blocks), n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
}

// callStdByName emits a call to a command by name, for the code the compiler
// generates itself.
func (c *Compiler) callStdByName(name string, argc int, n *ast.Node) {
	entry, ok := c.registry.Lookup(name)
	if !ok {
		c.fail(fmt.Sprintf("関数『%s』が見つかりません。", name), n)
	}
	c.emit(ir.OpCallStd, entry.ID, argc, n)
}

// compileCallValue compiles 『F(...)』 where F is a value holding a function.
func (c *Compiler) compileCallValue(n *ast.Node) {
	if len(n.Blocks) == 0 {
		c.fail("関数呼び出しの対象がありません。", n)
	}
	c.compileExpr(n.Block(0))
	for _, a := range n.Blocks[1:] {
		c.compileExpr(a)
	}
	c.emit(ir.OpCallValue, 0, len(n.Blocks)-1, n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
}

// compileFuncPointer pushes a reference to a named function.
func (c *Compiler) compileFuncPointer(n *ast.Node) {
	index, ok := c.userFuncs[n.Name]
	if !ok {
		c.fail(fmt.Sprintf("関数『%s』への参照を取得できません。", n.Name), n)
	}
	c.emit(ir.OpMakeFunc, index, 0, n)
}
