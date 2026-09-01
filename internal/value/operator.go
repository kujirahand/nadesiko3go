package value

import (
	"math"
	"math/big"
	"strings"
)

// The operators here mirror what nako_gen.mts emits, which is mostly plain
// JavaScript operators. Two of them are not:
//
//   - 『+』 runs every operand that is not a numeric literal through parseFloat,
//     so it always adds numbers and never concatenates strings.
//   - 『&』 is the string concatenation operator (`left + "" + right`).

// Add implements 『+』. Both operands have already been through ParseFloat by
// the compiler, except literal numbers, which need no conversion.
func Add(a, b float64) float64 { return a + b }

// Sub, Mul, Div and Mod implement the plain JavaScript operators, which coerce
// their operands with ToNumber rather than parseFloat.
func Sub(a, b Value) float64 { return ToNumber(a) - ToNumber(b) }
func Mul(a, b Value) float64 { return ToNumber(a) * ToNumber(b) }
func Div(a, b Value) float64 { return ToNumber(a) / ToNumber(b) }

// Mod implements JavaScript's %, which keeps the sign of the dividend and is
// therefore math.Mod rather than a Euclidean remainder.
func Mod(a, b Value) float64 { return math.Mod(ToNumber(a), ToNumber(b)) }

// Pow implements 『^』 and 『**』.
func Pow(a, b Value) float64 { return jsPow(ToNumber(a), ToNumber(b)) }

// jsPow follows the ECMAScript exponentiation operator. math.Pow disagrees
// with it in the cases where a special-cased base wins over a NaN or infinite
// exponent: JavaScript lets the exponent decide first.
func jsPow(base, exp float64) float64 {
	switch {
	case math.IsNaN(exp):
		return math.NaN()
	case exp == 0:
		return 1 // 底がNaNでも1
	case math.IsNaN(base):
		return math.NaN()
	case math.IsInf(exp, 0) && (base == 1 || base == -1):
		return math.NaN()
	}
	if n, ok := halfIntegerExponent(exp); ok && base > 0 && !math.IsInf(base, 0) {
		return exactHalfPow(base, n)
	}
	return math.Pow(base, exp)
}

// intPowLimit bounds the exponent that goes through the exact path. Squaring
// past this overflows big.Float's exponent, and every result out there is
// already ±Inf or 0, which math.Pow gets right on its own.
const intPowLimit = 1 << 20

// halfIntegerExponent reports twice the exponent when the exponent is a whole
// number of halves, which is the case for every ordinary power and for the
// square roots written as 『x^0.5』.
func halfIntegerExponent(exp float64) (int64, bool) {
	twice := exp * 2
	if twice != math.Trunc(twice) || math.Abs(twice) > intPowLimit {
		return 0, false
	}
	return int64(twice), true
}

// exactHalfPow computes base^(twice/2) at high precision and rounds once at the
// end.
//
// math.Pow is not correctly rounded, so it can land one ulp away from the
// JavaScript result — 『3.14^12』 is the shortest example. Squaring in big.Float
// and rounding a single time removes that difference, and big.Float's Sqrt
// extends it to half-integer exponents.
//
// An exponent that is not a whole number of halves still goes through math.Pow
// and can differ from JavaScript by one ulp. Computing those exactly would need
// an arbitrary-precision exp and log, which the standard library does not have.
func exactHalfPow(base float64, twice int64) float64 {
	negative := twice < 0
	if negative {
		twice = -twice
	}
	const prec = 300
	acc := big.NewFloat(1).SetPrec(prec)
	b := big.NewFloat(base).SetPrec(prec)
	for n := twice; n > 0; n >>= 1 {
		if n&1 == 1 {
			acc.Mul(acc, b)
		}
		if n > 1 {
			b.Mul(b, b)
		}
	}
	acc.Sqrt(acc) // 指数を2倍して計算したぶんを戻す
	if negative {
		acc.Quo(big.NewFloat(1).SetPrec(prec), acc)
	}
	f, _ := acc.Float64()
	return f
}

// IntDiv implements 『÷÷』, the integer division operator (#1152).
func IntDiv(a, b Value) float64 { return math.Floor(ToNumber(a) / ToNumber(b)) }

// Concat implements 『&』. It is always a string concatenation.
func Concat(a, b Value) string { return ToString(a) + ToString(b) }

