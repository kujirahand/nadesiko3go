// Package value defines the runtime value model shared by the compiler, VM,
// standard library, and host boundary.
package value

import "unsafe"

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
	// data is shared by every reference-like kind. Keeping a single GC-visible
	// pointer instead of one field per kind reduces Value from 56 to 32 bytes.
	// For strings it points at the first byte and aux holds the byte length;
	// for arrays, dictionaries and functions it points at the object itself.
	data unsafe.Pointer
	num  float64
	aux  uintptr
	kind Kind
}

// Func is an opaque runtime function reference. Executable Go callbacks do not
// cross the value boundary; the VM resolves ID in its own function table.
//
// Captured holds the cells the function closed over, shared with the enclosing
// frame rather than copied.
type Func struct {
	ID       int
	Captured []*Cell
}

func Undefined() Value       { return Value{kind: KindUndefined} }
func Null() Value            { return Value{kind: KindNull} }
func Bool(v bool) Value      { return Value{kind: KindBool, num: boolNumber(v)} }
func Number(v float64) Value { return Value{kind: KindNumber, num: v} }
func String(v string) Value {
	return Value{kind: KindString, data: unsafe.Pointer(unsafe.StringData(v)), aux: uintptr(len(v))}
}
func ArrayValue(v *Array) Value {
	return Value{kind: KindArray, data: unsafe.Pointer(v)}
}
func DictValue(v *Dict) Value { return Value{kind: KindDict, data: unsafe.Pointer(v)} }
func FuncValue(v *Func) Value { return Value{kind: KindFunc, data: unsafe.Pointer(v)} }

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
func (v Value) String() (string, bool) {
	if v.kind != KindString {
		return "", false
	}
	return unsafe.String((*byte)(v.data), v.aux), true
}
func (v Value) Array() (*Array, bool) {
	if v.kind != KindArray {
		return nil, false
	}
	return (*Array)(v.data), true
}
func (v Value) Dict() (*Dict, bool) {
	if v.kind != KindDict {
		return nil, false
	}
	return (*Dict)(v.data), true
}
func (v Value) Func() (*Func, bool) {
	if v.kind != KindFunc {
		return nil, false
	}
	return (*Func)(v.data), true
}
