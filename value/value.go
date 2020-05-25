package value

import (
	"strconv"
	"strings"
)

// ValueType : valueの型
type ValueType int

const (
	Null ValueType = iota
	Int
	Float
	Str
	Bool
	Array
	Hash
	Function
	UserFunc
)

// Value : Value
type Value struct {
	Type  ValueType
	Value interface{}
	Tag   int
}

type ValueArray []Value

type ValueFunc func(args ValueArray) (*Value, error)

func NewValueNull() Value {
	return Value{Type: Null, Value: nil}
}
func NewValueInt(v int64) Value {
	return Value{Type: Int, Value: v}
}
func NewValueFloat(v float64) Value {
	return Value{Type: Float, Value: v}
}
func NewValueStr(v string) Value {
	return Value{Type: Str, Value: v}
}
func NewValueBool(v bool) Value {
	return Value{Type: Bool, Value: v}
}
func NewValueFunc(v ValueFunc) Value {
	return Value{Type: Function, Value: v}
}
func NewValueUserFunc(v int) Value {
	return Value{Type: UserFunc, Value: v}
}

func StrToInt(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}
func StrToFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func NewValue(vtype ValueType, s string) Value {
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

func (v *Value) ToBool() bool {
	switch v.Type {
	case Int:
		i := v.Value.(int64)
		return (i != 0)
	case Float:
		i := int64(v.Value.(float64))
		return (i != 0)
	case Str:
		s := v.Value.(string)
		return (s != "")
	case Bool:
		return v.Value.(bool)
	}
	return false
}

func (v *Value) ToInt() int64 {
	switch v.Type {
	case Int:
		return v.Value.(int64)
	case Float:
		return int64(v.Value.(float64))
	case Str:
		return StrToInt(v.Value.(string))
	case Bool:
		if v.Value.(bool) {
			return 1
		}
	}
	return 0
}

func (v *Value) ToFloat() float64 {
	switch v.Type {
	case Int:
		return float64(v.Value.(int64))
	case Float:
		return v.Value.(float64)
	case Str:
		return StrToFloat(v.Value.(string))
	case Bool:
		if v.Value.(bool) {
			return 1
		}
	}
	return 0
}

func (v *Value) IsNumber() bool {
	switch v.Type {
	case Int:
		return true
	case Float:
		return true
	}
	return false
}

func (v *Value) SetInt(value int64) {
	v.Type = Int
	v.Value = value
}
func (v *Value) SetFloat(value float64) {
	v.Type = Float
	v.Value = value
}
func (v *Value) SetStr(value string) {
	v.Type = Str
	v.Value = value
}
func (v *Value) SetBool(value bool) {
	v.Type = Bool
	v.Value = value
}

func (v *Value) SetValue(value *Value) {
	if value == nil {
		v.Type = Null
		v.Value = nil
		v.Tag = 0
		return
	}
	v.Type = value.Type
	v.Value = value.Value
	v.Tag = value.Tag
}

// IsFuncType : 関数タイプか
func (v *Value) IsFunction() bool {
	return v.Type == Function || v.Type == UserFunc
}

func (v *Value) ToString() string {
	switch v.Type {
	case Int:
		return IntToStr(v.Value.(int64))
	case Float:
		return FloatToStr(v.Value.(float64))
	case Str:
		return v.Value.(string)
	case Bool:
		if true {
			return "真"
		}
		return "偽"
	case Array:
		a := v.Value.(ValueArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(ValueHash)
		return h.ToString()
	}
	return ""
}

// ToJSONString : Convert to JSON string
func (v *Value) ToJSONString() string {
	switch v.Type {
	case Int:
		return IntToStr(v.Value.(int64))
	case Float:
		return FloatToStr(v.Value.(float64))
	case Str:
		return "\"" + EncodeStrToJSON(v.Value.(string)) + "\""
	case Bool:
		if true {
			return "true"
		}
		return "false"
	case Array:
		a := v.Value.(ValueArray)
		return a.ToJSONString()
	case Hash:
		h := v.Value.(ValueHash)
		return h.ToString()
	}
	return ""
}
