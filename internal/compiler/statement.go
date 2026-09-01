package compiler

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// declareFuncs walks the tree and reserves a slot in prog.Funcs for every named
// function, so that a call can be compiled before the definition is reached.
func (c *Compiler) declareFuncs(n *ast.Node) {
	if n == nil {
		return
	}
	if (n.Type == ast.DefFunc || n.Type == ast.DefTest) && n.Name != "" {
		if _, seen := c.userFuncs[n.Name]; !seen {
			c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: n.Name})
			c.userFuncs[n.Name] = len(c.prog.Funcs) - 1
		}
	}
	for _, b := range n.Blocks {
		c.declareFuncs(b)
	}
}

// compileBlockValue compiles a block as a sequence of statements. The value of
// the block is whatever 『それ』 holds afterwards.
func (c *Compiler) compileBlockValue(n *ast.Node) {
	if n == nil {
		return
	}
	for _, b := range n.Blocks {
		c.compileStatement(b)
	}
}

// compileStatement compiles one statement, leaving nothing on the stack.
func (c *Compiler) compileStatement(n *ast.Node) {
	if n == nil {
		return
	}
	switch n.Type {
	case ast.Nop, ast.EOL, ast.Comment, ast.RunMode:
		return

	case ast.Block:
		c.compileBlockValue(n)
		return

	case ast.DefFunc, ast.DefTest:
		c.compileDefFunc(n)
		return

	case ast.Let:
		c.compileExpr(n.Block(0))
		c.storeVar(n.Name, n)
		return

	case ast.DefLocalVar:
		// 関数の中なら局所変数、外ならモジュール変数。名前は構文解析で解決済み。
		if c.fn.name != "main" {
			c.slot(n.Name)
		}
		c.compileExpr(n.Block(0))
		c.storeVar(n.Name, n)
		return

	case ast.DefLocalVarList:
		c.compileDefVarList(n)
		return

	case ast.LetArray:
		c.compileLetArray(n)
		return

	case ast.Inc:
		c.compileInc(n)
		return

	case ast.If:
		c.compileIf(n)
		return

	case ast.While:
		c.compileWhile(n)
		return

	case ast.Atohantei:
		c.compileAtohantei(n)
		return

	case ast.RepeatTimes:
		c.compileRepeatTimes(n)
		return

	case ast.For:
		c.compileFor(n)
		return

	case ast.Foreach:
		c.compileForeach(n)
		return

	case ast.Switch:
		c.compileSwitch(n)
		return

	case ast.TryExcept:
		c.compileTryExcept(n)
		return

	case ast.Break:
		c.emitLoopJump(n, true)
		return

	case ast.Continue:
		c.emitLoopJump(n, false)
		return

	case ast.Return:
		c.compileExpr(n.Block(0))
		c.emit(ir.OpReturn, 1, 0, n)
		return

	case ast.SpeedMode, ast.PerformanceMonitor:
		// 実行速度やモニタの指定は意味を変えないので、中身だけを実行する
		c.compileStatement(n.Block(0))
		return
	}

	// 残りは値を作る式。文として書かれた場合は結果を捨てる。
	c.compileExpr(n)
	c.emit(ir.OpPop, 0, 0, n)
}

// compileDefVarList compiles 『変数[A,B]=[1,2]』.
func (c *Compiler) compileDefVarList(n *ast.Node) {
	c.compileExpr(n.Block(0))
	for i, name := range n.Names {
		if i < len(n.Names)-1 {
			c.emit(ir.OpDup, 0, 0, n)
		}
		c.emit(ir.OpConst, c.constNumber(float64(i)), 0, n)
		c.emit(ir.OpIndexGet, 0, 1, n)
		if c.fn.name != "main" {
			c.slot(name.StringValue())
		}
		c.storeVar(name.StringValue(), n)
	}
	if len(n.Names) == 0 {
		c.emit(ir.OpPop, 0, 0, n)
	}
}

// compileLetArray compiles 『A[1]=値』. blocks[0] is the value and the rest are
// the indexes, in the order they were written.
func (c *Compiler) compileLetArray(n *ast.Node) {
	indexes := n.Blocks[1:]
	c.loadVar(n.Name, n)
	for _, idx := range indexes {
		c.compileExpr(idx)
	}
	c.compileExpr(n.Block(0))
	c.emit(ir.OpIndexSet, 0, len(indexes), n)
	// 代入で作り直された可能性があるので、変数に書き戻す
	c.storeVar(n.Name, n)
}

