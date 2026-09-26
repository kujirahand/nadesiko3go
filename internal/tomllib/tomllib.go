// Package tomllib は本家の外部プラグイン nadesiko3-toml (plugin_toml) 相当の
// 命令を実装する。plugin_system の互換保証対象外なので、Goらしい実装でよい。
package tomllib

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) FuncList() lexer.FuncList {
	return lexer.FuncList{
		"TOMLデコード":  {Name: "TOMLデコード", Type: "func", Josi: [][]string{{"を", "の", "から"}}, Pure: true},
		"TOMLエンコード": {Name: "TOMLエンコード", Type: "func", Josi: [][]string{{"を", "から", "の"}}, Pure: true},
		"TOML取得":    {Name: "TOML取得", Type: "func", Josi: [][]string{{"を", "の", "から"}}, Pure: true},
		"TOML変換":    {Name: "TOML変換", Type: "func", Josi: [][]string{{"を", "から", "の"}}, Pure: true},
	}
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	decode := func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		s := ""
		if len(args) > 0 {
			s = value.ToString(args[0])
		}
		return decodeTOML(s)
	}
	encode := func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		var v value.Value = value.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		s, err := encodeTOML(v)
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(s), nil
	}
	return map[string]stdlib.Impl{
		"TOMLデコード":  decode,
		"TOML取得":    decode,
		"TOMLエンコード": encode,
		"TOML変換":    encode,
	}
}

// decodeTOML はTOML文字列をなでしこの値（辞書・配列・文字列・数値・真偽値）にデコードする。
func decodeTOML(s string) (value.Value, error) {
	var data map[string]any
	if _, err := toml.Decode(s, &data); err != nil {
		return value.Undefined(), errors.New("TOMLデコードに失敗しました。" + err.Error())
	}
	return tomlAnyToValue(data), nil
}

// encodeTOML はなでしこの辞書をTOML文字列にエンコードする。
// TOMLの仕様上、最上位は辞書（テーブル）でなければならない。
func encodeTOML(v value.Value) (string, error) {
	m, ok := valueToTomlTable(v)
	if !ok {
		return "", errors.New("TOMLエンコードできるのは辞書のみです。")
	}
	var b strings.Builder
	enc := toml.NewEncoder(&b)
	if err := enc.Encode(m); err != nil {
		return "", errors.New("TOMLエンコードに失敗しました。" + err.Error())
	}
	return b.String(), nil
}

func tomlAnyToValue(v any) value.Value {
	switch t := v.(type) {
	case nil:
		return value.Null()
	case bool:
		return value.Bool(t)
	case int64:
		return value.Number(float64(t))
	case int:
		return value.Number(float64(t))
	case float64:
		return value.Number(t)
	case string:
		return value.String(t)
	case time.Time:
		return value.String(t.Format(time.RFC3339))
	case []map[string]any:
		items := make([]value.Value, len(t))
		for i, item := range t {
			items[i] = tomlAnyToValue(item)
		}
		return value.ArrayValue(value.NewArray(items...))
	case []any:
		items := make([]value.Value, len(t))
		for i, item := range t {
			items[i] = tomlAnyToValue(item)
		}
		return value.ArrayValue(value.NewArray(items...))
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		d := value.NewDict()
		for _, k := range keys {
			d.Set(k, tomlAnyToValue(t[k]))
		}
		return value.DictValue(d)
	default:
		return value.Undefined()
	}
}

func valueToTomlTable(v value.Value) (map[string]any, bool) {
	d, ok := v.Dict()
	if !ok || d == nil {
		return nil, false
	}
	m := make(map[string]any, d.Len())
	for _, k := range d.Keys() {
		item, _ := d.Get(k)
		if item.Kind() == value.KindUndefined {
			continue // undefined のキーは出力しない (JSONエンコードと同じ扱い)
		}
		m[k] = valueToTomlAny(item)
	}
	return m, true
}

func valueToTomlAny(v value.Value) any {
	switch v.Kind() {
	case value.KindUndefined, value.KindNull:
		return "" // TOMLにnullは無いので空文字列にする
	case value.KindBool:
		return value.ToBool(v)
	case value.KindNumber:
		n, _ := v.Number()
		if !math.IsNaN(n) && !math.IsInf(n, 0) && n == math.Trunc(n) &&
			n >= math.MinInt64 && n <= math.MaxInt64 {
			return int64(n)
		}
		return n
	case value.KindString:
		s, _ := v.String()
		return s
	case value.KindArray:
		arr, _ := v.Array()
		items := make([]any, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			items[i] = valueToTomlAny(arr.Get(i))
		}
		return items
	case value.KindDict:
		m, _ := valueToTomlTable(v)
		return m
	default:
		return ""
	}
}
