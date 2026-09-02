package compiler

// 定数畳み込み。『(1+2)*3』のように、実行するまでもなく値が決まる式は
// コンパイル時に1つの定数へ潰す。VMもgogenも同じだけ得をする
// (AGENTS.md §12: gogenは速度のためのバックエンドなので、演算1回あたりの
// 経路が深いぶん、そもそも演算を減らせるならそれが一番効く)。
//
// 畳み込みは internal/ops を通して行う。実行時と同じコードで評価するので、
// 「畳み込んだ結果」と「実行して得られる結果」が食い違うことはない。
// NaN や -0、文字列連結の型変換といったJavaScript由来の細かい規則
// (AGENTS.md §4) も、実行時と同じ実装がそのまま効く。
//
// 効き方の例。『((1+2)*3)を表示』の生成コードは、畳み込み前は定数3つと
// 演算2回（スタック操作5回）だったのが、定数1つになる。
//
// 定数伝播。『定数』で宣言された名前は、コンパイル時に代入が拒否される
// (compiler.go の storeVar / declareConst) ので値が動かない。値がリテラルで
// 決まるものは名前と値の対応を覚えておき、あとの式で名前が出てきたら値に
// 置き換える。こうすると『定数Nは10』『(N*N)を表示』が定数1つに潰れる。
//
// 覚えるのは「関数の直線上にある宣言」だけ。もし文やループの中の宣言は
// 実行されないまま先へ進むことがあり、そのとき現在の実装は未定義を読むので、
// 値に置き換えると挙動が変わってしまう (branchDepth がこれを数えている)。
// 同じ理由で、入れ子の関数の中から外側の定数を引くこともしない。関数は
// 定義より前の行から呼べる (declareFuncs) ので、宣言がまだ実行されていない
// 状態で本体が動きうる。

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/ops"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// maxFoldedString caps the string a fold may produce. 『"あ"*1000』 のような式を
// 畳み込むと、実行時には一度しか作らない文字列を定数プールに抱え込むことに
// なるので、大きくなったら実行時に任せる。
const maxFoldedString = 4096

// foldConst evaluates n at compile time when it is made only of literals,
// operators, and 『定数』 whose value is already known. It reports false for
// anything that reads a variable, calls a command, or otherwise needs the
// program to be running.
func (c *Compiler) foldConst(n *ast.Node) (value.Value, bool) {
	if n == nil {
		return value.Undefined(), false
	}
	switch n.Type {
	case ast.Number:
		return value.Number(n.NumberValue()), true

	case ast.String:
		return value.String(n.StringValue()), true

	case ast.Bool:
		b, ok := n.Value.(bool)
		if !ok {
			return value.Undefined(), false
		}
		return value.Bool(b), true

	case ast.Null:
		return value.Null(), true

	case ast.Word, ast.Variable:
		// 添字が付いていたら『A[0]』であって名前そのものではない
		if len(n.Index) != 0 {
			return value.Undefined(), false
		}
		return c.constValueOf(n.StringValue())

	case ast.Not:
		v, ok := c.foldConst(n.Block(0))
		if !ok {
			return value.Undefined(), false
		}
		return ops.Unary(ir.UnaryNot, v), true

	case ast.Op:
		return c.foldOp(n)
	}
	return value.Undefined(), false
}

// foldOp folds a binary operator node. 『かつ』 and 『または』 are left alone:
// they short-circuit and yield an operand rather than a boolean, so they are
// control flow, not arithmetic.
func (c *Compiler) foldOp(n *ast.Node) (value.Value, bool) {
	if n.Operator == "and" || n.Operator == "or" {
		return value.Undefined(), false
	}
	op, ok := binaryOps[n.Operator]
	if !ok {
		return value.Undefined(), false
	}
	a, ok := c.foldConst(n.Block(0))
	if !ok {
		return value.Undefined(), false
	}
	b, ok := c.foldConst(n.Block(1))
	if !ok {
		return value.Undefined(), false
	}
	return ops.Binary(op, a, b), true
}

// constantOf turns a folded value into a constant pool entry. It reports
// false for a value the pool cannot hold, which keeps a fold from quietly
// changing what the program does.
func (c *Compiler) constantOf(v value.Value) (int, bool) {
	switch v.Kind() {
	case value.KindNull:
		return c.constant(ir.Const{Kind: ir.ConstNull}), true
	case value.KindBool:
		b, _ := v.Bool()
		return c.constant(ir.Const{Kind: ir.ConstBool, Bool: b}), true
	case value.KindNumber:
		f, _ := v.Number()
		// NaN と ±Inf は Const に入れてもJSON往復で壊れる(IRは §6 のとおり
		// 直列化可能でなければならない)ので、畳み込まず実行時に任せる。
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return c.constNumber(f), true
	case value.KindString:
		s, _ := v.String()
		if len(s) > maxFoldedString {
			return 0, false
		}
		return c.constString(s), true
	}
	return 0, false
}

// tryEmitFolded emits n as a single constant when it can be folded, and
// reports whether it did.
func (c *Compiler) tryEmitFolded(n *ast.Node) bool {
	v, ok := c.foldConst(n)
	if !ok {
		return false
	}
	index, ok := c.constantOf(v)
	if !ok {
		return false
	}
	c.emit(ir.OpLoadConst, index, 0, n)
	return true
}

// constValueOf returns the value of a name declared 『定数』, when this compiler
// has recorded one. It must not have side effects: folding is speculative, and
// Compiler.resolve would allocate a global slot or thread a capture for a name
// the emitted code never ends up reading.
func (c *Compiler) constValueOf(name string) (value.Value, bool) {
	if name == "" {
		return value.Undefined(), false
	}
	if _, ok := ir.SpecialByName(name); ok {
		return value.Undefined(), false // 『それ』などは定数ではない
	}
	if slot, ok := c.fn.slots[name]; ok {
		v, ok := c.fn.constLocals[slot]
		return v, ok
	}
	if len(c.fnStack) != 0 {
		return value.Undefined(), false // 入れ子の関数からは外側を辿らない
	}
	if slot, ok := c.globalIndex[name]; ok {
		v, ok := c.constGlobalValues[slot]
		return v, ok
	}
	return value.Undefined(), false
}

// recordConstValue remembers the value of a 『定数』 just declared, so later
// expressions can fold it away. It is called after declareConst, which has
// already allocated the slot and refused a redeclaration.
func (c *Compiler) recordConstValue(name string, init *ast.Node) {
	if c.branchDepth != 0 {
		return // 実行されるとは限らない宣言
	}
	v, ok := c.foldConst(init)
	if !ok {
		return
	}
	// 定数プールに入らない値 (NaN・巨大な文字列) は伝播させても
	// 定数として出せないので、覚えない。
	if _, ok := c.constantOf(v); !ok {
		return
	}
	if slot, ok := c.fn.slots[name]; ok {
		c.fn.constLocals[slot] = v
		return
	}
	if len(c.fnStack) != 0 {
		return
	}
	if slot, ok := c.globalIndex[name]; ok {
		c.constGlobalValues[slot] = v
	}
}
