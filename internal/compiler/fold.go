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

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/ops"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// foldConst evaluates n at compile time when it is made only of literals and
// operators. It reports false for anything that reads a variable, calls a
// command, or otherwise needs the program to be running.
func foldConst(n *ast.Node) (value.Value, bool) {
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

	case ast.Not:
		v, ok := foldConst(n.Block(0))
		if !ok {
			return value.Undefined(), false
		}
		return ops.Unary(ir.UnaryNot, v), true

	case ast.Op:
		return foldOp(n)
	}
	return value.Undefined(), false
}

// foldOp folds a binary operator node. 『かつ』 and 『または』 are left alone:
// they short-circuit and yield an operand rather than a boolean, so they are
// control flow, not arithmetic.
func foldOp(n *ast.Node) (value.Value, bool) {
	if n.Operator == "and" || n.Operator == "or" {
		return value.Undefined(), false
	}
	op, ok := binaryOps[n.Operator]
	if !ok {
		return value.Undefined(), false
	}
	a, ok := foldConst(n.Block(0))
	if !ok {
		return value.Undefined(), false
	}
	b, ok := foldConst(n.Block(1))
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
		return c.constString(s), true
	}
	return 0, false
}

// tryEmitFolded emits n as a single constant when it can be folded, and
// reports whether it did.
func (c *Compiler) tryEmitFolded(n *ast.Node) bool {
	v, ok := foldConst(n)
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
