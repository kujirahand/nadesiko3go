package value

import "strings"

// Type : valueの型
type Type int

const (
	// Null : Null
	Null Type = iota
	// Int : 整数
	Int
	// Float : 実数
	Float
	// Str : 文字列
	Str
	// Bool : 真偽型
	Bool
	// Array : 配列
	Array
	// Hash : ハッシュ
	Hash
	// Function : 関数
	Function
	// UserFunc : ユーザー関数
	UserFunc
	// Bytes : バイナリ
	Bytes
)

// Value : Value
type Value struct {
	Type     Type
	Value    interface{}
	IValue   int
	FValue   float64
	Tag      int
	IsFreeze bool
}

// TArray : 配列型の型
type TArray []*Value

// THash : ハッシュ型の型
type THash map[string]*Value

// TFunction : 関数型の型
type TFunction func(args TArray) (*Value, error)

// NewValueNull : NULL型の値を返す
func NewValueNull() Value {
	return Value{Type: Null}
}

// NewValueNullPtr : NULL型の値を生成し、そのポインタを返す
func NewValueNullPtr() *Value {
	p := NewValueNull()
	return &p
}

// NewValueInt : 整数型を返す
func NewValueInt(v int) Value {
	return Value{Type: Int, IValue: v}
}

// NewValueIntPtr : 整数型を生成してそのポインタを返す
func NewValueIntPtr(v int) *Value {
	vv := NewValueInt(v)
	return &vv
}

// NewValueFloat : 実数型を生成
func NewValueFloat(v float64) Value {
	return Value{Type: Float, FValue: v}
}

// NewValueStr : 文字列を生成
func NewValueStr(v string) Value {
	return Value{Type: Str, Value: v}
}

// NewValueBytes : []byteを生成
func NewValueBytes(v []byte) Value {
	return Value{Type: Bytes, Value: v}
}

// NewValueBool : 真偽値
func NewValueBool(v bool) Value {
	i := 0
	if v {
		i = 1
	}
	return Value{Type: Bool, IValue: i}
}

// NewValueFunc : 関数オブジェクトを生成
func NewValueFunc(v TFunction) Value {
	return Value{Type: Function, Value: v}
}

// NewValueUserFunc : ユーザー定義関数を生成
func NewValueUserFunc(v int) Value {
	return Value{Type: UserFunc, Value: v}
}

// NewValueArray : 配列を生成
func NewValueArray() Value {
	return Value{Type: Array, Value: TArray{}}
}

// NewValueHash : ハッシュを生成
func NewValueHash() Value {
	return Value{Type: Hash, Value: THash{}}
}

// NewValueByType : タイプに応じた値を生成する
func NewValueByType(vtype Type, s string) Value {
	switch vtype {
	case Null:
		return Value{Type: Null, Value: nil}
	case Int:
		return NewValueInt(StrToInt(s))
	case Float:
		// IntにできるならIntに変換
		if strings.Index(s, ".") >= 0 {
			return NewValueFloat(StrToFloat(s))
		}
		return NewValueInt(StrToInt(s))
	case Str:
		return NewValueStr(s)
	case Bool:
		if s == "" {
			return NewValueBool(false)
		}
		return NewValueBool(true)
	default:
		return Value{Type: vtype, Value: s}
	}
}

// ToBool : 真偽型に変換
func (v *Value) ToBool() bool {
	switch v.Type {
	case Int, Bool:
		return (v.IValue != 0)
	case Float:
		i := v.ToInt()
		return (i != 0)
	case Str:
		s := v.ToString()
		return (s != "")
	}
	return false
}

// ToInt : 整数型に変換
func (v *Value) ToInt() int {
	switch v.Type {
	case Int, Bool:
		return v.IValue
	case Float:
		return int(v.FValue)
	case Str:
		return StrToInt(v.Value.(string))
	}
	return 0
}

// ToFloat : 実数型に変換
func (v *Value) ToFloat() float64 {
	switch v.Type {
	case Int, Bool:
		return float64(v.IValue)
	case Float:
		return v.FValue
	case Str:
		return StrToFloat(v.Value.(string))
	}
	return 0
}

