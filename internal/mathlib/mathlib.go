// Package mathlib implements commands from the separate TypeScript plugin_math.
package mathlib

import (
	"math"
	"math/rand/v2"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	add := func(name string, josi [][]string) {
		list[name] = &lexer.FuncItem{Name: name, Type: "func", Josi: josi, Pure: true}
	}
	for _, name := range []string{"SIN", "COS", "TAN", "ARCSIN", "ARCCOS", "ARCTAN", "座標角度計算", "SIGN", "符号", "ABS", "絶対値", "EXP", "LN", "LOG", "FRAC", "小数部分", "整数部分", "乱数", "SQRT", "平方根"} {
		add(name, [][]string{{"の"}})
	}
	for _, name := range []string{"RAD2DEG", "DEG2RAD", "度変換", "ラジアン変換", "ROUND", "CEIL", "切上", "FLOOR", "切捨"} {
		add(name, [][]string{{"を"}})
	}
	for _, name := range []string{"ATAN2", "HYPOT", "斜辺"} {
		add(name, [][]string{{"と"}, {"の"}})
	}
	add("LOGN", [][]string{{"で"}, {"の"}})
	add("乱数範囲", [][]string{{"から"}, {"までの", "の"}})
	add("四捨五入", [][]string{{"を", "の"}})
	for _, name := range []string{"小数点切上", "小数点切下", "小数点四捨五入"} {
		add(name, [][]string{{"を"}, {"で"}})
	}
	return list
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	m := map[string]stdlib.Impl{}
	unary := func(fn func(float64) float64) stdlib.Impl {
		return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
			return value.Number(fn(first(args))), nil
		}
	}
	m["SIN"] = unary(math.Sin)
	m["COS"] = unary(math.Cos)
	m["TAN"] = unary(math.Tan)
	m["ARCSIN"] = unary(math.Asin)
	m["ARCCOS"] = unary(math.Acos)
	m["ARCTAN"] = unary(math.Atan)
	m["ABS"] = unary(math.Abs)
	m["絶対値"] = m["ABS"]
	m["EXP"] = unary(math.Exp)
	m["LN"] = unary(math.Log)
	m["LOG"] = m["LN"]
	m["SQRT"] = unary(math.Sqrt)
	m["平方根"] = m["SQRT"]
	m["RAD2DEG"] = unary(func(n float64) float64 { return n / math.Pi * 180 })
	m["度変換"] = m["RAD2DEG"]
	m["DEG2RAD"] = unary(func(n float64) float64 { return n / 180 * math.Pi })
	m["ラジアン変換"] = m["DEG2RAD"]
	m["SIGN"] = unary(func(n float64) float64 {
		if n == 0 {
			return 0
		}
		if n > 0 {
			return 1
		}
		return -1
	})
	m["符号"] = m["SIGN"]
	m["FRAC"] = unary(func(n float64) float64 { return math.Mod(n, 1) })
	m["小数部分"] = m["FRAC"]
	m["整数部分"] = unary(math.Trunc)
	m["ROUND"] = unary(jsRound)
	m["四捨五入"] = m["ROUND"]
	m["CEIL"] = unary(math.Ceil)
	m["切上"] = m["CEIL"]
	m["FLOOR"] = unary(math.Floor)
	m["切捨"] = m["FLOOR"]
	m["ATAN2"] = binary(math.Atan2)
	m["HYPOT"] = binary(math.Hypot)
	m["斜辺"] = m["HYPOT"]
	m["LOGN"] = binary(func(base, n float64) float64 { return math.Log(n) / math.Log(base) })
	m["小数点切上"] = decimalRound(math.Ceil)
	m["小数点切下"] = decimalRound(math.Floor)
	m["小数点四捨五入"] = decimalRound(jsRound)
	m["座標角度計算"] = func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		a, ok := valueAt(args, 0).Array()
		if !ok {
			return value.Undefined(), nil
		}
		return value.Number(math.Atan2(value.ToNumber(a.Get(1)), value.ToNumber(a.Get(0))) / math.Pi * 180), nil
	}
	m["乱数"] = func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		v := valueAt(args, 0)
		if n, ok := v.Number(); ok {
			return value.Number(math.Floor(rand.Float64() * n)), nil
		}
		if a, ok := v.Array(); ok {
			return randomRange(value.ToNumber(a.Get(0)), value.ToNumber(a.Get(1))), nil
		}
		if d, ok := v.Dict(); ok {
			lo, lok := d.Get("先頭")
			hi, hik := d.Get("末尾")
			if lok && hik {
				return randomRange(value.ToNumber(lo), value.ToNumber(hi)), nil
			}
		}
		return value.Undefined(), nil
	}
	m["乱数範囲"] = func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		return randomRange(first(args), second(args)), nil
	}
	return m
}

func first(args []value.Value) float64 {
	return value.ToNumber(valueAt(args, 0))
}

func second(args []value.Value) float64 { return value.ToNumber(valueAt(args, 1)) }

func valueAt(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func binary(fn func(float64, float64) float64) stdlib.Impl {
	return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		return value.Number(fn(first(args), second(args))), nil
	}
}

func decimalRound(fn func(float64) float64) stdlib.Impl {
	return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		base := math.Pow(10, second(args))
		return value.Number(fn(first(args)*base) / base), nil
	}
}

func jsRound(n float64) float64 { return math.Floor(n + 0.5) }

func randomRange(low, high float64) value.Value {
	return value.Number(math.Floor(rand.Float64()*(high-low+1)) + low)
}