// ShiftLeft, ShiftRight and ShiftRightZero implement <<, >> and >>>.
// The shift count is taken modulo 32, as in JavaScript.
func ShiftLeft(a, b Value) float64  { return float64(ToInt32(a) << (ToUint32(b) & 31)) }
func ShiftRight(a, b Value) float64 { return float64(ToInt32(a) >> (ToUint32(b) & 31)) }
func ShiftRightZero(a, b Value) float64 {
	return float64(ToUint32(a) >> (ToUint32(b) & 31))
}

// BitAnd, BitOr and BitXor implement the bitwise operators behind the
// 『AND』『OR』『XOR』 commands.
func BitAnd(a, b Value) float64 { return float64(ToInt32(a) & ToInt32(b)) }
func BitOr(a, b Value) float64  { return float64(ToInt32(a) | ToInt32(b)) }
func BitXor(a, b Value) float64 { return float64(ToInt32(a) ^ ToInt32(b)) }

// StrictEquals implements === : same type and same value, with NaN equal to
// nothing and the two zeros equal to each other.
func StrictEquals(a, b Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case KindUndefined, KindNull:
		return true
	case KindBool:
		x, _ := a.Bool()
		y, _ := b.Bool()
		return x == y
	case KindNumber:
		x, _ := a.Number()
		y, _ := b.Number()
		return x == y // NaN != NaN, -0 == 0 はGoの比較と同じ
	case KindString:
		x, _ := a.String()
		y, _ := b.String()
		return x == y
	case KindArray:
		x, _ := a.Array()
		y, _ := b.Array()
		return x == y // 参照の同一性
	case KindDict:
		x, _ := a.Dict()
		y, _ := b.Dict()
		return x == y
	case KindFunc:
		x, _ := a.Func()
		y, _ := b.Func()
		return x == y
	}
	return false
}

// LooseEquals implements == , which coerces across types. It is what 『=』 and
// 『≠』 compile to, so 『0="0"』 and 『オン=1』 are both true.
func LooseEquals(a, b Value) bool {
	ak, bk := a.Kind(), b.Kind()

	if ak == bk {
		return StrictEquals(a, b)
	}
	// null と undefined は互いにだけ等しい
	nullish := func(k Kind) bool { return k == KindNull || k == KindUndefined }
	if nullish(ak) || nullish(bk) {
		return nullish(ak) && nullish(bk)
	}
	// 真偽値は数値に直してから比べ直す
	if ak == KindBool {
		return LooseEquals(Number(ToNumber(a)), b)
	}
	if bk == KindBool {
		return LooseEquals(a, Number(ToNumber(b)))
	}
	// オブジェクトはプリミティブに直してから比べ直す
	if isObject(ak) {
		return LooseEquals(ToPrimitive(a), b)
	}
	if isObject(bk) {
		return LooseEquals(a, ToPrimitive(b))
	}
	// 残るのは数値と文字列の組み合わせ
	if (ak == KindNumber && bk == KindString) || (ak == KindString && bk == KindNumber) {
		return ToNumber(a) == ToNumber(b)
	}
	return false
}

func isObject(k Kind) bool {
	return k == KindArray || k == KindDict || k == KindFunc
}

// Compare implements the relational operators. ordered is false when either
// side is NaN, in which case every one of <, >, <= and >= is false.
//
// Two strings compare by code point rather than by UTF-16 code unit. The two
// orders differ only for characters outside the BMP, which is the same
// deliberate difference the string commands carry.
func Compare(a, b Value) (result int, ordered bool) {
	pa, pb := ToPrimitive(a), ToPrimitive(b)
	if pa.Kind() == KindString && pb.Kind() == KindString {
		x, _ := pa.String()
		y, _ := pb.String()
		return strings.Compare(x, y), true
	}
	x, y := ToNumber(pa), ToNumber(pb)
	if math.IsNaN(x) || math.IsNaN(y) {
		return 0, false
	}
	switch {
	case x < y:
		return -1, true
	case x > y:
		return 1, true
	}
	return 0, true
}

// LessThan, GreaterThan, LessEqual and GreaterEqual implement <, >, <= and >=.
func LessThan(a, b Value) bool {
	c, ok := Compare(a, b)
	return ok && c < 0
}

func GreaterThan(a, b Value) bool {
	c, ok := Compare(a, b)
	return ok && c > 0
}

func LessEqual(a, b Value) bool {
	c, ok := Compare(a, b)
	return ok && c <= 0
}

func GreaterEqual(a, b Value) bool {
	c, ok := Compare(a, b)
	return ok && c >= 0
}
