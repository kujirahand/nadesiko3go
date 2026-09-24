package value

import (
	"math"
	"sort"
	"strconv"
)

// Array keeps explicit undefined values when it is extended, matching a
// JavaScript sparse array's observable reads without exposing Go nil values.
type Array struct {
	items []Value
	props *Dict // 配列の名前付きプロパティ（非整数キー用）
}

func NewArray(values ...Value) *Array {
	items := append([]Value(nil), values...)
	return &Array{items: items}
}

// GetProp は配列の名前付きプロパティを取得する。存在しない場合は Undefined() を返す。
func (a *Array) GetProp(key string) Value {
	if a.props == nil {
		return Undefined()
	}
	v, ok := a.props.Get(key)
	if !ok {
		return Undefined()
	}
	return v
}

// SetProp は配列の名前付きプロパティを設定する。
func (a *Array) SetProp(key string, v Value) {
	if a.props == nil {
		a.props = NewDict()
	}
	a.props.Set(key, v)
}

// Props は配列のプロパティ辞書を返す（存在しない場合は nil）。
func (a *Array) Props() *Dict {
	return a.props
}

// HasProp は指定した名前付きプロパティが存在するかどうかを返す。
func (a *Array) HasProp(key string) bool {
	if a.props == nil {
		return false
	}
	_, ok := a.props.Get(key)
	return ok
}

// DeleteProp は指定した名前付きプロパティを削除する。
func (a *Array) DeleteProp(key string) bool {
	if a.props == nil {
		return false
	}
	return a.props.Delete(key)
}

func (a *Array) Len() int { return len(a.items) }

func (a *Array) Get(index int) Value {
	if index < 0 || index >= len(a.items) {
		return Undefined()
	}
	return a.items[index]
}

func (a *Array) Set(index int, v Value) bool {
	if index < 0 {
		return false
	}
	for len(a.items) <= index {
		a.items = append(a.items, Undefined())
	}
	a.items[index] = v
	return true
}

func (a *Array) Values() []Value {
	return append([]Value(nil), a.items...)
}

// Truncate shortens the array to n elements.
func (a *Array) Truncate(n int) {
	if n < 0 {
		n = 0
	}
	if n < len(a.items) {
		a.items = a.items[:n]
	}
}

// Insert puts v at index, shifting the rest right. An index past the end
// appends, and a negative index counts from the end, as splice does.
func (a *Array) Insert(index int, v Value) {
	index = a.clampSpliceStart(index)
	a.items = append(a.items, Undefined())
	copy(a.items[index+1:], a.items[index:])
	a.items[index] = v
}

// Remove takes count elements away starting at index and returns them, with
// the same clamping rules as splice.
func (a *Array) Remove(index, count int) []Value {
	index = a.clampSpliceStart(index)
	if count < 0 {
		count = 0
	}
	end := index + count
	if end > len(a.items) {
		end = len(a.items)
	}
	removed := append([]Value(nil), a.items[index:end]...)
	a.items = append(a.items[:index], a.items[end:]...)
	return removed
}

// clampSpliceStart resolves a splice start index: negative counts from the
// end, and anything past the end lands at the end.
func (a *Array) clampSpliceStart(index int) int {
	if index < 0 {
		index += len(a.items)
		if index < 0 {
			index = 0
		}
	}
	if index > len(a.items) {
		index = len(a.items)
	}
	return index
}

// SortStable reorders the array in place, keeping equal elements in order.
func (a *Array) SortStable(less func(x, y Value) bool) {
	sort.SliceStable(a.items, func(i, j int) bool { return less(a.items[i], a.items[j]) })
}

// Reverse reverses the array in place.
func (a *Array) Reverse() {
	for i, j := 0, len(a.items)-1; i < j; i, j = i+1, j-1 {
		a.items[i], a.items[j] = a.items[j], a.items[i]
	}
}

// MaxArrayIndex はJavaScript (ECMA-262) の配列インデックスの上限（2^32 - 2）。
const MaxArrayIndex = 4294967294

// AsArrayIndex は値 v が配列要素のインデックス（非負整数）かどうかを判定し、
// 配列インデックスであればその整数値と true を返す。
// それ以外のキー（非数値文字列、負数、小数など）はオブジェクトのプロパティ名として扱う。
func AsArrayIndex(v Value) (int, bool) {
	switch v.Kind() {
	case KindNumber:
		n, _ := v.Number()
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || math.Trunc(n) != n || n > MaxArrayIndex {
			return 0, false
		}
		return int(n), true
	case KindString:
		s, _ := v.String()
		if s == "" {
			return 0, false
		}
		// 正準数値文字列: "0" は許容するが、先行ゼロを持つ "01" 等はプロパティ名
		if len(s) > 1 && s[0] == '0' {
			return 0, false
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return 0, false
			}
		}
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil || u > MaxArrayIndex {
			return 0, false
		}
		return int(u), true
	default:
		return 0, false
	}
}
