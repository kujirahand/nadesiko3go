package value

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
