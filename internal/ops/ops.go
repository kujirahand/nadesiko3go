// Package ops holds what なでしこ's operators actually do, in one place.
//
// The interpreter (internal/vm), the compiler's constant folding
// (internal/compiler) and the generated-code backend (internal/gogen, through
// pkg/runtime) all evaluate operators through here. Folding 『1+2』 at compile
// time is only safe if it produces exactly what the same expression would
// produce at run time, and the only way to guarantee that is for both to run
// the same code — the same rule §12 already applies to gogen.
//
// The value semantics themselves live in internal/value (AGENTS.md §4: they
// are JavaScript's, deliberately). This package only maps an ir.BinaryOp or
// ir.UnaryOp onto them.
package ops

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Binary applies a binary operator, exactly as the interpreter's OpBinary does.
func Binary(op ir.BinaryOp, a, b value.Value) value.Value {
	// 両辺が数値のときは最短経路を通る。数値に対する ToPrimitive も
	// ToNumber も ParseFloat も値を変えないので、下の一般経路と結果は
	// 同じになる。同じであることは ops_test.go が全演算子×代表値
	// (NaN・±Inf・±0・巨大値…) の総当たりで確かめている。
	if x, ok := a.Number(); ok {
		if y, ok := b.Number(); ok {
			if v, done := binaryNumbers(op, x, y); done {
				return v
			}
		}
	}
	return binaryGeneral(op, a, b)
}

// binaryNumbers is the fast path for two numbers. It reports false for the
// operators it does not shortcut (『&』の連結、シフト、整数割り) so that they
// keep going through the general path rather than being reimplemented here.
func binaryNumbers(op ir.BinaryOp, x, y float64) (value.Value, bool) {
	switch op {
	case ir.BinAdd:
		return value.Number(x + y), true
	case ir.BinSub:
		return value.Number(x - y), true
	case ir.BinMul:
		return value.Number(x * y), true
	case ir.BinDiv:
		return value.Number(x / y), true
	case ir.BinMod:
		return value.Number(math.Mod(x, y)), true
	// NaN が絡む比較はGoも false になるので、順序判定を分ける必要はない
	case ir.BinLt:
		return value.Bool(x < y), true
	case ir.BinLtEq:
		return value.Bool(x <= y), true
	case ir.BinGt:
		return value.Bool(x > y), true
	case ir.BinGtEq:
		return value.Bool(x >= y), true
	// 数値どうしなら、緩い比較も厳密比較も同じ「==」になる
	// (NaN != NaN、-0 == 0 はGoとJavaScriptで一致する)
	case ir.BinEq, ir.BinStrictEq:
		return value.Bool(x == y), true
	case ir.BinNotEq, ir.BinStrictNotEq:
		return value.Bool(x != y), true
	}
	return value.Undefined(), false
}

// binaryGeneral is the operator table as the interpreter has always had it.
func binaryGeneral(op ir.BinaryOp, a, b value.Value) value.Value {
	switch op {
	case ir.BinAdd:
		return value.Number(value.Add(value.ParseFloat(a), value.ParseFloat(b)))
	case ir.BinSub:
		return value.Number(value.Sub(a, b))
	case ir.BinMul:
		return value.Number(value.Mul(a, b))
	case ir.BinDiv:
		return value.Number(value.Div(a, b))
	case ir.BinIntDiv:
		return value.Number(value.IntDiv(a, b))
	case ir.BinMod:
		return value.Number(value.Mod(a, b))
	case ir.BinPow:
		return value.Number(value.Pow(a, b))
	case ir.BinConcat:
		return value.String(value.Concat(a, b))
	case ir.BinEq:
		return value.Bool(value.LooseEquals(a, b))
	case ir.BinNotEq:
		return value.Bool(!value.LooseEquals(a, b))
	case ir.BinStrictEq:
		return value.Bool(value.StrictEquals(a, b))
	case ir.BinStrictNotEq:
		return value.Bool(!value.StrictEquals(a, b))
	case ir.BinLt:
		return value.Bool(value.LessThan(a, b))
	case ir.BinLtEq:
		return value.Bool(value.LessEqual(a, b))
	case ir.BinGt:
		return value.Bool(value.GreaterThan(a, b))
	case ir.BinGtEq:
		return value.Bool(value.GreaterEqual(a, b))
	case ir.BinShiftL:
		return value.Number(value.ShiftLeft(a, b))
	case ir.BinShiftR:
		return value.Number(value.ShiftRight(a, b))
	case ir.BinShiftR0:
		return value.Number(value.ShiftRightZero(a, b))
	}
	return value.Undefined()
}

// Unary applies a unary operator, exactly as the interpreter's OpUnary does.
func Unary(op ir.UnaryOp, v value.Value) value.Value {
	switch op {
	case ir.UnaryNot:
		return value.Bool(!value.ToBool(v))
	case ir.UnaryNeg:
		return value.Number(-value.ToNumber(v))
	}
	return value.Undefined()
}
