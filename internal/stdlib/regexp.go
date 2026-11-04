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
			found, err := pattern.FindAll(str(a, 0))
			if err != nil {
				return value.Undefined(), patternError(str(a, 1), err)
			}
			if found == nil {
				return value.Null(), nil
			}
			return stringArray(found), nil
		}

		matched, ok := pattern.Find(str(a, 0))
		if !ok {
			return value.Null(), nil
		}
		// 部分マッチは『抽出文字列』に入れ、戻り値は一致した全体にする。
		// 参加しなかった組はJavaScriptと同じくundefinedにし、空文字に
		// 一致した参加組は空文字のまま残す。
		ctx.SetSysVar(SysMatched, groupArray(matched.Groups))
		return value.String(matched.Text), nil
	}

	m["正規表現抽出"] = func(ctx Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		clearMatched(ctx)
		matches, err := pattern.FindAllMatches(str(a, 0))
		if err != nil {
			return value.Undefined(), patternError(str(a, 1), err)
		}
		flat := make([]value.Value, 0)
		rows := make([]value.Value, 0, len(matches))
		names := pattern.SubexpNames()
		hasNames := false
		for _, name := range names {
			if name != "" {
				hasNames = true
				break
			}
		}
		for _, match := range matches {
			if hasNames {
				row := value.NewDict()
				for i := 1; i <= len(match.Groups) && i < len(names); i++ {
					if names[i] != "" {
						item := groupValue(match.Groups[i-1])
						row.Set(names[i], item)
						flat = append(flat, item)
					}
				}
				rows = append(rows, value.DictValue(row))
				continue
			}
			groups := match.Groups
			if len(groups) == 0 {
				// 組がないときは一致した全体を1件として返す
				item := value.String(match.Text)
				flat = append(flat, item)
				rows = append(rows, value.ArrayValue(value.NewArray(item)))
				continue
			}
			items := make([]value.Value, len(groups))
			for i, group := range groups {
				items[i] = groupValue(group)
				flat = append(flat, items[i])
			}
			rows = append(rows, value.ArrayValue(value.NewArray(items...)))
		}
		ctx.SetSysVar(SysMatched, value.ArrayValue(value.NewArray(rows...)))
		return value.ArrayValue(value.NewArray(flat...)), nil
	}

	m["正規表現置換"] = func(_ Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		replaced, err := pattern.Replace(str(a, 0), str(a, 2))
		if err != nil {
			return value.Undefined(), patternError(str(a, 1), err)
		}
		return value.String(replaced), nil
	}

	m["正規表現区切"] = func(_ Context, a []value.Value) (value.Value, error) {
		pattern, err := compilePattern(str(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		parts := pattern.Split(str(a, 0))
		items := make([]value.Value, len(parts))
		for i, part := range parts {
			// 未参加の捕獲はJavaScriptと同じくundefinedにする。空文字に
			// 潰すと、区切りの結果に『』が混ざったのと区別できない。
			if part.Undefined {
				items[i] = value.Undefined()
				continue
			}
			items[i] = value.String(part.Text)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
}

// compilePattern compiles a pattern, turning an engine limitation into a
// message that names the construct rather than the engine.
func compilePattern(pattern string) (*re.Regexp, error) {
	compiled, err := re.Compile(pattern)
	if err != nil {
		return nil, patternError(pattern, err)
	}
	return compiled, nil
}

// patternError converts an error from compiling or running a pattern into a
// language error. The sticky context variant is built only when an operation
// actually needs it, so a StickyVariantError can surface from FindAll and
// friends, not only from Compile; both go through the same conversion so the
// wording does not depend on when the failure is detected.
func patternError(pattern string, err error) error {
	if errors.Is(err, re.ErrUnsupported) {
		return errors.New("正規表現『" + pattern + "』の後方参照や先読みには対応していません。")
	}
	var flagErr *re.FlagError
	if errors.As(err, &flagErr) {
		if flagErr.Duplicate {
			return errors.New("正規表現『" + pattern + "』のフラグ『" + string(flagErr.Flag) + "』が重複しています。")
		}
		if flagErr.Conflict != 0 {
			return errors.New("正規表現『" + pattern + "』のフラグ『" + string(flagErr.Flag) + "』と『" + string(flagErr.Conflict) + "』は同時に使えません。")
		}
		return errors.New("正規表現『" + pattern + "』のフラグ『" + string(flagErr.Flag) + "』は使えません。")
	}
	var variantErr *re.StickyVariantError
	if errors.As(err, &variantErr) {
		// 元のパターンは正しい。文脈を見る位置指定照合にだけ使えない。
		if variantErr.Depth {
			return errors.New("正規表現『" + pattern + "』はyフラグの位置指定照合には深すぎます。")
		}
		return errors.New("正規表現『" + pattern + "』はyフラグの位置指定照合に使えません。")
	}
	return errors.New("正規表現『" + pattern + "』が正しくありません。")
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

// groupValue は捕獲1つ分をなでしこの値にする。参加しなかった組はundefined、
// 空文字に一致した参加組は空文字にする。Goの正規表現はどちらも空文字で返す
// ので、re.Group.Okの区別をここで値モデルへ写す。
func groupValue(group re.Group) value.Value {
	if !group.Ok {
		return value.Undefined()
	}
	return value.String(group.Text)
}

// groupArray は捕獲の並びを値の配列にする。
func groupArray(groups []re.Group) value.Value {
	values := make([]value.Value, len(groups))
	for i, group := range groups {
		values[i] = groupValue(group)
	}
	return value.ArrayValue(value.NewArray(values...))
}
