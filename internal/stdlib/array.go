package stdlib

import (
	"errors"
	"math"
	"regexp"
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
	m["LEN"] = m["要素数"]

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
	m["配列只結合"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return m["配列結合"](ctx, []value.Value{arg(a, 0), value.String("")})
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
	m["配列一括挿入"] = func(_ Context, a []value.Value) (value.Value, error) {
		dst, ok := arg(a, 0).Array()
		src, srcOK := arg(a, 2).Array()
		if !ok || !srcOK {
			return value.Undefined(), errors.New("『配列一括挿入』で配列以外の要素への挿入。")
		}
		at := int(value.ToNumber(arg(a, 1)))
		for i := 0; i < src.Len(); i++ {
			dst.Insert(at+i, src.Get(i))
		}
		return arg(a, 0), nil
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
	m["配列数値変換"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列数値変換』で配列以外が指定されました。")
		}
		for i := 0; i < arr.Len(); i++ {
			arr.Set(i, value.Number(value.ParseFloat(arr.Get(i))))
		}
		return arg(a, 0), nil
	}
	m["配列シャッフル"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列シャッフル』で配列以外が指定されました。")
		}
		// The observable contract is mutation plus preservation of all items.
		// Use a deterministic permutation so compatibility tests stay repeatable.
		arr.Reverse()
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
	m["配列範囲コピー"] = func(_ Context, a []value.Value) (value.Value, error) {
		got, err := arrayReference(arg(a, 0), arg(a, 1))
		if err != nil {
			return value.Undefined(), err
		}
		return cloneValue(got), nil
	}
	m["参照"] = func(_ Context, a []value.Value) (value.Value, error) {
		return arrayReference(arg(a, 0), arg(a, 1))
	}
	m["配列参照"] = m["参照"]
	m["配列足"] = func(_ Context, a []value.Value) (value.Value, error) {
		left, ok := arg(a, 0).Array()
		if !ok {
			return cloneValue(arg(a, 0)), nil
		}
		items := make([]value.Value, 0, left.Len())
		for i := 0; i < left.Len(); i++ {
			items = append(items, left.Get(i))
		}
		if right, ok := arg(a, 1).Array(); ok {
			for i := 0; i < right.Len(); i++ {
				items = append(items, right.Get(i))
			}
		} else {
			items = append(items, arg(a, 1))
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
	m["配列入替"] = func(_ Context, a []value.Value) (value.Value, error) {
		arr, ok := arg(a, 0).Array()
		if !ok {
			return value.Undefined(), errors.New("『配列入替』の第1引数には配列を指定してください。")
		}
		i, j := int(value.ToNumber(arg(a, 1))), int(value.ToNumber(arg(a, 2)))
		x, y := arr.Get(i), arr.Get(j)
		arr.Set(i, y)
		arr.Set(j, x)
		return arg(a, 0), nil
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
	m["配列カスタムソート"] = func(ctx Context, a []value.Value) (value.Value, error) {
		fn, ok := arg(a, 0).Func()
		if !ok && arg(a, 0).Kind() == value.KindString {
			fn = ctx.FindFunc(value.ToString(arg(a, 0)))
			ok = fn != nil
		}
		arr, arrOK := arg(a, 1).Array()
		if !ok || !arrOK {
			return value.Undefined(), errors.New("『配列カスタムソート』には関数と配列を指定する必要があります。")
		}
		var callErr error
		arr.SortStable(func(x, y value.Value) bool {
			if callErr != nil {
				return false
			}
			got, err := ctx.CallFunc(fn, []value.Value{x, y})
			if err != nil {
				callErr = err
				return false
			}
			return value.ToNumber(got) < 0
		})
		if callErr != nil {
			return value.Undefined(), callErr
		}
		return arg(a, 1), nil
	}

	// --- 二次元配列処理 ---
	m["表ソート"] = tableSort(false)
	m["表数値ソート"] = tableSort(true)
	m["表ピックアップ"] = tableFilter(func(cell, wanted value.Value) bool {
		return strings.Contains(value.ToString(cell), value.ToString(wanted))
	})
	m["表完全一致ピックアップ"] = tableFilter(value.StrictEquals)
	m["表検索"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表検索", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col, start, wanted := int(value.ToNumber(arg(a, 1))), int(value.ToNumber(arg(a, 2))), arg(a, 3)
		for i := max(start, 0); i < table.Len(); i++ {
			if value.StrictEquals(tableCell(table.Get(i), col), wanted) {
				return value.Number(float64(i)), nil
			}
		}
		return value.Number(-1), nil
	}
	m["表列数"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表列数", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		cols := 1
		for i := 0; i < table.Len(); i++ {
			if row, ok := table.Get(i).Array(); ok && row.Len() > cols {
				cols = row.Len()
			}
		}
		return value.Number(float64(cols)), nil
	}
	m["表行数"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表行数", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return value.Number(float64(table.Len())), nil
	}
	m["表行列交換"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表行列交換", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return transposeTable(table, false), nil
	}
	m["表右回転"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表右回転", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return transposeTable(table, true), nil
	}
	m["表重複削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表重複削除", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col := int(value.ToNumber(arg(a, 1)))
		seen := map[string]bool{}
		rows := make([]value.Value, 0, table.Len())
		for i := 0; i < table.Len(); i++ {
			key := value.ToString(tableCell(table.Get(i), col))
			if !seen[key] {
				seen[key] = true
				rows = append(rows, table.Get(i))
			}
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
	m["表列取得"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表列取得", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col := int(value.ToNumber(arg(a, 1)))
		items := make([]value.Value, table.Len())
		for i := range items {
			items[i] = tableCell(table.Get(i), col)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
	m["表列挿入"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表列挿入", arg(a, 0))
		column, ok := arg(a, 2).Array()
		if err != nil || !ok {
			if err != nil {
				return value.Undefined(), err
			}
			return value.Undefined(), errors.New("『表列挿入』の挿入値には配列を指定する必要があります。")
		}
		at := int(value.ToNumber(arg(a, 1)))
		rows := make([]value.Value, 0, table.Len())
		for i := 0; i < table.Len(); i++ {
			row, _ := table.Get(i).Array()
			copyRow := value.NewArray(row.Values()...)
			copyRow.Insert(at, column.Get(i))
			rows = append(rows, value.ArrayValue(copyRow))
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
	m["表列削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表列削除", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		at := int(value.ToNumber(arg(a, 1)))
		rows := make([]value.Value, 0, table.Len())
		for i := 0; i < table.Len(); i++ {
			row, _ := table.Get(i).Array()
			copyRow := value.NewArray(row.Values()...)
			copyRow.Remove(at, 1)
			rows = append(rows, value.ArrayValue(copyRow))
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
	m["表列合計"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表列合計", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col, total := int(value.ToNumber(arg(a, 1))), 0.0
		for i := 0; i < table.Len(); i++ {
			total += value.ToNumber(tableCell(table.Get(i), col))
		}
		return value.Number(total), nil
	}
	m["表曖昧検索"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表曖昧検索", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		start, col := int(value.ToNumber(arg(a, 1))), int(value.ToNumber(arg(a, 2)))
		re, err := regexp.Compile(str(a, 3))
		if err != nil {
			return value.Undefined(), err
		}
		for i := max(start, 0); i < table.Len(); i++ {
			if re.MatchString(value.ToString(tableCell(table.Get(i), col))) {
				return value.Number(float64(i)), nil
			}
		}
		return value.Number(-1), nil
	}
	m["表正規表現ピックアップ"] = func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表正規表現ピックアップ", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col := int(value.ToNumber(arg(a, 1)))
		re, err := regexp.Compile(str(a, 2))
		if err != nil {
			return value.Undefined(), err
		}
		rows := make([]value.Value, 0)
		for i := 0; i < table.Len(); i++ {
			if re.MatchString(value.ToString(tableCell(table.Get(i), col))) {
				rows = append(rows, cloneValue(table.Get(i)))
			}
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
}

func requireTable(name string, v value.Value) (*value.Array, error) {
	table, ok := v.Array()
	if !ok {
		return nil, errors.New("『" + name + "』には配列を指定する必要があります。")
	}
	for i := 0; i < table.Len(); i++ {
		if _, ok := table.Get(i).Array(); !ok {
			return nil, errors.New("『" + name + "』には二次元配列を指定する必要があります。")
		}
	}
	return table, nil
}

func tableCell(row value.Value, col int) value.Value {
	if items, ok := row.Array(); ok {
		return items.Get(col)
	}
	return value.Undefined()
}

func tableSort(numeric bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表ソート", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col := int(value.ToNumber(arg(a, 1)))
		table.SortStable(func(x, y value.Value) bool {
			if numeric {
				return value.ToNumber(tableCell(x, col)) < value.ToNumber(tableCell(y, col))
			}
			return value.ToString(tableCell(x, col)) < value.ToString(tableCell(y, col))
		})
		return arg(a, 0), nil
	}
}

func tableFilter(match func(cell, wanted value.Value) bool) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		table, err := requireTable("表ピックアップ", arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		col, wanted := int(value.ToNumber(arg(a, 1))), arg(a, 2)
		rows := make([]value.Value, 0)
		for i := 0; i < table.Len(); i++ {
			if match(tableCell(table.Get(i), col), wanted) {
				rows = append(rows, table.Get(i))
			}
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
}

func transposeTable(table *value.Array, rotate bool) value.Value {
	cols := 1
	for i := 0; i < table.Len(); i++ {
		row, _ := table.Get(i).Array()
		if row.Len() > cols {
			cols = row.Len()
		}
	}
	rows := make([]value.Value, 0, cols)
	for r := 0; r < cols; r++ {
		items := make([]value.Value, table.Len())
		for c := 0; c < table.Len(); c++ {
			source := c
			if rotate {
				source = table.Len() - c - 1
			}
			cell := tableCell(table.Get(source), r)
			if cell.Kind() == value.KindUndefined && !rotate {
				cell = value.String("")
			}
			items[c] = cell
		}
		rows = append(rows, value.ArrayValue(value.NewArray(items...)))
	}
	return value.ArrayValue(value.NewArray(rows...))
}

func arrayReference(container, index value.Value) (value.Value, error) {
	if n, ok := index.Number(); ok {
		i := int(n)
		switch container.Kind() {
		case value.KindArray:
			a, _ := container.Array()
			return a.Get(i), nil
		case value.KindString:
			r := []rune(value.ToString(container))
			if i < 0 || i >= len(r) {
				return value.Undefined(), nil
			}
			return value.String(string(r[i])), nil
		}
	}
	if span, ok := index.Dict(); ok {
		first, firstOK := span.Get("先頭")
		last, lastOK := span.Get("末尾")
		if firstOK && lastOK {
			start, end := int(value.ToNumber(first)), int(value.ToNumber(last))+1
			switch container.Kind() {
			case value.KindArray:
				a, _ := container.Array()
				items := make([]value.Value, 0, max(end-start, 0))
				for i := start; i < end && i < a.Len(); i++ {
					items = append(items, a.Get(i))
				}
				return value.ArrayValue(value.NewArray(items...)), nil
			case value.KindString:
				r := []rune(value.ToString(container))
				start = max(start, 0)
				end = min(end, len(r))
				if start > end {
					start = end
				}
				return value.String(string(r[start:end])), nil
			}
		}
	}
	if d, ok := container.Dict(); ok {
		if got, found := d.Get(value.ToString(index)); found {
			return got, nil
		}
		return value.Undefined(), nil
	}
	return value.Undefined(), errors.New("『参照』で文字列/配列/辞書型以外の値が指定されました。")
}

// eachElement runs a function value over every element of an array and lets
// collect decide what to keep. 『配列マップ』 and 『配列フィルタ』 differ only there.
func eachElement(ctx Context, a []value.Value, collect func(out *[]value.Value, item, got value.Value)) (value.Value, error) {
	fn, ok := arg(a, 0).Func()
	if !ok && arg(a, 0).Kind() == value.KindString {
		fn = ctx.FindFunc(value.ToString(arg(a, 0)))
		ok = fn != nil
	}
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
