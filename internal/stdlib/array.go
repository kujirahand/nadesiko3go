package stdlib

import (
	"errors"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// arrayImpls returns the plugin_system_array commands.
//
// Arrays are references, so the commands that reorder or extend one change the
// caller's array in place, exactly as the JavaScript ones do.
func arrayImpls(m map[string]Impl) {
	m["要素数"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Number(float64(elementCount(arg(a, 0)))), nil
	}
	m["配列要素数"] = m["要素数"]

	m["配列結合"] = func(_ Context, a []value.Value) (value.Value, error) {
		sep := str(a, 1)
		if arr, ok := arg(a, 0).Array(); ok {
			parts := make([]string, arr.Len())
			for i := range parts {
				parts[i] = value.ToString(arr.Get(i))
			}
			return value.String(strings.Join(parts, sep)), nil
		}
		// 配列でなければ改行で区切ってから繋ぎ直す
		return value.String(strings.Join(strings.Split(str(a, 0), "\n"), sep)), nil
	}

	m["配列検索"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Number(-1), nil
		}
		want := arg(a, 1)
		for i := 0; i < arr.Len(); i++ {
			if value.StrictEquals(arr.Get(i), want) {
				return value.Number(float64(i)), nil
			}
		}
		return value.Number(-1), nil
	}

	// --- 出し入れ ---

	m["配列追加"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列追加』で配列以外の処理。")
		}
		arr.Set(arr.Len(), arg(a, 1))
		return arg(a, 0), nil
	}
	m["配列プッシュ"] = m["配列追加"]

	m["配列ポップ"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列ポップ』で配列以外の処理。")
		}
		if arr.Len() == 0 {
			return value.Undefined(), nil
		}
		last := arr.Get(arr.Len() - 1)
		arr.Truncate(arr.Len() - 1)
		return last, nil
	}

	m["配列挿入"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列挿入』で配列以外の要素への挿入。")
		}
		arr.Insert(int(value.ToNumber(arg(a, 1))), arg(a, 2))
		// splice の戻り値は取り除いた要素の配列。挿入だけなので空になる。
		return value.ArrayValue(value.NewArray()), nil
	}

	m["配列取出"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列取出』で配列以外を指定。")
		}
		removed := arr.Remove(int(value.ToNumber(arg(a, 1))), int(value.ToNumber(arg(a, 2))))
		return value.ArrayValue(value.NewArray(removed...)), nil
	}

	m["配列削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		return arrayCut(arg(a, 0), arg(a, 1))
	}
	m["配列切取"] = m["配列削除"]

	// --- 並べ替え ---

	m["配列ソート"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列ソート』で配列以外が指定されました。")
		}
		// 既定の Array.prototype.sort は文字列として比べる
		arr.SortStable(func(x, y value.Value) bool {
			return value.ToString(x) < value.ToString(y)
		})
		return arg(a, 0), nil
	}

	m["配列数値ソート"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列数値ソート』で配列以外が指定されました。")
		}
		arr.SortStable(func(x, y value.Value) bool {
			return value.ParseFloat(x) < value.ParseFloat(y)
		})
		return arg(a, 0), nil
	}

	m["配列逆順"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列逆順』で配列以外が指定されました。")
		}
		arr.Reverse()
		return arg(a, 0), nil
	}

	// --- 集計 ---

	m["配列合計"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列合計』で配列変数以外の値が指定されました。")
		}
		total := 0.0
		for i := 0; i < arr.Len(); i++ {
			// 数値にできない要素は読み飛ばす
			if n := value.ParseFloat(arr.Get(i)); !math.IsNaN(n) {
				total += n
			}
		}
		return value.Number(total), nil
	}
	m["配列最大値"] = reduceNumbers(math.Max)
	m["配列最小値"] = reduceNumbers(math.Min)

	// --- 生成 ---

	m["配列連番作成"] = func(_ Context, a []value.Value) (value.Value, error) {
		from := int(value.ToNumber(arg(a, 0)))
		to := int(value.ToNumber(arg(a, 1)))
		var items []value.Value
		for i := from; i <= to; i++ {
			items = append(items, value.Number(float64(i)))
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}

	m["配列要素作成"] = func(_ Context, a []value.Value) (value.Value, error) {
		count := int(value.ToNumber(arg(a, 1)))
		items := make([]value.Value, 0, max(count, 0))
		for i := 0; i < count; i++ {
			items = append(items, cloneValue(arg(a, 0)))
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}

	m["配列複製"] = func(_ Context, a []value.Value) (value.Value, error) {
		return cloneValue(arg(a, 0)), nil
	}

	// --- 関数を受け取る命令 ---

	m["配列マップ"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return eachElement(ctx, a, func(out *[]value.Value, item, got value.Value) {
			*out = append(*out, got)
		})
	}
	m["配列関数適用"] = m["配列マップ"]

	m["配列フィルタ"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return eachElement(ctx, a, func(out *[]value.Value, item, got value.Value) {
			if value.ToBool(got) {
				*out = append(*out, item)
			}
		})
	}
}

// eachElement runs a function value over every element of an array and lets
// collect decide what to keep. 『配列マップ』 and 『配列フィルタ』 differ only there.
func eachElement(ctx Context, a []value.Value, collect func(out *[]value.Value, item, got value.Value)) (value.Value, error) {
	fn, ok := arg(a, 0).Func()
	if !ok {
		return value.Undefined(), errors.New("配列に適用する関数が関数ではありません。")
	}
	arr, ok := arg(a, 1).Array()
	if !ok {
		return value.Undefined(), errors.New("配列以外が指定されました。")
	}
	var out []value.Value
	for i := 0; i < arr.Len(); i++ {
		item := arr.Get(i)
		got, err := ctx.CallFunc(fn, []value.Value{item})
		if err != nil {
			return value.Undefined(), err
		}
		collect(&out, item, got)
	}
	return value.ArrayValue(value.NewArray(out...)), nil
}

// arrayCut removes one element by index from an array, or one key from a
// dictionary, and returns what was removed.
func arrayCut(container, index value.Value) (value.Value, error) {
	switch container.Kind() {
	case value.KindArray:
		arr, _ := container.Array()
		removed := arr.Remove(int(value.ToNumber(index)), 1)
		if len(removed) == 0 {
			return value.Null(), nil
		}
		return removed[0], nil
	case value.KindDict:
		d, _ := container.Dict()
		key := value.ToString(index)
		v, ok := d.Get(key)
		if !ok {
			return value.Null(), nil
		}
		d.Delete(key)
		return v, nil
	}
	return value.Null(), nil
}

// reduceNumbers builds 『配列最大値』 and 『配列最小値』.
func reduceNumbers(f func(a, b float64) float64) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok || arr.Len() == 0 {
			return value.Undefined(), errors.New("空の配列は集計できません。")
		}
		acc := value.ToNumber(arr.Get(0))
		for i := 1; i < arr.Len(); i++ {
			acc = f(acc, value.ToNumber(arr.Get(i)))
		}
		return value.Number(acc), nil
	}
}

// elementCount reports the number of elements: array length, dictionary key
// count, or rune count for a string. Anything else counts as one.
func elementCount(v value.Value) int {
	switch v.Kind() {
	case value.KindArray:
		arr, _ := v.Array()
		return arr.Len()
	case value.KindDict:
		d, _ := v.Dict()
		return d.Len()
	case value.KindString:
		s, _ := v.String()
		return runeLen(s)
	}
	return 1
}

// cloneValue makes a deep copy, so that changing the copy leaves the original
// alone.
func cloneValue(v value.Value) value.Value {
	switch v.Kind() {
	case value.KindArray:
		arr, _ := v.Array()
		items := make([]value.Value, arr.Len())
		for i := range items {
			items[i] = cloneValue(arr.Get(i))
		}
		return value.ArrayValue(value.NewArray(items...))
	case value.KindDict:
		d, _ := v.Dict()
		out := value.NewDict()
		for _, k := range d.Keys() {
			item, _ := d.Get(k)
			out.Set(k, cloneValue(item))
		}
		return value.DictValue(out)
	}
	return v
}
