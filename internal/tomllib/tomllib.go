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
	m, ok, err := valueToTomlTable(v, make(map[any]bool))
	if err != nil {
		return "", err
	}
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
		return value.String(formatTOMLTime(t))
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

// formatTOMLTime はデコードしたtime.Timeを、元のTOMLの表記（オフセット付き
// 日時／オフセット無しのローカル日時／日付のみ／時刻のみ）に合わせて文字列化する。
// BurntSushi/tomlは、オフセットの無い値を専用の*time.Locationの名前
// （"date-local"/"time-local"/"datetime-local"）で表す。
func formatTOMLTime(t time.Time) string {
	switch t.Location().String() {
	case "date-local":
		return t.Format("2006-01-02")
	case "time-local":
		return t.Format("15:04:05.999999999")
	case "datetime-local":
		return t.Format("2006-01-02T15:04:05.999999999")
	default:
		return t.Format(time.RFC3339)
	}
}

// valueToTomlTable はなでしこの辞書をTOMLエンコード用のmapに変換する。
// seenは現在の再帰経路上にある辞書・配列を記録し、循環参照を検出する
// （辞書・配列は参照型なので、自身や祖先を要素として持ちうる）。
func valueToTomlTable(v value.Value, seen map[any]bool) (map[string]any, bool, error) {
	d, ok := v.Dict()
	if !ok || d == nil {
		return nil, false, nil
	}
	if seen[d] {
		return nil, true, errors.New("TOMLエンコードできません。辞書が循環参照しています。")
	}
	seen[d] = true
	defer delete(seen, d)
	m := make(map[string]any, d.Len())
	for _, k := range d.Keys() {
		item, _ := d.Get(k)
		if item.Kind() == value.KindUndefined {
			continue // undefined のキーは出力しない (JSONエンコードと同じ扱い)
		}
		conv, err := valueToTomlAny(item, seen)
		if err != nil {
			return nil, true, err
		}
		m[k] = conv
	}
	return m, true, nil
}

func valueToTomlAny(v value.Value, seen map[any]bool) (any, error) {
	switch v.Kind() {
	case value.KindUndefined, value.KindNull:
		return "", nil // TOMLにnullは無いので空文字列にする
	case value.KindBool:
		return value.ToBool(v), nil
	case value.KindNumber:
		n, _ := v.Number()
		// float64は2^63をちょうど表現できるので、math.MaxInt64(float64に
		// 丸めると2^63になる)との比較だとint64の範囲外を通してしまう。
		// 上限は 0x1p63 (2^63) 未満で判定する。
		if !math.IsNaN(n) && !math.IsInf(n, 0) && n == math.Trunc(n) &&
			n >= math.MinInt64 && n < 0x1p63 {
			return int64(n), nil
		}
		return n, nil
	case value.KindString:
		s, _ := v.String()
		return s, nil
	case value.KindArray:
		arr, _ := v.Array()
		if seen[arr] {
			return nil, errors.New("TOMLエンコードできません。配列が循環参照しています。")
		}
		seen[arr] = true
		defer delete(seen, arr)
		items := make([]any, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			conv, err := valueToTomlAny(arr.Get(i), seen)
			if err != nil {
				return nil, err
			}
			items[i] = conv
		}
		return items, nil
	case value.KindDict:
		m, _, err := valueToTomlTable(v, seen)
		if err != nil {
			return nil, err
		}
		return m, nil
	default:
		return "", nil
	}
}
