package stdlib

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	m["継続表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		ctx.Write(value.ToString(arg(args, 0)))
		return value.Undefined(), nil
	}
	m["連続表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		var b strings.Builder
		for _, item := range args {
			b.WriteString(value.ToString(item))
		}
		ctx.Print(b.String())
		return value.Undefined(), nil
	}
	m["連続無改行表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		var b strings.Builder
		for _, item := range args {
			b.WriteString(value.ToString(item))
		}
		ctx.Write(b.String())
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
	m["空配列"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return value.ArrayValue(value.NewArray()), nil
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
	m["非数判定"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Bool(math.IsNaN(value.ToNumber(arg(args, 0)))), nil
	}
	m["NAN判定"] = m["非数判定"]
	m["HEX"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.String(strconv.FormatInt(int64(value.ToNumber(arg(args, 0))), 16)), nil
	}
	m["進数変換"] = func(_ Context, args []value.Value) (value.Value, error) {
		base := int(value.ToNumber(arg(args, 1)))
		if base < 2 || base > 36 {
			return value.String(""), nil
		}
		return value.String(strconv.FormatInt(int64(value.ToNumber(arg(args, 0))), base)), nil
	}
	m["二進"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.String(strconv.FormatInt(int64(value.ToNumber(arg(args, 0))), 2)), nil
	}
	m["二進表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		got, _ := m["二進"](ctx, args)
		ctx.Print(value.ToString(got))
		return value.Undefined(), nil
	}

	for alias, canonical := range map[string]string{
		"TOSTR": "文字列変換", "TOINT": "整数変換", "INT": "整数変換",
		"TOFLOAT": "実数変換", "FLOAT": "実数変換", "TYPEOF": "変数型確認",
	} {
		m[alias] = m[canonical]
	}

	// --- エラー ---
	m["エラー発生"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Undefined(), errors.New(value.ToString(arg(args, 0)))
	}
	m["システム関数一覧取得"] = func(_ Context, _ []value.Value) (value.Value, error) {
		names := make([]string, 0)
		for name, item := range ParserFuncList() {
			if item.Type == "func" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return stringsToArray(names), nil
	}
	m["システム関数存在"] = func(_ Context, args []value.Value) (value.Value, error) {
		item, ok := ParserFuncList()[value.ToString(arg(args, 0))]
		return value.Bool(ok && item.Type == "func"), nil
	}
	m["実行"] = func(ctx Context, args []value.Value) (value.Value, error) {
		item := arg(args, 0)
		fn, ok := item.Func()
		if !ok && item.Kind() == value.KindString {
			fn = ctx.FindFunc(value.ToString(item))
			ok = fn != nil
		}
		if !ok {
			return item, nil
		}
		return ctx.CallFunc(fn, nil)
	}
	m["ASYNC"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return value.Undefined(), nil
	}
	m["AWAIT実行"] = func(ctx Context, args []value.Value) (value.Value, error) {
		callable := arg(args, 0)
		fn, ok := callable.Func()
		if !ok && callable.Kind() == value.KindString {
			fn = ctx.FindFunc(value.ToString(callable))
			ok = fn != nil
		}
		if !ok {
			return value.Undefined(), errors.New("『AWAIT実行』の第一引数はなでしこ関数名かFunction型で指定してください。")
		}
		callArgs := []value.Value{arg(args, 1)}
		if array, ok := arg(args, 1).Array(); ok {
			callArgs = array.Values()
		}
		return ctx.CallFunc(fn, callArgs)
	}
	m["JSオブジェクト取得"] = func(ctx Context, args []value.Value) (value.Value, error) {
		fn := ctx.FindFunc(value.ToString(arg(args, 0)))
		if fn == nil {
			return value.Undefined(), nil
		}
		return value.FuncValue(fn), nil
	}
	m["拝啓"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		ctx.SetCommandState("礼節レベル", value.Number(0))
		return value.Undefined(), nil
	}
	for _, name := range []string{"お願", "ください", "です"} {
		m[name] = func(ctx Context, _ []value.Value) (value.Value, error) {
			level := value.ToNumber(ctx.CommandState("礼節レベル"))
			ctx.SetCommandState("礼節レベル", value.Number(level+1))
			return value.Undefined(), nil
		}
	}
	m["敬具"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		level := value.ToNumber(ctx.CommandState("礼節レベル"))
		ctx.SetCommandState("礼節レベル", value.Number(level+100))
		return value.Undefined(), nil
	}
	m["礼節レベル取得"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.Number(value.ToNumber(ctx.CommandState("礼節レベル"))), nil
	}
	m["プラグイン一覧取得"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return stringsToArray([]string{"plugin_system"}), nil
	}
	m["モジュール一覧取得"] = m["プラグイン一覧取得"]
	m["予約語一覧取得"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return stringsToArray([]string{"もし", "違えば", "ここまで", "反復", "繰返", "回"}), nil
	}
	m["助詞一覧取得"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return stringsToArray([]string{"から", "まで", "を", "に", "へ", "で", "と", "の", "が"}), nil
	}

	mathImpls(m)
	stringImpls(m)
	urlImpls(m)
	datetimeImpls(m)
	arrayImpls(m)
	dictImpls(m)
	regexpImpls(m)
	timerImpls(m)
	return m
}

// allDigitsRE matches a string made only of digits, half-width or full-width,
// which is what 『数列判定』 asks about.
var allDigitsRE = regexp.MustCompile(`^[+\-＋－]?(?:[0-9０-９]+(?:[.．][0-9０-９]+)?|[.．][0-9０-９]+)(?:[eEｅＥ][+\-＋－]?[0-9０-９]+)?$`)

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

func stringsToArray(items []string) value.Value {
	values := make([]value.Value, len(items))
	for i, item := range items {
		values[i] = value.String(item)
	}
	return value.ArrayValue(value.NewArray(values...))
}
