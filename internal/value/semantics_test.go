package value_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Expected values come from evaluating the same expression in JavaScript. Every
// operation below was also checked against all 1521 ordered pairs of a
// 39-value sample during development; the only remaining difference is the one
// TestPowPrecision documents.

func arr(items ...value.Value) value.Value { return value.ArrayValue(value.NewArray(items...)) }

func dict(pairs ...any) value.Value {
	d := value.NewDict()
	for i := 0; i+1 < len(pairs); i += 2 {
		d.Set(pairs[i].(string), pairs[i+1].(value.Value))
	}
	return value.DictValue(d)
}

func TestToString(t *testing.T) {
	tests := []struct {
		in   value.Value
		want string
	}{
		{value.Undefined(), "undefined"},
		{value.Null(), "null"},
		{value.Bool(true), "true"},
		{value.Bool(false), "false"},
		{value.Number(3), "3"},
		{value.Number(3.1), "3.1"},
		{value.Number(math.NaN()), "NaN"},
		{value.String("abc"), "abc"},
		{arr(), ""},
		{arr(value.Number(1), value.Number(2), value.Number(3)), "1,2,3"},
		// 入れ子の配列は平坦に見える
		{arr(arr(value.Number(1), value.Number(2)), arr(value.Number(3), value.Number(4))), "1,2,3,4"},
		// 穴・undefined・null は空文字列になる
		{arr(value.Number(1), value.Undefined(), value.Null(), value.Number(2)), "1,,,2"},
		{dict(), "[object Object]"},
		{dict("x", value.Number(1)), "[object Object]"},
	}
	for _, tt := range tests {
		if got := value.ToString(tt.in); got != tt.want {
			t.Errorf("ToString(%v) = %q, want %q", tt.in.Kind(), got, tt.want)
		}
	}
}

func TestToNumber(t *testing.T) {
	tests := []struct {
		in   value.Value
		want float64
	}{
		{value.Undefined(), math.NaN()},
		{value.Null(), 0},
		{value.Bool(true), 1},
		{value.Bool(false), 0},
		{value.String(""), 0},
		{value.String(" "), 0},
		{value.String("  12  "), 12},
		{value.String("12.5"), 12.5},
		{value.String("0x10"), 16},
		{value.String("Infinity"), math.Inf(1)},
		{value.String("-Infinity"), math.Inf(-1)},
		// JSが受け付けない綴り。Goのstrconvは受け付けてしまうので弾いている。
		{value.String("inf"), math.NaN()},
		{value.String("1_000"), math.NaN()},
		{value.String("12abc"), math.NaN()},
		{value.String("あ"), math.NaN()},
		{arr(), 0},
		{arr(value.Number(5)), 5},
		{arr(value.Number(1), value.Number(2)), math.NaN()},
		{dict(), math.NaN()},
	}
	for _, tt := range tests {
		got := value.ToNumber(tt.in)
		if !sameNumber(got, tt.want) {
			t.Errorf("ToNumber(%s) = %v, want %v", value.ToString(tt.in), got, tt.want)
		}
	}
}

