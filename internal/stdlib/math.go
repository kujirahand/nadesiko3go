package stdlib

import (
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// mathImpls implements plugin_system_math. Keep the coercions here aligned
// with JavaScript because plugin_system compatibility includes those details.
func mathImpls(m map[string]Impl) {
	m["引"] = binaryNumber(func(a, b value.Value) float64 { return value.ToNumber(a) - value.ToNumber(b) })
	m["割"] = binaryNumber(func(a, b value.Value) float64 { return value.ToNumber(a) / value.ToNumber(b) })
	m["割余"] = binaryNumber(func(a, b value.Value) float64 { return math.Mod(value.ToNumber(a), value.ToNumber(b)) })
	m["倍"] = binaryNumber(value.Mul)
	m["掛"] = func(_ Context, a []value.Value) (value.Value, error) {
		left, count := arg(a, 0), int(value.ToNumber(arg(a, 1)))
		switch left.Kind() {
		case value.KindString:
			return value.String(strings.Repeat(value.ToString(left), max(count, 0))), nil
		case value.KindArray:
			src, _ := left.Array()
			out := value.NewArray()
			for n := 0; n < count; n++ {
				for i := 0; i < src.Len(); i++ {
					out.Set(out.Len(), src.Get(i))
				}
			}
			return value.ArrayValue(out), nil
		default:
			return value.Number(value.Mul(left, arg(a, 1))), nil
		}
	}
	m["偶数"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Bool(int64(value.ToNumber(arg(a, 0)))%2 == 0), nil
	}
	m["奇数"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Bool(int64(value.ToNumber(arg(a, 0)))%2 != 0), nil
	}
	m["二乗"] = func(_ Context, a []value.Value) (value.Value, error) {
		n := value.ToNumber(arg(a, 0))
		return value.Number(n * n), nil
	}
	m["べき乗"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Number(math.Pow(value.ToNumber(arg(a, 0)), value.ToNumber(arg(a, 1)))), nil
	}
	m["以上"] = compare(func(c int) bool { return c >= 0 })
	m["以下"] = compare(func(c int) bool { return c <= 0 })
	m["未満"] = compare(func(c int) bool { return c < 0 })
	m["超"] = compare(func(c int) bool { return c > 0 })
	m["等"] = equal(false)
	m["一致"] = matched(false)
	m["等無"] = equal(true)
	m["不一致"] = matched(true)
	m["範囲内"] = func(_ Context, a []value.Value) (value.Value, error) {
		low, ok1 := value.Compare(arg(a, 1), arg(a, 0))
		high, ok2 := value.Compare(arg(a, 0), arg(a, 2))
		return value.Bool(ok1 && ok2 && low <= 0 && high <= 0), nil
	}
	m["合計"] = sumValues
	m["連続加算"] = sumValues
	m["MAX"] = extrema(true)
	m["最大値"] = m["MAX"]
	m["MIN"] = extrema(false)
	m["最小値"] = m["MIN"]
	m["CLAMP"] = func(_ Context, a []value.Value) (value.Value, error) {
		x, low, high := value.ToNumber(arg(a, 0)), value.ToNumber(arg(a, 1)), value.ToNumber(arg(a, 2))
		return value.Number(math.Min(math.Max(x, low), high)), nil
	}
	m["論理OR"] = func(_ Context, a []value.Value) (value.Value, error) {
		if value.ToBool(arg(a, 0)) {
			return arg(a, 0), nil
		}
		return arg(a, 1), nil
	}
	m["論理AND"] = func(_ Context, a []value.Value) (value.Value, error) {
		if !value.ToBool(arg(a, 0)) {
			return arg(a, 0), nil
		}
		return arg(a, 1), nil
	}
	m["論理NOT"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Bool(!value.ToBool(arg(a, 0))), nil
	}
	m["NOT"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Number(float64(^value.ToInt32(arg(a, 0)))), nil
	}
	m["SHIFT_L"] = signedShift(func(v int32, n uint) int32 { return v << n })
	m["SHIFT_R"] = signedShift(func(v int32, n uint) int32 { return v >> n })
	m["SHIFT_UR"] = func(_ Context, a []value.Value) (value.Value, error) {
		v := uint32(value.ToInt32(arg(a, 0)))
		n := uint(value.ToInt32(arg(a, 1))) & 31
		return value.Number(float64(v >> n)), nil
	}
}

func compare(test func(int) bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		c, ok := value.Compare(arg(a, 0), arg(a, 1))
		return value.Bool(ok && test(c)), nil
	}
}

// equal は『等』『等無』の実装。本家の a === b / a !== b そのもので、
// 配列・辞書・関数については参照が同じときだけ等しい (#346)。
// 内容まで比べるのは『一致』『不一致』の役割で、ここへ混ぜない。
func equal(negate bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		eq := value.StrictEquals(arg(a, 0), arg(a, 1))
		return value.Bool(eq != negate), nil
	}
}

// matched は『一致』『不一致』の実装。本家と同じく、両辺が
// JavaScript の typeof で "object" になる値のときだけ内容を比べ、
// それ以外は厳密比較にする。
func matched(negate bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		eq, err := matchEquals(arg(a, 0), arg(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		return value.Bool(eq != negate), nil
	}
}

// jsObject は JavaScript の typeof v === "object" に当たる Kind かを返す。
// null (『NULL』定数) も "object" だが、関数 (KindFunc) は "function" で別扱い。
func jsObject(v value.Value) bool {
	switch v.Kind() {
	case value.KindNull, value.KindArray, value.KindDict:
		return true
	}
	return false
}

func matchEquals(left, right value.Value) (bool, error) {
	if !jsObject(left) || !jsObject(right) {
		return value.StrictEquals(left, right), nil
	}
	// 内容の比較は JSON 表現の一致で決まる (本家の JSON.stringify 比較)。
	// 循環参照は JSON.stringify が例外にするのと同じくエラーとして返し、
	// 『不一致』だけが真になるようなごまかしはしない。
	leftJSON, err := encodeJSON(left)
	if err != nil {
		return false, err
	}
	rightJSON, err := encodeJSON(right)
	if err != nil {
		return false, err
	}
	return leftJSON == rightJSON, nil
}

func sumValues(_ Context, a []value.Value) (value.Value, error) {
	if len(a) == 1 {
		if arr, ok := a[0].Array(); ok {
			total := 0.0
			for i := 0; i < arr.Len(); i++ {
				if n := value.ParseFloat(arr.Get(i)); !math.IsNaN(n) {
					total += n
				}
			}
			return value.Number(total), nil
		}
	}
	total := 0.0
	for _, v := range a {
		total += value.ParseFloat(v)
	}
	return value.Number(total), nil
}

func extrema(maximum bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		if len(a) == 0 {
			return value.Number(math.NaN()), nil
		}
		best := value.ToNumber(a[0])
		for _, v := range a[1:] {
			if n := value.ToNumber(v); (maximum && n > best) || (!maximum && n < best) {
				best = n
			}
		}
		return value.Number(best), nil
	}
}

func signedShift(op func(int32, uint) int32) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		v := value.ToInt32(arg(a, 0))
		n := uint(value.ToInt32(arg(a, 1))) & 31
		return value.Number(float64(op(v, n))), nil
	}
}
