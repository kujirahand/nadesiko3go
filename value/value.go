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
	Type    Type
	Value   interface{}
	Tag     int
	IsConst bool // 不変な値(immutable)かどうか
}

// TArrayItems : 値のスライス
type TArrayItems []*Value

// TArray : 配列型の型
type TArray struct {
	Items  TArrayItems
	length int
}

// THash : ハッシュ型の型
type THash map[string]*Value

// TFunction : 関数型の型
type TFunction func(args *TArray) (*Value, error)

var valueTypeStr = map[Type]string{
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
func TypeToStr(t Type) string {
	return valueTypeStr[t]
}

// --- NewValueXXX ---

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
	return Value{Type: Int, Value: v}
}

// NewValueIntPtr : 整数型を生成してそのポインタを返す
func NewValueIntPtr(v int) *Value {
	vv := NewValueInt(v)
	return &vv
}

// NewValueFloat : 実数型を生成
func NewValueFloat(v float64) Value {
	return Value{Type: Float, Value: v}
}

// NewValueFloatPtr : 実数型を生成
func NewValueFloatPtr(v float64) *Value {
	f := NewValueFloat(v)
	return &f
}

// NewValueStr : 文字列を生成
func NewValueStr(v string) Value {
	return Value{Type: Str, Value: v}
}

// NewValueStrPtr : 文字列を生成
func NewValueStrPtr(v string) *Value {
	s := NewValueStr(v)
	return &s
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
	return Value{Type: Bool, Value: i}
}

// NewValueBoolPtr : 真偽値
func NewValueBoolPtr(v bool) *Value {
	p := NewValueBool(v)
	return &p
}

// NewValueFunc : 関数オブジェクトを生成
func NewValueFunc(v TFunction) Value {
	return Value{Type: Function, Value: v}
}

// NewValueUserFunc : ユーザー定義関数を生成
func NewValueUserFunc(v interface{}) Value {
	return Value{
		Type:  UserFunc,
		Value: v,  // Link to Node.TNodeDefFunc
		Tag:   -1, // ArgsList Link
	}
}

// NewValueArray : 配列を生成
func NewValueArray() Value {
	return Value{
		Type:  Array,
		Value: NewTArray(),
	}
}

// NewValueArrayPtr : 配列を生成
func NewValueArrayPtr() *Value {
	v := NewValueArray()
	return &v
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
		v.Tag = 0
		return
	}
	// Copy Meta
	v.Type = value.Type
	v.Tag = value.Tag
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
	return v.Value.(*TArray).Items
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
		cv := NewValueStr(v.ToString())
		a := NewTArray()
		a.Append(&cv)
		v.Value = a
	}
	a := v.Value.(*TArray)
	alen := a.Append(val)
	return alen
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
		cv := NewValueStr(v.ToString())
		a := NewTArray()
		a.Append(&cv)
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
		return NewValueNullPtr()
	}
	var res *Value = nil
	// Clone basic data
	switch v.Type {
	case Int:
		res = NewValueIntPtr(v.ToInt())
	case Float:
		f := NewValueFloat(v.Value.(float64))
		res = &f
	case Str:
		return NewValueStrPtr(v.ToString())
	case Array:
		va := NewValueArray()
		a := va.Value.(*TArray)
		for _, v := range a.Items {
			va.Append(v.Clone())
		}
		res = &va
	case Hash:
		vh := NewValueHash()
		h := vh.Value.(THash)
		for key, val := range h {
			vh.HashSet(key, val.Clone())
		}
		res = &vh
	default:
		tmp := NewValueNull()
		tmp.Type = v.Type
		tmp.Value = v.Value
		res = &tmp
	}
	// clone other meta data
	res.Tag = v.Tag
	res.IsConst = v.IsConst
	return res
}
