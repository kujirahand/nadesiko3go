package stdlib

import (
	"errors"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// timerImpls returns the plugin_system_timer commands.
//
// Time is virtual: 『0.1秒待つ』 runs the callbacks that fall inside that
// interval and returns immediately (AGENTS.md §8).
func timerImpls(m map[string]Impl) {
	m["秒待"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return value.Undefined(), ctx.Wait(value.ParseFloat(arg(a, 0)))
	}
	m["秒待機"] = m["秒待"]

	m["秒後"] = setTimer(false)
	m["秒毎"] = setTimer(true)
	m["秒タイマー開始時"] = m["秒毎"]

	m["タイマー停止"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return value.Bool(ctx.CancelTimer(value.ToNumber(arg(a, 0)))), nil
	}
	m["全タイマー停止"] = func(ctx Context, a []value.Value) (value.Value, error) {
		ctx.CancelAllTimers()
		return value.Undefined(), nil
	}
}

// setTimer builds 『秒後』 and 『秒毎』, which differ only in whether the callback
// repeats.
//
// Both put the timer id in 『対象』 as well as returning it, so that a callback
// can stop its own timer with 『対象のタイマー停止』.
func setTimer(repeat bool) Impl {
	return func(ctx Context, a []value.Value) (value.Value, error) {
		fn, ok := arg(a, 0).Func()
		if !ok && arg(a, 0).Kind() == value.KindString {
			fn = ctx.FindFunc(value.ToString(arg(a, 0)))
			ok = fn != nil
		}
		if !ok {
			return value.Undefined(), errors.New("タイマーに指定できるのは関数だけです。")
		}
		id, err := ctx.SetTimer(fn, value.ParseFloat(arg(a, 1)), repeat)
		if err != nil {
			return value.Undefined(), err
		}
		ctx.SetSysVar(SysTarget, value.Number(id))
		return value.Number(id), nil
	}
}

// SysTarget is the system variable 『対象』, which a timer id is left in.
const SysTarget = "対象"