// compileInc compiles 『(変数)を(値)だけ増やす』.
func (c *Compiler) compileInc(n *ast.Node) {
	if len(n.Index) > 0 {
		// 添字付きの対象。読んで足して書き戻す。
		c.loadVar(n.Name, n)
		for _, idx := range n.Index {
			c.compileExpr(idx)
		}
		c.loadVar(n.Name, n)
		for _, idx := range n.Index {
			c.compileExpr(idx)
		}
		c.emit(ir.OpIndexGet, 0, len(n.Index), n)
		c.compileExpr(n.Block(0))
		c.emit(ir.OpBinary, int(ir.BinAdd), 0, n)
		c.emit(ir.OpIndexSet, 0, len(n.Index), n)
		c.storeVar(n.Name, n)
		return
	}
	c.loadVar(n.Name, n)
	c.compileExpr(n.Block(0))
	c.emit(ir.OpBinary, int(ir.BinAdd), 0, n)
	c.storeVar(n.Name, n)
}

// --- 制御構文 ---

func (c *Compiler) compileIf(n *ast.Node) {
	c.compileExpr(n.Block(0))
	toElse := c.emit(ir.OpJumpIfFalse, 0, 0, n)
	c.compileStatement(n.Block(1))
	toEnd := c.emit(ir.OpJump, 0, 0, n)
	c.patch(toElse, c.here())
	c.compileStatement(n.Block(2))
	c.patch(toEnd, c.here())
}

func (c *Compiler) compileWhile(n *ast.Node) {
	top := c.here()
	c.compileExpr(n.Block(0))
	toEnd := c.emit(ir.OpJumpIfFalse, 0, 0, n)
	loop := c.pushLoop()
	c.compileStatement(n.Block(1))
	c.patchContinues(loop, top)
	c.emit(ir.OpJump, top, 0, n)
	c.patch(toEnd, c.here())
	c.popLoop(loop)
}

// compileAtohantei compiles the loop that tests its condition after the body.
func (c *Compiler) compileAtohantei(n *ast.Node) {
	top := c.here()
	loop := c.pushLoop()
	c.compileStatement(n.Block(1))
	condAt := c.here()
	c.patchContinues(loop, condAt)
	c.compileExpr(n.Block(0))
	c.emit(ir.OpJumpIfTrue, top, 0, n)
	c.popLoop(loop)
}

// compileRepeatTimes compiles 『N回…ここまで』. 『回数』 counts from 1.
//
// The previous 『回数』 is saved and put back afterwards, so a nested loop does
// not clobber the count of the loop around it. 『それ』 is restored from the
// same saved value, which is what the TypeScript version does.
func (c *Compiler) compileRepeatTimes(n *ast.Node) {
	counter := c.slot(c.tempName("回"))
	limit := c.slot(c.tempName("回上限"))
	saved := c.slot(c.tempName("回数退避"))

	c.emit(ir.OpLoadGlobal, c.constString(SysCount), 0, n)
	c.emit(ir.OpStoreLocal, saved, 0, n)

	c.compileExpr(n.Block(0))
	c.emit(ir.OpStoreLocal, limit, 0, n)
	c.emit(ir.OpConst, c.constNumber(1), 0, n)
	c.emit(ir.OpStoreLocal, counter, 0, n)

	top := c.here()
	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpLoadLocal, limit, 0, n)
	c.emit(ir.OpBinary, int(ir.BinLtEq), 0, n)
	toEnd := c.emit(ir.OpJumpIfFalse, 0, 0, n)

	// 『回数』と『それ』に現在の回数を入れる
	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysCount), 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)

	loop := c.pushLoop()
	c.compileStatement(n.Block(1))
	c.patchContinues(loop, c.here())

	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpConst, c.constNumber(1), 0, n)
	c.emit(ir.OpBinary, int(ir.BinAdd), 0, n)
	c.emit(ir.OpStoreLocal, counter, 0, n)
	c.emit(ir.OpJump, top, 0, n)
	c.patch(toEnd, c.here())
	c.popLoop(loop)

	// 抜けた後に退避しておいた値へ戻す。『抜ける』もここへ来る。
	c.emit(ir.OpLoadLocal, saved, 0, n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysCount), 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
}

