package gogen

import (
	"math"
	"strconv"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// TestConstExprRoundTrips pins that a constant written into generated source
// reads back as exactly the same value. A literal that loses a bit here would
// not fail to compile — it would quietly compute something else.
func TestConstExprRoundTrips(t *testing.T) {
	nums := []float64{
		0, math.Copysign(0, -1), 1, -1, 2, 5000000, 0.1, 0.2, 0.30000000000000004,
		1.0 / 3.0, 1e15, 1e16, 1e21, -1e21, 9007199254740992, 9007199254740993,
		math.MaxFloat64, math.SmallestNonzeroFloat64, 3.141592653589793,
	}
	g := &generator{prog: &ir.Program{}, types: analyze(&ir.Program{})}
	for _, n := range nums {
		g.prog.Consts = []ir.Const{{Kind: ir.ConstNumber, Num: n}}
		expr := g.constExpr(0)
		lit, ok := numberLiteralOf(expr)
		if !ok {
			t.Fatalf("%v: 数値リテラルとして書き出されなかった: %s", n, expr)
		}
		back, err := strconv.ParseFloat(lit, 64)
		if err != nil {
			t.Fatalf("%v: 生成した %q をGoが読めない: %v", n, lit, err)
		}
		if math.Float64bits(back) != math.Float64bits(n) {
			t.Fatalf("%v: %q として書き出され、%v として読み戻された", n, lit, back)
		}
	}
}

// TestConstExprFallsBackForNonFinite pins that NaN and the infinities keep
// going through the constant pool: Go has no literal for them, and silently
// writing something else would change the program.
func TestConstExprFallsBackForNonFinite(t *testing.T) {
	g := &generator{prog: &ir.Program{}, types: analyze(&ir.Program{})}
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		g.prog.Consts = []ir.Const{{Kind: ir.ConstNumber, Num: n}}
		if got := g.constExpr(0); got != "m.ConstValue(0)" {
			t.Fatalf("%v: %s（定数プール経由になっていない）", n, got)
		}
	}
}

// TestConstExprKinds covers the other constant kinds, including a string with
// characters that must survive being quoted into Go source.
func TestConstExprKinds(t *testing.T) {
	cases := []struct {
		k    ir.Const
		want string
	}{
		{ir.Const{Kind: ir.ConstUndefined}, "rt.Undefined()"},
		{ir.Const{Kind: ir.ConstNull}, "rt.Null()"},
		{ir.Const{Kind: ir.ConstBool, Bool: true}, "rt.Bool(true)"},
		{ir.Const{Kind: ir.ConstBool, Bool: false}, "rt.Bool(false)"},
		{ir.Const{Kind: ir.ConstString, Str: "こんにちは"}, `rt.String("こんにちは")`},
		{ir.Const{Kind: ir.ConstString, Str: "\"引用符\"と\\と改行\n"}, `rt.String("\"引用符\"と\\と改行\n")`},
		{ir.Const{Kind: ir.ConstString, Str: "バッククォート`も"}, "rt.String(\"バッククォート`も\")"},
		{ir.Const{Kind: ir.ConstString, Str: "𩸽"}, `rt.String("𩸽")`},
	}
	g := &generator{prog: &ir.Program{}, types: analyze(&ir.Program{})}
	for _, c := range cases {
		g.prog.Consts = []ir.Const{c.k}
		if got := g.constExpr(0); got != c.want {
			t.Errorf("constExpr = %s, want %s", got, c.want)
		}
	}
}

// numberLiteralOf pulls the literal out of `rt.Number(...)`.
func numberLiteralOf(expr string) (string, bool) {
	const prefix, suffix = "rt.Number(", ")"
	if len(expr) < len(prefix)+len(suffix) || expr[:len(prefix)] != prefix {
		return "", false
	}
	return expr[len(prefix) : len(expr)-len(suffix)], true
}
