package ops

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// numbers covers the float64 cases where a shortcut is most likely to be
// wrong: NaN and the infinities (comparisons must all be false), the two
// zeros (-0 == 0 but 1/-0 != 1/0), the edge of exact integers, and ordinary
// values.
var numbers = []float64{
	0, math.Copysign(0, -1), 1, -1, 2, 0.5, -0.5,
	3, 7, 10, 0.1, 0.2, 1e21, -1e21, math.MaxFloat64, math.SmallestNonzeroFloat64,
	9007199254740992, 9007199254740993, // 2^53 とその隣（float64では同じ値になる）
	math.Inf(1), math.Inf(-1), math.NaN(),
}

var allBinaryOps = []ir.BinaryOp{
	ir.BinAdd, ir.BinSub, ir.BinMul, ir.BinDiv, ir.BinIntDiv, ir.BinMod,
	ir.BinPow, ir.BinConcat, ir.BinEq, ir.BinNotEq, ir.BinStrictEq,
	ir.BinStrictNotEq, ir.BinLt, ir.BinLtEq, ir.BinGt, ir.BinGtEq,
	ir.BinShiftL, ir.BinShiftR, ir.BinShiftR0,
}

// TestNumberFastPathMatchesGeneral is what makes the fast path safe to have:
// for every operator and every pair of representative numbers, taking the
// shortcut must produce bit-for-bit what the general path produces. A future
// edit to either side that breaks that shows up here rather than as a
// mis-computed program.
func TestNumberFastPathMatchesGeneral(t *testing.T) {
	for _, op := range allBinaryOps {
		for _, x := range numbers {
			for _, y := range numbers {
				fast, done := binaryNumbers(op, x, y)
				if !done {
					continue // この演算子は一般経路に任せている
				}
				slow := binaryGeneral(op, value.Number(x), value.Number(y))
				if !identical(fast, slow) {
					t.Fatalf("%v(%v, %v): 高速経路=%s 一般経路=%s",
						op, x, y, describe(fast), describe(slow))
				}
			}
		}
	}
}

// TestBinaryUsesGeneralPathForNonNumbers pins that anything other than two
// numbers still goes through the general path — the fast path must never
// change what 『1+"2"』 or 『"a"&1』 do.
func TestBinaryUsesGeneralPathForNonNumbers(t *testing.T) {
	cases := []struct {
		name string
		op   ir.BinaryOp
		a, b value.Value
		want string
	}{
		{"数値と文字列の加算", ir.BinAdd, value.Number(1), value.String("2"), "num:3"},
		{"数値と文字列の連結", ir.BinConcat, value.Number(1), value.String("a"), "str:1a"},
		{"文字列どうしの比較", ir.BinLt, value.String("a"), value.String("b"), "bool:true"},
		{"真偽値の緩い比較", ir.BinEq, value.Bool(true), value.Number(1), "bool:true"},
		{"空と未定義の緩い比較", ir.BinEq, value.Null(), value.Undefined(), "bool:true"},
		{"空と未定義の厳密比較", ir.BinStrictEq, value.Null(), value.Undefined(), "bool:false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(Binary(c.op, c.a, c.b)); got != c.want {
				t.Fatalf("Binary = %s, want %s", got, c.want)
			}
		})
	}
}

// identical compares two values the way the fixtures do: same kind, and for
// numbers the same bits, so that NaN and -0 are not waved through.
func identical(a, b value.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	if a.Kind() == value.KindNumber {
		x, _ := a.Number()
		y, _ := b.Number()
		return math.Float64bits(x) == math.Float64bits(y)
	}
	return describe(a) == describe(b)
}

func describe(v value.Value) string {
	switch v.Kind() {
	case value.KindNumber:
		n, _ := v.Number()
		return "num:" + value.ToString(value.Number(n))
	case value.KindBool:
		return "bool:" + value.ToString(v)
	case value.KindString:
		s, _ := v.String()
		return "str:" + s
	}
	return "other:" + value.ToString(v)
}
