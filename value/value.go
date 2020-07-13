package value

// プロファイラを見ると、NewValueXXXのコストが高い

import "strings"

// VType : valueの型
type VType int

const (
	// Null : Null
	Null VType = iota
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
	Type  VType
	Value interface{}
	// IsConst bool // 不変な値(immutable)かどうか
}

var valueTypeStr = map[VType]string{
	Null:     "Null",
	Int:      "Int",
	Float:    "Float",
	Str:      "Str",
	Bool:     "Bool",
	Array:    "Array",
	Hash:     "Hash",
	Function: "Function",
	UserFunc: "UserFunc",
	Bytes:    "Bytes",
}

// TypeToStr : value.Type を文字列に変換
func TypeToStr(t VType) string {
	return valueTypeStr[t]
}

// --- NewValueXXX ---

// NewNullPtr : NULL型の値を生成し、そのポインタを返す
func NewNullPtr() *Value {
	ga := GetGabadgeMan()
	p := ga.NewValue()
	return p
}

// NewIntPtr : 整数型を生成してそのポインタを返す
func NewIntPtr(v int) *Value {
	p := NewNullPtr()
	p.Type = Int
	p.Value = v
	return p
}

// NewFloatPtr : 実数型を生成
func NewFloatPtr(v float64) *Value {
	p := NewNullPtr()
	p.Type = Float
	p.Value = v
	return p
}

// NewStrPtr : 文字列を生成
func NewStrPtr(v string) *Value {
	p := NewNullPtr()
	p.Type = Str
	p.Value = v
	return p
}

// NewBytes : []byteを生成
func NewBytes(v []byte) Value {
	return Value{Type: Bytes, Value: v}
}

// NewBoolPtr : 真偽値
func NewBoolPtr(v bool) *Value {
	i := 0
	if v {
		i = 1
	}
	p := NewNullPtr()
	p.Type = Bool
	p.Value = i
	return p
}

// NewArrayPtr : 配列を生成
func NewArrayPtr() *Value {
	return &Value{
		Type:  Array,
		Value: NewTArray(),
	}
}

// NewArrayPtrFromStrings : 配列を生成
func NewArrayPtrFromStrings(sa []string) *Value {
	a := NewArrayPtr()
	for _, v := range sa {
		a.Append(NewStrPtr(v))
	}
	return a
}

// NewHashPtr : ハッシュを生成
func NewHashPtr() *Value {
	return &Value{Type: Hash, Value: THash{}}
}

// NewByType : タイプに応じた値を生成する
func NewByType(vtype VType, s string) *Value {
	switch vtype {
	case Null:
		return NewNullPtr()
	case Int:
		return NewIntPtr(StrToInt(s))
	case Float:
		// IntにできるならIntに変換
		if strings.Index(s, ".") >= 0 {
			return NewFloatPtr(StrToFloat(s))
		}
		return NewIntPtr(StrToInt(s))
	case Str:
		return NewStrPtr(s)
	case Bool:
		if s == "" {
			return NewBoolPtr(false)
		}
		return NewBoolPtr(true)
	default:
		return NewNullPtr()
	}
}

// ToBool : 真偽型に変換
func (v *Value) ToBool() bool {
	switch v.Type {
	case Int, Bool:
		return (v.Value.(int) != 0)
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
		return v.Value.(int)
	case Float:
		return int(v.Value.(float64))
	case Str:
		return StrToInt(v.Value.(string))
	}
	return 0
}

// ToFloat : 実数型に変換
func (v *Value) ToFloat() float64 {
	switch v.Type {
	case Int, Bool:
		return float64(v.Value.(int))
	case Float:
		return v.Value.(float64)
	case Str:
		return StrToFloat(v.Value.(string))
	}
	return 0
}

