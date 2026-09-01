package stdlib

import (
	"errors"
	"math"
	"regexp"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// implementations returns the commands that are implemented so far. A command
// in the signature table without an entry here parses and compiles, but raises
// 未実装 when it actually runs.
func implementations() map[string]Impl {
	m := map[string]Impl{}

	// --- 表示と出力 ---
	m["表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		ctx.Print(value.ToString(arg(args, 0)))
		return value.Undefined(), nil
	}
	m["ハテナ関数実行"] = m["表示"] // 『??』のエイリアス #1745

	// --- 演算 ---
	m["足"] = binaryNumber(func(a, b value.Value) float64 {
		return value.Add(value.ParseFloat(a), value.ParseFloat(b))
	})
	m["掛"] = binaryNumber(value.Mul)
	m["AND"] = binaryNumber(value.BitAnd)
	m["OR"] = binaryNumber(value.BitOr)
	m["XOR"] = binaryNumber(value.BitXor)

	// --- 範囲オブジェクト (#1704) ---
	m["範囲"] = func(_ Context, args []value.Value) (value.Value, error) {
		d := value.NewDict()
		d.Set("先頭", arg(args, 0))
		d.Set("末尾", arg(args, 1))
		return value.DictValue(d), nil
	}

	// --- 型変換と判定 ---
	m["整数変換"] = func(_ Context, args []value.Value) (value.Value, error) {
		// TS版は parseInt 相当。小数点以下を切り捨てるので Trunc を使う。
		n := value.ParseFloat(arg(args, 0))
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return value.Number(n), nil
		}
		return value.Number(math.Trunc(n)), nil
	}
	m["実数変換"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Number(value.ParseFloat(arg(args, 0))), nil
	}
	m["文字列変換"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.String(value.ToString(arg(args, 0))), nil
	}
	m["変数型確認"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.String(value.TypeName(arg(args, 0))), nil
	}
	m["真偽判定"] = func(_ Context, args []value.Value) (value.Value, error) {
		// 真偽値ではなく『真』『偽』という文字列を返す
		if value.ToBool(arg(args, 0)) {
			return value.String("真"), nil
		}
		return value.String("偽"), nil
	}
	m["数列判定"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Bool(allDigitsRE.MatchString(value.ToString(arg(args, 0)))), nil
	}

	// --- エラー ---
	m["エラー発生"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Undefined(), errors.New(value.ToString(arg(args, 0)))
	}

	return m
}

// allDigitsRE matches a string made only of digits, half-width or full-width,
// which is what 『数列判定』 asks about.
var allDigitsRE = regexp.MustCompile(`^[0-9０-９]+$`)

// binaryNumber adapts a two-operand numeric operation into a command.
func binaryNumber(f func(a, b value.Value) float64) Impl {
	return func(_ Context, args []value.Value) (value.Value, error) {
		return value.Number(f(arg(args, 0), arg(args, 1))), nil
	}
}

// arg reads an argument, treating a missing one as undefined.
func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}
