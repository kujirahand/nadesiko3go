package stdlib

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/lexer/josi"
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
	m["言"] = m["表示"]
	m["コンソール表示"] = m["表示"]
	m["表示ログクリア"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		ctx.SetSysVar("表示ログ", value.String(""))
		return value.Undefined(), nil
	}

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
	m["空辞書"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return value.DictValue(value.NewDict()), nil
	}
	m["空ハッシュ"] = m["空辞書"]
	m["空オブジェクト"] = m["空辞書"]

	// --- 型変換と判定 ---
	m["整数変換"] = func(_ Context, args []value.Value) (value.Value, error) {
		return value.Number(value.ParseInt(arg(args, 0))), nil
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
	m["RGB"] = func(_ Context, args []value.Value) (value.Value, error) {
		component := func(v value.Value) string {
			s := "00" + strconv.FormatInt(int64(value.ParseInt(v)), 16)
			return s[len(s)-2:]
		}
		return value.String("#" + component(arg(args, 0)) + component(arg(args, 1)) + component(arg(args, 2))), nil
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
		name := value.ToString(arg(args, 0))
		if got := ctx.FindValue(name); got.Kind() != value.KindUndefined {
			return got, nil
		}
		if fn := ctx.FindFunc(name); fn != nil {
			return value.FuncValue(fn), nil
		}
		return value.Undefined(), nil
	}
	m["実行時間計測"] = func(ctx Context, args []value.Value) (value.Value, error) {
		item := arg(args, 0)
		fn, ok := item.Func()
		if !ok && item.Kind() == value.KindString {
			fn = ctx.FindFunc(value.ToString(item))
			ok = fn != nil
		}
		if !ok {
			return value.Undefined(), errors.New("『実行時間計測』には関数を指定してください。")
		}
		start := time.Now()
		if _, err := ctx.CallFunc(fn, nil); err != nil {
			return value.Undefined(), err
		}
		return value.Number(float64(time.Since(start).Nanoseconds()) / 1e6), nil
	}
	m["デバッグ表示"] = func(ctx Context, args []value.Value) (value.Value, error) {
		v := arg(args, 0)
		var s string
		if v.Kind() == value.KindArray || v.Kind() == value.KindDict {
			if jsonStr, err := encodeJSON(v); err == nil {
				s = jsonStr
			} else {
				s = value.ToString(v)
			}
		} else {
			s = value.ToString(v)
		}
		fname, line := ctx.CurrentSourcePos()
		var prefix string
		if fname != "" && line > 0 {
			prefix = fmt.Sprintf("%s(%d): ", fname, line)
		} else if line > 0 {
			prefix = fmt.Sprintf("(%d): ", line)
		}
		ctx.Print(prefix + s)
		return value.Undefined(), nil
	}
	m["ハテナ関数設定"] = func(ctx Context, args []value.Value) (value.Value, error) {
		ctx.SetCommandState("ハテナ関数", cloneValue(arg(args, 0)))
		return value.Undefined(), nil
	}
	m["ハテナ関数実行"] = func(ctx Context, args []value.Value) (value.Value, error) {
		pipeline := ctx.CommandState("ハテナ関数")
		if pipeline.Kind() == value.KindUndefined {
			pipeline = value.String("表示")
		}
		current := arg(args, 0)
		if names, ok := pipeline.Array(); ok {
			for i := 0; i < names.Len(); i++ {
				var err error
				current, err = ctx.CallCommand(value.ToString(names.Get(i)), []value.Value{current})
				if err != nil {
					return value.Undefined(), err
				}
			}
			return current, nil
		}
		return ctx.CallCommand(value.ToString(pipeline), []value.Value{current})
	}
	m["グローバル関数一覧取得"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		return stringsToArray(ctx.GlobalFuncNames()), nil
	}
	m["プラグイン名設定"] = func(ctx Context, args []value.Value) (value.Value, error) {
		ctx.SetSysVar("プラグイン名", arg(args, 0))
		return value.Undefined(), nil
	}
	m["名前空間設定"] = func(ctx Context, args []value.Value) (value.Value, error) {
		stack, _ := ctx.CommandState("名前空間スタック").Array()
		if stack == nil {
			stack = value.NewArray()
		}
		pair := value.NewArray(ctx.SysVar("名前空間"), ctx.SysVar("プラグイン名"))
		stack.Set(stack.Len(), value.ArrayValue(pair))
		ctx.SetCommandState("名前空間スタック", value.ArrayValue(stack))
		ctx.SetSysVar("名前空間", arg(args, 0))
		return value.Undefined(), nil
	}
	m["名前空間ポップ"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		stack, ok := ctx.CommandState("名前空間スタック").Array()
		if !ok || stack.Len() == 0 {
			return value.Undefined(), nil
		}
		pair, _ := stack.Get(stack.Len() - 1).Array()
		stack.Truncate(stack.Len() - 1)
		ctx.SetSysVar("名前空間", pair.Get(0))
		ctx.SetSysVar("プラグイン名", pair.Get(1))
		return value.Undefined(), nil
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
		return stringsToArray(lexer.ReservedWords()), nil
	}
	m["助詞一覧取得"] = func(_ Context, _ []value.Value) (value.Value, error) {
		return stringsToArray(josi.List), nil
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
