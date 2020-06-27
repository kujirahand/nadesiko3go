package value

import (
	"math"
)

// AddStr : 文字列同士の結合
func AddStr(l, r *Value) *Value {
	return NewValueStrPtr(l.ToString() + r.ToString())
}

// Add : 加算
func Add(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueStrPtr(l.ToString() + r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueFloatPtr(l.ToFloat() + r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueIntPtr(l.ToInt() + r.ToInt())
	}
	return NewValueNullPtr()
}

// Mul : かけ算
func Mul(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueIntPtr(l.ToInt() * r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloatPtr(l.ToFloat() * r.ToFloat())
	}
	// 文字列 * 回数
	if l.Type == Str || r.IsNumber() {
		s := ""
		for i := 0; i < int(r.ToInt()); i++ {
			s += l.ToString()
		}
		return NewValueStrPtr(s)
	}
	return NewValueNullPtr()
}

// Sub : 引き算
func Sub(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueIntPtr(l.ToInt() - r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloatPtr(l.ToFloat() - r.ToFloat())
	}
	return NewValueNullPtr()
}

// Div : 割り算
func Div(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueIntPtr(l.ToInt() / r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloatPtr(l.ToFloat() / r.ToFloat())
	}
	return NewValueNullPtr()
}

// Mod : 割り算の余り
func Mod(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueIntPtr(l.ToInt() % r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloatPtr(math.Mod(l.ToFloat(), r.ToFloat()))
	}
	return NewValueNullPtr()
}

// Exp : べき乗
func Exp(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	f := math.Pow(l.ToFloat(), r.ToFloat())
	return NewValueFloatPtr(f)
}

// EqEq : ==
func EqEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() == r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() == r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() == r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// NtEq : !=
func NtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() != r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() != r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() != r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// Gt : >
func Gt(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() > r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() > r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() > r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// GtEq : >=
func GtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() >= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() >= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() >= r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// Lt : <
func Lt(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() < r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() < r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() < r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// LtEq : <=
func LtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBoolPtr(l.ToString() <= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBoolPtr(l.ToFloat() <= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBoolPtr(l.ToInt() <= r.ToInt())
	}
	return NewValueBoolPtr(false)
}

// And : And
func And(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	boolLeft := l.ToBool()
	if boolLeft == false {
		return NewValueBoolPtr(false)
	}
	boolRight := r.ToBool()
	if boolRight == false {
		return NewValueBoolPtr(false)
	}
	return NewValueBoolPtr(true)
}

// Or : Or
func Or(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewValueNullPtr()
	}

	boolLeft := l.ToBool()
	boolRight := r.ToBool()
	return NewValueBoolPtr(boolLeft || boolRight)
}

// Not : Not
func Not(r *Value) *Value {
	if r == nil {
		return NewValueNullPtr()
	}

	return NewValueBoolPtr(!r.ToBool())
}