// IsNumber : 数値？
func (v *Value) IsNumber() bool {
	switch v.Type {
	case Int:
		return true
	case Float:
		return true
	}
	return false
}

// SetInt : 整数を設定
func (v *Value) SetInt(value int) {
	v.Type = Int
	v.IValue = value
}

// SetFloat : 実数を設定
func (v *Value) SetFloat(value float64) {
	v.Type = Float
	v.FValue = value
}

// SetStr : 文字列を設定
func (v *Value) SetStr(value string) {
	v.Type = Str
	v.Value = value
}

// SetBool : 真偽型を設定
func (v *Value) SetBool(value bool) {
	v.Type = Bool
	if value {
		v.IValue = 1
	} else {
		v.IValue = 0
	}
}

// SetValue : Value型を設定
func (v *Value) SetValue(value *Value) {
	if value == nil {
		v.Type = Null
		v.Value = nil
		v.Tag = 0
		return
	}
	// Copy Type and Tag
	v.Type = value.Type
	v.Tag = value.Tag
	switch value.Type {
	case Int, Bool:
		v.IValue = value.IValue
	case Float:
		v.FValue = value.FValue
	case Str:
		v.Value = value.Value
	default:
		v.Value = value.Value
	}
}

// IsFunction : 関数タイプか
func (v *Value) IsFunction() bool {
	return v.Type == Function || v.Type == UserFunc
}

// ToString : 文字列型に変換
func (v *Value) ToString() string {
	if v == nil {
		return ""
	}
	switch v.Type {
	case Int:
		return IntToStr(v.IValue)
	case Float:
		return FloatToStr(v.FValue)
	case Str:
		return v.Value.(string)
	case Bool:
		if v.IValue != 0 {
			return "真"
		}
		return "偽"
	case Array:
		a := v.Value.(TArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(THash)
		return h.ToString()
	}
	return ""
}

// ToJSONString : Convert to JSON string
func (v *Value) ToJSONString() string {
	if v == nil {
		return "undefined"
	}
	switch v.Type {
	case Int:
		return IntToStr(v.IValue)
	case Float:
		return FloatToStr(v.FValue)
	case Str:
		return "\"" + EncodeStrToJSON(v.Value.(string)) + "\""
	case Bool:
		if true {
			return "true"
		}
		return "false"
	case Array:
		a := v.Value.(TArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(THash)
		return h.ToString()
	}
	return ""
}

// ToArray : to array
func (v *Value) ToArray() TArray {
	if v.Type != Array {
		return nil
	}
	return v.Value.(TArray)
}

// ToHash : to hash
func (v *Value) ToHash() THash {
	if v.Type != Hash {
		return nil
	}
	return v.Value.(THash)
}

// Append : append value to array value
func (v *Value) Append(val *Value) int {
	if v.Type != Array {
		v.Type = Array
		v.Value = TArray{}
	}
	b := append(v.Value.(TArray), val)
	v.Value = b
	return len(b)
}

// HashSet : append value to hash value
func (v *Value) HashSet(key string, val *Value) {
	if v.Type != Hash {
		v.Type = Hash
		v.Value = THash{}
	}
	vh := v.Value.(THash)
	vh[key] = val
}

// HashGet : get value from hash value
func (v *Value) HashGet(key string) *Value {
	if v.Type != Hash {
		return nil
	}
	vh := v.Value.(THash)
	return vh[key]
}

// ArraySet : Set value to array
func (v *Value) ArraySet(idx int, val *Value) {
	if v.Type != Array {
		v.Type = Array
		cv := NewValueStr(v.ToString())
		v.Value = TArray{&cv}
	}
	a := v.Value.(TArray)
	a[idx] = val
}

// ArrayGet : get value from array
func (v *Value) ArrayGet(idx int) *Value {
	if v.Type != Array {
		return nil
	}
	a := v.Value.(TArray)
	return a[idx]
}
