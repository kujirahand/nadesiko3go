// Package value defines the runtime value model shared by the compiler, VM,
// standard library, and host boundary.
package value

type Kind uint8

const (
	KindUndefined Kind = iota
	KindNull
	KindBool
	KindNumber
	KindString
	KindArray
	KindDict
	KindFunc
)

type Value struct {
	kind Kind
	num  float64
	str  string
	arr  *Array
	dict *Dict
	fn   *Func
}

// Func is an opaque runtime function reference. Executable Go callbacks do not
// cross the value boundary; the VM resolves ID in its own function table.
type Func struct {
	ID int
}

func Undefined() Value       { return Value{kind: KindUndefined} }
func Null() Value            { return Value{kind: KindNull} }
func Bool(v bool) Value      { return Value{kind: KindBool, num: boolNumber(v)} }
func Number(v float64) Value { return Value{kind: KindNumber, num: v} }
func String(v string) Value  { return Value{kind: KindString, str: v} }
func ArrayValue(v *Array) Value {
	return Value{kind: KindArray, arr: v}
}
func DictValue(v *Dict) Value { return Value{kind: KindDict, dict: v} }
func FuncValue(v *Func) Value { return Value{kind: KindFunc, fn: v} }

func boolNumber(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func (v Value) Kind() Kind { return v.kind }
func (v Value) Bool() (bool, bool) {
	return v.num != 0, v.kind == KindBool
}
func (v Value) Number() (float64, bool) { return v.num, v.kind == KindNumber }
func (v Value) String() (string, bool)  { return v.str, v.kind == KindString }
func (v Value) Array() (*Array, bool)   { return v.arr, v.kind == KindArray }
func (v Value) Dict() (*Dict, bool)     { return v.dict, v.kind == KindDict }
func (v Value) Func() (*Func, bool)     { return v.fn, v.kind == KindFunc }
