package value

import "sort"

// Array keeps explicit undefined values when it is extended, matching a
// JavaScript sparse array's observable reads without exposing Go nil values.
type Array struct {
	items []Value
}

func NewArray(values ...Value) *Array {
	items := append([]Value(nil), values...)
	return &Array{items: items}
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