// compileFor compiles 『AからBまで繰り返す』.
func (c *Compiler) compileFor(n *ast.Node) {
	counter := c.slot(c.tempName("繰返"))
	limit := c.slot(c.tempName("繰返上限"))
	step := c.slot(c.tempName("繰返増分"))

	from, to, inc := n.Block(0), n.Block(1), n.Block(2)

	// 『AからBの範囲を繰り返す』のときは範囲オブジェクトから取り出す (#1704)
	if to != nil && to.Type == ast.Func && to.Name == "範囲" {
		rangeVar := c.slot(c.tempName("範囲"))
		c.compileExpr(to)
		c.emit(ir.OpStoreLocal, rangeVar, 0, n)
		c.emit(ir.OpLoadLocal, rangeVar, 0, n)
		c.emit(ir.OpConst, c.constString("先頭"), 0, n)
		c.emit(ir.OpIndexGet, 0, 1, n)
		c.emit(ir.OpStoreLocal, counter, 0, n)
		c.emit(ir.OpLoadLocal, rangeVar, 0, n)
		c.emit(ir.OpConst, c.constString("末尾"), 0, n)
		c.emit(ir.OpIndexGet, 0, 1, n)
		c.emit(ir.OpStoreLocal, limit, 0, n)
	} else {
		c.compileExpr(from)
		c.emit(ir.OpStoreLocal, counter, 0, n)
		c.compileExpr(to)
		c.emit(ir.OpStoreLocal, limit, 0, n)
	}

	if inc != nil && inc.Type != ast.Nop {
		c.compileExpr(inc)
	} else {
		c.emit(ir.OpConst, c.constNumber(1), 0, n)
	}
	if n.LoopDirection == "down" {
		c.emit(ir.OpUnary, int(ir.UnaryNeg), 0, n)
	}
	c.emit(ir.OpStoreLocal, step, 0, n)

	top := c.here()
	// 増分の向きで比較を変える。減る向きなら下限との比較になる。
	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpLoadLocal, limit, 0, n)
	if n.LoopDirection == "down" {
		c.emit(ir.OpBinary, int(ir.BinGtEq), 0, n)
	} else {
		c.emit(ir.OpBinary, int(ir.BinLtEq), 0, n)
	}
	toEnd := c.emit(ir.OpJumpIfFalse, 0, 0, n)

	// ループ変数と『それ』に現在値を入れる
	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
	if n.Word != "" {
		if c.fn.name != "main" || c.isLocal(n.Word) {
			c.slot(n.Word)
		}
		c.storeVar(n.Word, n)
	} else {
		// ループ変数がなければ『それ』だけ。TS版のconvForは『対象』を設定しない。
		c.emit(ir.OpPop, 0, 0, n)
	}

	loop := c.pushLoop()
	c.compileStatement(n.Block(3))
	c.patchContinues(loop, c.here())

	c.emit(ir.OpLoadLocal, counter, 0, n)
	c.emit(ir.OpLoadLocal, step, 0, n)
	c.emit(ir.OpBinary, int(ir.BinAdd), 0, n)
	c.emit(ir.OpStoreLocal, counter, 0, n)
	c.emit(ir.OpJump, top, 0, n)
	c.patch(toEnd, c.here())
	c.popLoop(loop)
}

// compileForeach compiles 『(配列)を反復』, setting 『対象』 and 『対象キー』.
//
// The three values the loop takes over — 『対象』『対象キー』『それ』 — are saved
// and put back afterwards, so a nested loop leaves the outer one intact.
func (c *Compiler) compileForeach(n *ast.Node) {
	target := c.slot(c.tempName("反復対象"))
	keys := c.slot(c.tempName("反復キー"))
	index := c.slot(c.tempName("反復添字"))
	savedTarget := c.slot(c.tempName("対象退避"))
	savedKey := c.slot(c.tempName("対象キー退避"))
	savedSore := c.slot(c.tempName("それ退避"))

	c.emit(ir.OpLoadGlobal, c.constString(SysTarget), 0, n)
	c.emit(ir.OpStoreLocal, savedTarget, 0, n)
	c.emit(ir.OpLoadGlobal, c.constString(SysTargetKey), 0, n)
	c.emit(ir.OpStoreLocal, savedKey, 0, n)
	c.emit(ir.OpLoadLocal, SoreSlot, 0, n)
	c.emit(ir.OpStoreLocal, savedSore, 0, n)

	if t := n.Block(0); t != nil && t.Type != ast.Nop {
		c.compileExpr(t)
	} else {
		c.emit(ir.OpLoadLocal, SoreSlot, 0, n) // 対象の指定がなければ『それ』
	}
	c.emit(ir.OpStoreLocal, target, 0, n)

	// 配列なら添字、辞書ならキーの一覧を回す
	c.emit(ir.OpLoadLocal, target, 0, n)
	c.emit(ir.OpIterKeys, 0, 0, n)
	c.emit(ir.OpStoreLocal, keys, 0, n)
	c.emit(ir.OpConst, c.constNumber(0), 0, n)
	c.emit(ir.OpStoreLocal, index, 0, n)

	top := c.here()
	c.emit(ir.OpLoadLocal, index, 0, n)
	c.emit(ir.OpLoadLocal, keys, 0, n)
	c.emit(ir.OpLen, 0, 0, n)
	c.emit(ir.OpBinary, int(ir.BinLt), 0, n)
	toEnd := c.emit(ir.OpJumpIfFalse, 0, 0, n)

	// 『対象キー』に今回のキーを入れる
	c.emit(ir.OpLoadLocal, keys, 0, n)
	c.emit(ir.OpLoadLocal, index, 0, n)
	c.emit(ir.OpIndexGet, 0, 1, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysTargetKey), 0, n)

	// 『対象』と『それ』(必要ならループ変数)に今回の値を入れる
	c.emit(ir.OpLoadLocal, target, 0, n)
	c.emit(ir.OpLoadGlobal, c.constString(SysTargetKey), 0, n)
	c.emit(ir.OpIndexGet, 0, 1, n)
	c.emit(ir.OpDup, 0, 0, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysTarget), 0, n)
	if n.Word != "" {
		c.emit(ir.OpDup, 0, 0, n)
		if c.fn.name != "main" || c.isLocal(n.Word) {
			c.slot(n.Word)
		}
		c.storeVar(n.Word, n)
	}
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)

	loop := c.pushLoop()
	c.compileStatement(n.Block(1))
	c.patchContinues(loop, c.here())

	c.emit(ir.OpLoadLocal, index, 0, n)
	c.emit(ir.OpConst, c.constNumber(1), 0, n)
	c.emit(ir.OpBinary, int(ir.BinAdd), 0, n)
	c.emit(ir.OpStoreLocal, index, 0, n)
	c.emit(ir.OpJump, top, 0, n)
	c.patch(toEnd, c.here())
	c.popLoop(loop)

	// 抜けた後に退避しておいた値へ戻す
	c.emit(ir.OpLoadLocal, savedTarget, 0, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysTarget), 0, n)
	c.emit(ir.OpLoadLocal, savedKey, 0, n)
	c.emit(ir.OpStoreGlobal, c.constString(SysTargetKey), 0, n)
	c.emit(ir.OpLoadLocal, savedSore, 0, n)
	c.emit(ir.OpStoreLocal, SoreSlot, 0, n)
}