// Length : 要素の長さ
func (v *Value) Length() int {
	if v == nil {
		return 0
	}
	switch v.Type {
	case Array:
		a := v.Value.(*TArray)
		return a.Length()
	case Hash:
		h := v.Value.(THash)
		return h.Length()
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

// Clear : クリア
func (v *Value) Clear() {
	switch v.Type {
	case Array:
		a := v.Value.(TArray)
		a.Clear()
	case Hash:
		h := v.Value.(THash)
		h.Clear()
	}
	v.Type = Null
	v.Value = nil
}

// SetInt : 整数を設定
func (v *Value) SetInt(value int) {
	v.Type = Int
	v.Value = value
}

// SetFloat : 実数を設定
func (v *Value) SetFloat(value float64) {
	v.Type = Float
	v.Value = value
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
		v.Value = 1
	} else {
		v.Value = 0
	}
}

// SetValue : Value型を設定
func (v *Value) SetValue(value *Value) {
	if value == nil {
		v.Type = Null
		v.Value = nil
		return
	}
	// Copy Meta
	v.Type = value.Type
	// Copy Value
	switch value.Type {
	case Int, Bool:
		v.Value = value.Value.(int)
	case Float:
		v.Value = value.Value.(float64)
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
	case Null:
		return ""
	case Int:
		return IntToStr(v.Value.(int))
	case Float:
		return FloatToStr(v.Value.(float64))
	case Str:
		return v.Value.(string)
	case Bool:
		if v.Value.(int) != 0 {
			return "真"
		}
		return "偽"
	case Array:
		a := v.Value.(*TArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(THash)
		return h.ToString()
	case UserFunc:
		uf := v.Value.(TFuncValue)
		return uf.Name
	case Function:
		uf := v.Value.(TFuncValue)
		return uf.Name
	}
	return v.ToJSONString()
}

// IsSimpleValue : is simple value?
func (v *Value) IsSimpleValue() bool {
	if v == nil {
		return true
	}
	switch v.Type {
	case Null, Int, Float, Str, Bool:
		return true
	default:
		return false
	}
}

// ToJSONString : Convert to JSON string
func (v *Value) ToJSONString() string {
	if v == nil {
		return "undefined"
	}
	switch v.Type {
	case Null:
		return "null"
	case Int:
		return IntToStr(v.Value.(int))
	case Float:
		return FloatToStr(v.Value.(float64))
	case Str:
		return "\"" + EncodeStrToJSON(v.Value.(string)) + "\""
	case Bool:
		if v.ToBool() {
			return "true"
		}
		return "false"
	case Array:
		a := v.Value.(*TArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(THash)
		return h.ToString()
	case Function:
		return "\"[Function]\""
	case UserFunc:
		return "\"[UserFunction]\""
	}
	return ""
}

// ToJSONStringFormat : Convert to JSON string
func (v *Value) ToJSONStringFormat(level int) string {
	tab := ""
	for i := 0; i < level; i++ {
		tab += "  "
	}
	if v == nil {
		return "undefined"
	}
	switch v.Type {
	case Array:
		a := v.Value.(*TArray)
		return a.ToJSONStringFormat(level)
	case Hash:
		h := v.Value.(THash)
		return h.ToJSONStringFormat(level)
	default:
		return tab + v.ToJSONString()
	}
}

// ToArray : to array
func (v *Value) ToArray() *TArray {
	if v.Type != Array {
		return nil
	}
	a := v.Value.(*TArray)
	return a
}

// ToArrayItems : to array
func (v *Value) ToArrayItems() []*Value {
	if v.Type != Array {
		return nil
	}
	return v.Value.(*TArray).GetItems()
}

// ToHash : to hash
func (v *Value) ToHash() THash {
	if v.Type != Hash {
		return nil
	}
	return v.Value.(THash)
}

// Append : append value to array value
func (v *Value) Append(val *Value) {
	if v.Type != Array {
		v.Type = Array
		cv := NewStrPtr(v.ToString())
		a := NewTArray()
		a.Append(cv)
		v.Value = a
	}
	a := v.Value.(*TArray)
	a.Append(val)
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

// HashKeys : get keys
func (v *Value) HashKeys() []string {
	if v.Type != Hash {
		return []string{}
	}
	vh := v.Value.(THash)
	return vh.Keys()
}

// ArraySet : Set value to array
func (v *Value) ArraySet(idx int, val *Value) {
	if v.Type != Array {
		v.Type = Array
		cv := NewStrPtr(v.ToString())
		a := NewTArray()
		a.Append(cv)
		v.Value = a
	}
	a := v.Value.(*TArray)
	a.Set(idx, val)
}

// ArrayGet : get value from array
func (v *Value) ArrayGet(idx int) *Value {
	if v.Type != Array {
		return nil
	}
	a := v.Value.(*TArray)
	return a.Get(idx)
}

// Clone : clone value
func (v *Value) Clone() *Value {
	if v == nil {
		return NewNullPtr()
	}
	var res *Value = nil
	// Clone basic data
	switch v.Type {
	case Int:
		res = NewIntPtr(v.ToInt())
	case Float:
		res = NewFloatPtr(v.Value.(float64))
	case Str:
		return NewStrPtr(v.ToString())
	case Array:
		va := NewArrayPtr()
		a := va.Value.(*TArray)
		for _, v := range a.items {
			va.Append(v.Clone())
		}
		res = va
	case Hash:
		vh := NewHashPtr()
		h := vh.Value.(THash)
		for key, val := range h {
			vh.HashSet(key, val.Clone())
		}
		res = vh
	default:
		tmp := NewNullPtr()
		tmp.Type = v.Type
		tmp.Value = v.Value
		res = tmp
	}
	return res
}