// TestParseFloat pins the conversion 『+』 uses. Unlike Number() it reads a
// leading number and ignores the rest, and an empty string is NaN rather than 0.
func TestParseFloat(t *testing.T) {
	tests := []struct {
		in   value.Value
		want float64
	}{
		{value.String(""), math.NaN()},
		{value.String("12abc"), 12},
		{value.String("  3.5e2xyz"), 350},
		{value.String(".5"), 0.5},
		{value.String("1e"), 1}, // 指数部に数字がないので e は読み捨てる
		{value.String("0x10"), 0},
		{value.String("Infinity"), math.Inf(1)},
		{value.String("-Infinity"), math.Inf(-1)},
		{value.String("あ"), math.NaN()},
		{value.Bool(true), math.NaN()}, // 『オン+オン』がNaNになる理由
		{value.Null(), math.NaN()},
		{value.Undefined(), math.NaN()},
		{arr(value.Number(1), value.Number(2)), 1}, // "1,2" の先頭だけ読む
		{value.Number(3.5), 3.5},
	}
	for _, tt := range tests {
		got := value.ParseFloat(tt.in)
		if !sameNumber(got, tt.want) {
			t.Errorf("ParseFloat(%s) = %v, want %v", value.ToString(tt.in), got, tt.want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	num := value.Number
	str := value.String
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		// 『+』は常に数値。文字列連結にはならない。
		{"1+\"2\"", value.Add(value.ParseFloat(num(1)), value.ParseFloat(str("2"))), 3},
		{"\"1\"+\"2\"", value.Add(value.ParseFloat(str("1")), value.ParseFloat(str("2"))), 3},
		{"1+\"あ\"", value.Add(value.ParseFloat(num(1)), value.ParseFloat(str("あ"))), math.NaN()},
		{"オン+オン", value.Add(value.ParseFloat(value.Bool(true)), value.ParseFloat(value.Bool(true))), math.NaN()},
		// 『-』『*』『/』『%』は素のJS演算子なので ToNumber で変換する
		{"\"5\"-\"2\"", value.Sub(str("5"), str("2")), 3},
		{"\"5\"*\"2\"", value.Mul(str("5"), str("2")), 10},
		{"7/2", value.Div(num(7), num(2)), 3.5},
		{"1/0", value.Div(num(1), num(0)), math.Inf(1)},
		{"0/0", value.Div(num(0), num(0)), math.NaN()},
		{"7%3", value.Mod(num(7), num(3)), 1},
		{"-7%3", value.Mod(num(-7), num(3)), -1}, // 符号は被除数に従う
		{"2^10", value.Pow(num(2), num(10)), 1024},
		{"7÷÷2", value.IntDiv(num(7), num(2)), 3},
	}
	for _, tt := range tests {
		if !sameNumber(tt.got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

// TestPowPrecision pins how far the exponentiation operator matches
// JavaScript. math.Pow is not correctly rounded, so integer and half-integer
// exponents take an exact big.Float path instead.
func TestPowPrecision(t *testing.T) {
	tests := []struct {
		base, exp float64
		want      string
	}{
		{2, 10, "1024"},
		{3.14, 12, "918662.0518429504"},
		{1e21, 12, "1e+252"},
		{1e-7, 16, "9.999999999999993e-113"},
		{3.14, 12.5, "1627873.303318898"},
		{2, 0.5, "1.4142135623730951"},
		{2, -2, "0.25"},
		// 指数が半整数でない場合は math.Pow に任せるので、JSと1ulpずれうる。
		// 差分fixtureには現れないケース。
	}
	for _, tt := range tests {
		got := value.NumberToString(value.Pow(value.Number(tt.base), value.Number(tt.exp)))
		if got != tt.want {
			t.Errorf("%v^%v = %s, want %s", tt.base, tt.exp, got, tt.want)
		}
	}
}

func TestPowSpecialCases(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		base, exp, want float64
	}{
		{1, nan, nan}, // 指数がNaNなら底に関わらずNaN
		{nan, 0, 1},   // 指数が0なら底に関わらず1
		{nan, 1, nan},
		{1, inf, nan},
		{-1, inf, nan},
		{2, inf, inf},
	}
	for _, tt := range tests {
		got := value.Pow(value.Number(tt.base), value.Number(tt.exp))
		if !sameNumber(got, tt.want) {
			t.Errorf("%v^%v = %v, want %v", tt.base, tt.exp, got, tt.want)
		}
	}
}

func TestConcatAndBitwise(t *testing.T) {
	num, str := value.Number, value.String
	if got := value.Concat(str("a"), str("b")); got != "ab" {
		t.Errorf("\"a\"&\"b\" = %q, want \"ab\"", got)
	}
	if got := value.Concat(str("a"), num(1)); got != "a1" {
		t.Errorf("\"a\"&1 = %q, want \"a1\"", got)
	}
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"12 AND 10", value.BitAnd(num(12), num(10)), 8},
		{"12 OR 10", value.BitOr(num(12), num(10)), 14},
		{"12 XOR 10", value.BitXor(num(12), num(10)), 6},
		{"1<<4", value.ShiftLeft(num(1), num(4)), 16},
		{"16>>2", value.ShiftRight(num(16), num(2)), 4},
		{"-1>>>28", value.ShiftRightZero(num(-1), num(28)), 15},
		// シフト量は32で剰余を取る
		{"1<<32", value.ShiftLeft(num(1), num(32)), 1},
		// 32bitに丸めてから演算する
		{"NaN AND 1", value.BitAnd(num(math.NaN()), num(1)), 0},
	}
	for _, tt := range tests {
		if !sameNumber(tt.got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestEqualityAndComparison(t *testing.T) {
	num, str, b := value.Number, value.String, value.Bool
	eq := []struct {
		name string
		got  bool
		want bool
	}{
		{"0=\"0\"", value.LooseEquals(num(0), str("0")), true},
		{"0=\"\"", value.LooseEquals(num(0), str("")), true},
		{"オン=1", value.LooseEquals(b(true), num(1)), true},
		{"null=undefined", value.LooseEquals(value.Null(), value.Undefined()), true},
		{"null=0", value.LooseEquals(value.Null(), num(0)), false},
		{"NaN=NaN", value.LooseEquals(num(math.NaN()), num(math.NaN())), false},
		{"\"abc\"=\"abc\"", value.LooseEquals(str("abc"), str("abc")), true},
		{"[1]=1", value.LooseEquals(arr(num(1)), num(1)), true},
		{"0===\"0\"", value.StrictEquals(num(0), str("0")), false},
		{"1===1", value.StrictEquals(num(1), num(1)), true},
	}
	for _, tt := range eq {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}

	cmp := []struct {
		name string
		got  bool
		want bool
	}{
		{"1<2", value.LessThan(num(1), num(2)), true},
		{"2<=2", value.LessEqual(num(2), num(2)), true},
		{"3>2", value.GreaterThan(num(3), num(2)), true},
		{"3>=3", value.GreaterEqual(num(3), num(3)), true},
		{"\"a\"<\"b\"", value.LessThan(str("a"), str("b")), true},
		// NaNが絡むと4つとも偽になる
		{"NaN<1", value.LessThan(num(math.NaN()), num(1)), false},
		{"NaN>=1", value.GreaterEqual(num(math.NaN()), num(1)), false},
		// 片方が文字列でないなら数値として比べる
		{"\"10\"<9", value.LessThan(str("10"), num(9)), false},
	}
	for _, tt := range cmp {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestToBoolAndTypeName(t *testing.T) {
	tests := []struct {
		in       value.Value
		want     bool
		typeName string
	}{
		{value.Undefined(), false, "undefined"},
		{value.Null(), false, "object"},
		{value.Bool(false), false, "boolean"},
		{value.Number(0), false, "number"},
		{value.Number(math.NaN()), false, "number"},
		{value.Number(1), true, "number"},
		{value.String(""), false, "string"},
		{value.String("a"), true, "string"},
		{arr(), true, "object"},  // 空配列も真
		{dict(), true, "object"}, // 空辞書も真
	}
	for _, tt := range tests {
		if got := value.ToBool(tt.in); got != tt.want {
			t.Errorf("ToBool(%s) = %v, want %v", value.ToString(tt.in), got, tt.want)
		}
		if got := value.TypeName(tt.in); got != tt.typeName {
			t.Errorf("TypeName(%s) = %q, want %q", value.ToString(tt.in), got, tt.typeName)
		}
	}
}

// sameNumber treats NaN as equal to NaN, which the tests need but == does not.
func sameNumber(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}