// compileSwitch compiles 『(値)で条件分岐』. blocks[0] is the value, blocks[1]
// the 違えば block, and the rest are condition and body pairs.
func (c *Compiler) compileSwitch(n *ast.Node) {
	subject := c.slot(c.tempName("条件分岐"))
	c.compileExpr(n.Block(0))
	c.emit(ir.OpStoreLocal, subject, 0, n)

	var toEnd []int
	for i := 2; i+1 < len(n.Blocks); i += 2 {
		c.emit(ir.OpLoadLocal, subject, 0, n)
		c.compileExpr(n.Blocks[i])
		c.emit(ir.OpBinary, int(ir.BinEq), 0, n)
		toNext := c.emit(ir.OpJumpIfFalse, 0, 0, n)
		c.compileStatement(n.Blocks[i+1])
		toEnd = append(toEnd, c.emit(ir.OpJump, 0, 0, n))
		c.patch(toNext, c.here())
	}
	c.compileStatement(n.Block(1)) // 違えば
	for _, at := range toEnd {
		c.patch(at, c.here())
	}
}

func (c *Compiler) compileTryExcept(n *ast.Node) {
	toHandler := c.emit(ir.OpTry, 0, 0, n)
	c.compileStatement(n.Block(0))
	c.emit(ir.OpEndTry, 0, 0, n)
	toEnd := c.emit(ir.OpJump, 0, 0, n)
	c.patch(toHandler, c.here())
	c.compileStatement(n.Block(1))
	c.patch(toEnd, c.here())
}

// --- ループの出入り ---

func (c *Compiler) pushLoop() *loopCtx {
	l := &loopCtx{}
	c.fn.loops = append(c.fn.loops, l)
	return l
}

// patchContinues points every 『続ける』 in this loop at target. It is called
// once the address of the step section is known.
func (c *Compiler) patchContinues(l *loopCtx, target int) {
	for _, at := range l.continues {
		c.patch(at, target)
	}
}

// popLoop points every 『抜ける』 in this loop just past its end, and leaves the
// enclosing loop current again.
func (c *Compiler) popLoop(l *loopCtx) {
	for _, at := range l.breaks {
		c.patch(at, c.here())
	}
	c.fn.loops = c.fn.loops[:len(c.fn.loops)-1]
}

func (c *Compiler) emitLoopJump(n *ast.Node, isBreak bool) {
	if len(c.fn.loops) == 0 {
		word := "続ける"
		if isBreak {
			word = "抜ける"
		}
		c.fail(fmt.Sprintf("『%s』文がありますが、それは繰り返しの中で利用してください。", word), n)
	}
	l := c.fn.loops[len(c.fn.loops)-1]
	at := c.emit(ir.OpJump, 0, 0, n)
	if isBreak {
		l.breaks = append(l.breaks, at)
	} else {
		l.continues = append(l.continues, at)
	}
}

// tempName builds a slot name the source cannot collide with.
func (c *Compiler) tempName(kind string) string {
	return fmt.Sprintf("$%s%d", kind, c.fn.numVars)
}
