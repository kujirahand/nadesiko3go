package value

import (
	"math"
)

// AddStr : 文字列同士の結合
func AddStr(l, r *Value) *Value {
	return NewStrPtr(l.ToString() + r.ToString())
}

// Add : 加算
func Add(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewStrPtr(l.ToString() + r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewFloatPtr(l.ToFloat() + r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewIntPtr(l.ToInt() + r.ToInt())
	}
	return NewNullPtr()
}

// Mul : かけ算
func Mul(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewIntPtr(l.ToInt() * r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewFloatPtr(l.ToFloat() * r.ToFloat())
	}
	// 文字列 * 回数
	if l.Type == Str || r.IsNumber() {
		s := ""
		for i := 0; i < int(r.ToInt()); i++ {
			s += l.ToString()
		}
		return NewStrPtr(s)
	}
	return NewNullPtr()
}

// Sub : 引き算
func Sub(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewIntPtr(l.ToInt() - r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewFloatPtr(l.ToFloat() - r.ToFloat())
	}
	return NewNullPtr()
}

// Div : 割り算
func Div(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewIntPtr(l.ToInt() / r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewFloatPtr(l.ToFloat() / r.ToFloat())
	}
	return NewNullPtr()
}

// Mod : 割り算の余り
func Mod(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewIntPtr(l.ToInt() % r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewFloatPtr(math.Mod(l.ToFloat(), r.ToFloat()))
	}
	return NewNullPtr()
}

// Exp : べき乗
func Exp(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	f := math.Pow(l.ToFloat(), r.ToFloat())
	return NewFloatPtr(f)
}

// EqEq : ==
func EqEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() == r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() == r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() == r.ToInt())
	}
	return NewBoolPtr(false)
}

// NtEq : !=
func NtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() != r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() != r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() != r.ToInt())
	}
	return NewBoolPtr(false)
}

// Gt : >
func Gt(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() > r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() > r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() > r.ToInt())
	}
	return NewBoolPtr(false)
}

// GtEq : >=
func GtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() >= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() >= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() >= r.ToInt())
	}
	return NewBoolPtr(false)
}

// Lt : <
func Lt(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() < r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() < r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() < r.ToInt())
	}
	return NewBoolPtr(false)
}

// LtEq : <=
func LtEq(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewBoolPtr(l.ToString() <= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewBoolPtr(l.ToFloat() <= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewBoolPtr(l.ToInt() <= r.ToInt())
	}
	return NewBoolPtr(false)
}

// And : And
func And(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	boolLeft := l.ToBool()
	if boolLeft == false {
		return NewBoolPtr(false)
	}
	boolRight := r.ToBool()
	if boolRight == false {
		return NewBoolPtr(false)
	}
	return NewBoolPtr(true)
}

// Or : Or
func Or(l, r *Value) *Value {
	if l == nil || r == nil {
		return NewNullPtr()
	}

	boolLeft := l.ToBool()
	boolRight := r.ToBool()
	return NewBoolPtr(boolLeft || boolRight)
}

// Not : Not
func Not(r *Value) *Value {
	if r == nil {
		return NewNullPtr()
	}

	return NewBoolPtr(!r.ToBool())
}
