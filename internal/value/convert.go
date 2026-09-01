package value

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// The conversions here follow the ECMAScript abstract operations, because the
// TypeScript implementation compiles nadesiko expressions into JavaScript and
// inherits its coercion rules wholesale. The compat fixtures pin the results.

// ToString converts a value to a string, the way JavaScript's String() does.
func ToString(v Value) string {
	switch v.Kind() {
	case KindUndefined:
		return "undefined"
	case KindNull:
		return "null"
	case KindBool:
		if b, _ := v.Bool(); b {
			return "true"
		}
		return "false"
	case KindNumber:
		n, _ := v.Number()
		return NumberToString(n)
	case KindString:
		s, _ := v.String()
		return s
	case KindArray:
		arr, _ := v.Array()
		return arrayToString(arr)
	case KindDict:
		return "[object Object]"
	case KindFunc:
		return "function"
	}
	return ""
}

// arrayToString joins the elements with commas. JavaScript renders a hole,
// undefined, or null as an empty string rather than as its own name.
func arrayToString(a *Array) string {
	if a == nil {
		return ""
	}
	parts := make([]string, 0, a.Len())
	for i := 0; i < a.Len(); i++ {
		item := a.Get(i)
		switch item.Kind() {
		case KindUndefined, KindNull:
			parts = append(parts, "")
		default:
			parts = append(parts, ToString(item))
		}
	}
	return strings.Join(parts, ",")
}

// ToPrimitive reduces an array or dictionary to the primitive JavaScript would
// use in an arithmetic or comparison context. Other values pass through.
func ToPrimitive(v Value) Value {
	switch v.Kind() {
	case KindArray, KindDict, KindFunc:
		return String(ToString(v))
	}
	return v
}

// ToNumber converts a value to a number, the way JavaScript's Number() does.
func ToNumber(v Value) float64 {
	switch v.Kind() {
	case KindUndefined:
		return math.NaN()
	case KindNull:
		return 0
	case KindBool:
		if b, _ := v.Bool(); b {
			return 1
		}
		return 0
	case KindNumber:
		n, _ := v.Number()
		return n
	case KindString:
		s, _ := v.String()
		return stringToNumber(s)
	case KindArray, KindDict, KindFunc:
		return stringToNumber(ToString(v))
	}
	return math.NaN()
}

// decimalNumberRE matches the decimal literals Number() accepts. Go's
// ParseFloat is more permissive than JavaScript — it takes "Inf", "inf",
// "NaN" and hexadecimal floats like "0x1p4" — so the input is checked
// against this first.
var decimalNumberRE = regexp.MustCompile(`^(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// stringToNumber implements JavaScript's string-to-number conversion, which
// accepts a whole trimmed numeric literal and nothing else. An empty or
// blank string is 0, unlike ParseFloat.
func stringToNumber(s string) float64 {
	s = strings.TrimFunc(s, isJSSpace)
	if s == "" {
		return 0
	}
	// 基数の接頭辞が付く形。符号は付けられない。
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			n, err := strconv.ParseUint(s[2:], base, 64)
			if err != nil {
				return math.NaN()
			}
			return float64(n)
		}
	}
	body := s
	sign := 1.0
	switch {
	case strings.HasPrefix(body, "+"):
		body = body[1:]
	case strings.HasPrefix(body, "-"):
		sign, body = -1, body[1:]
	}
	if body == "Infinity" {
		return sign * math.Inf(1)
	}
	if !decimalNumberRE.MatchString(body) {
		return math.NaN()
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return n
}

// ParseFloat implements JavaScript's global parseFloat, which reads as much of
// a leading decimal number as it can and ignores the rest.
//
// This is what nadesiko's 『+』 uses on every operand that is not a numeric
// literal, so 『1+"2"』 is 3 and 『オン+オン』 is NaN.
func ParseFloat(v Value) float64 {
	if v.Kind() == KindNumber {
		n, _ := v.Number()
		return n
	}
	s := strings.TrimLeftFunc(ToString(v), isJSSpace)

	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if strings.HasPrefix(s[i:], "Infinity") {
		if strings.HasPrefix(s, "-") {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return math.NaN()
	}
	// 指数部は、e の後に数字が続くときだけ取り込む
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		expDigits := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			expDigits++
		}
		if expDigits > 0 {
			i = j
		}
	}
	n, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return math.NaN()
	}
	return n
}

// ParseInt implements JavaScript's global parseInt without an explicit radix.
// It accepts a hexadecimal 0x prefix and otherwise consumes the leading decimal
// digits, stopping at the first character that is not valid for that radix.
func ParseInt(v Value) float64 {
	s := strings.TrimLeftFunc(ToString(v), isJSSpace)
	sign := 1.0
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}

	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	}
	i := 0
	for i < len(s) {
		c := s[i]
		valid := c >= '0' && c <= '9'
		if base == 16 {
			valid = valid || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
		}
		if !valid {
			break
		}
		i++
	}
	if i == 0 {
		return math.NaN()
	}
	if base == 10 {
		n, err := strconv.ParseFloat(s[:i], 64)
		if err != nil && !math.IsInf(n, 0) {
			return math.NaN()
		}
		return sign * n
	}
	integer, ok := new(big.Int).SetString(s[:i], base)
	if !ok {
		return math.NaN()
	}
	n, _ := new(big.Float).SetInt(integer).Float64()
	return sign * n
}

// isJSSpace reports whether r is whitespace for the purposes of parseFloat and
// Number(). JavaScript counts the Unicode space separators, the line
// terminators, and the byte order mark.
func isJSSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x00A0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// ToBool converts a value to a boolean, the way JavaScript's truthiness does.
// Every array and dictionary is truthy, including an empty one.
func ToBool(v Value) bool {
	switch v.Kind() {
	case KindUndefined, KindNull:
		return false
	case KindBool:
		b, _ := v.Bool()
		return b
	case KindNumber:
		n, _ := v.Number()
		return n != 0 && !math.IsNaN(n)
	case KindString:
		s, _ := v.String()
		return s != ""
	}
	return true
}

// ToInt32 converts a value to a signed 32-bit integer, as the bitwise
// operators do in JavaScript.
func ToInt32(v Value) int32 {
	return int32(toUint32Bits(ToNumber(v)))
}

// ToUint32 converts a value to an unsigned 32-bit integer, as the unsigned
// right shift does.
func ToUint32(v Value) uint32 {
	return toUint32Bits(ToNumber(v))
}

func toUint32Bits(n float64) uint32 {
	if math.IsNaN(n) || math.IsInf(n, 0) || n == 0 {
		return 0
	}
	n = math.Trunc(n)
	m := math.Mod(n, 4294967296)
	if m < 0 {
		m += 4294967296
	}
	return uint32(m)
}

// TypeName reports the value's type the way JavaScript's typeof does, which is
// what 『変数型確認』 returns.
func TypeName(v Value) string {
	switch v.Kind() {
	case KindUndefined:
		return "undefined"
	case KindBool:
		return "boolean"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindFunc:
		return "function"
	}
	return "object" // null も配列も辞書も object
}
