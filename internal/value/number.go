package value

import (
	"math"
	"strconv"
	"strings"
)

// NumberToString converts a number to its string form using the ECMAScript
// Number::toString algorithm, so that 表示 and string concatenation produce the
// same text as the TypeScript implementation.
//
// The notable differences from Go's own formatting are that the exponent form
// kicks in only outside 1e-7..1e21, the exponent has no zero padding, and -0
// prints as "0".
func NumberToString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0" // JSは -0 も "0" になる
	}

	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}

	// 最短往復表現の桁と指数を取り出す。"d.dddde±XX" の形で返る。
	digits, exp10 := shortestDigits(f)
	k := len(digits) // 有効桁数
	n := exp10 + 1   // 値 = 0.digits × 10^n

	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}

	// 指数表記
	e := n - 1
	expSign := "+"
	if e < 0 {
		expSign = "-"
		e = -e
	}
	mantissa := digits
	if k > 1 {
		mantissa = digits[:1] + "." + digits[1:]
	}
	return sign + mantissa + "e" + expSign + strconv.Itoa(e)
}

// shortestDigits returns the shortest round-tripping decimal digits of f
// (without a decimal point) and the base-10 exponent of its first digit.
func shortestDigits(f float64) (digits string, exp10 int) {
	s := strconv.FormatFloat(f, 'e', -1, 64) // 例: "1.234e+05"
	mantissa, expPart, _ := strings.Cut(s, "e")
	exp10, _ = strconv.Atoi(expPart)
	return strings.Replace(mantissa, ".", "", 1), exp10
}
