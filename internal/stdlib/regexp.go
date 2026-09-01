package stdlib

import (
	"errors"

	"github.com/kujirahand/nadesiko3go/internal/re"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// SysMatched is the system variable 『抽出文字列』, where a non-global match
// leaves its capture groups.
const SysMatched = "抽出文字列"

// regexpImpls returns the plugin_system_regexp commands.
//
// A pattern is written 『/pattern/flags』. Anything else is used as the pattern
// itself with the 『g』 flag, which is why an empty pattern matches at every
// position rather than failing.
func regexpImpls(m map[string]Impl) {
	m["正規表現マッチ"] = func(ctx Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		clearMatched(ctx)

		if pattern.Global {
			// gが付くときは一致した全体の配列を返し、抽出文字列は空のまま
			found := pattern.FindAll(str(a, 0))
			if found == nil {
				return value.Null(), nil
			}
			return stringArray(found), nil
		}

		groups := pattern.Find(str(a, 0))
		if groups == nil {
			return value.Null(), nil
		}
		// 部分マッチは『抽出文字列』に入れ、戻り値は一致した全体にする
		ctx.SetSysVar(SysMatched, stringArray(groups[1:]))
		return value.String(groups[0]), nil
	}

	m["正規表現置換"] = func(_ Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(pattern.Replace(str(a, 0), str(a, 2))), nil
	}

	m["正規表現区切"] = func(_ Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		return stringArray(pattern.Split(str(a, 0))), nil
	}
}

// compilePattern compiles a pattern, turning an engine limitation into a
// message that names the construct rather than the engine.
func compilePattern(pattern string) (*re.Regexp, error) {
	compiled, err := re.Compile(pattern)
	if errors.Is(err, re.ErrUnsupported) {
		return nil, errors.New("正規表現『" + pattern + "』の後方参照や先読みには対応していません。")
	}
	if err != nil {
		return nil, errors.New("正規表現『" + pattern + "』が正しくありません。")
	}
	return compiled, nil
}

// clearMatched empties 『抽出文字列』 before a match, so that a failed match
// does not leave the previous groups behind.
func clearMatched(ctx Context) {
	ctx.SetSysVar(SysMatched, value.ArrayValue(value.NewArray()))
}

func stringArray(items []string) value.Value {
	values := make([]value.Value, len(items))
	for i, s := range items {
		values[i] = value.String(s)
	}
	return value.ArrayValue(value.NewArray(values...))
}
