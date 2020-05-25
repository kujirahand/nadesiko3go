package value

import (
	"math"
)

// AddStr : 文字列同士の結合
func AddStr(l, r *Value) Value {
	return NewValueStr(l.ToString() + r.ToString())
}

// Add : 加算
func Add(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueStr(l.ToString() + r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueFloat(l.ToFloat() + r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueInt(l.ToInt() + r.ToInt())
	}
	return NewValueNull()
}

// Mul : かけ算
func Mul(l, r *Value) Value {
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueInt(l.ToInt() * r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloat(l.ToFloat() * r.ToFloat())
	}
	// 文字列 * 回数
	if l.Type == Str || r.IsNumber() {
		s := ""
		for i := 0; i < int(r.ToInt()); i++ {
			s += l.ToString()
		}
		return NewValueStr(s)
	}
	return NewValueNull()
}

// Sub : 引き算
func Sub(l, r *Value) Value {
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueInt(l.ToInt() - r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloat(l.ToFloat() - r.ToFloat())
	}
	return NewValueNull()
}

// Div : 割り算
func Div(l, r *Value) Value {
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueInt(l.ToInt() / r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloat(l.ToFloat() / r.ToFloat())
	}
	return NewValueNull()
}

// Mod : 割り算の余り
func Mod(l, r *Value) Value {
	// 整数同士
	if l.Type == Int && r.Type == Int {
		return NewValueInt(l.ToInt() % r.ToInt())
	}
	// 数値同士
	if l.IsNumber() && r.IsNumber() {
		return NewValueFloat(math.Mod(l.ToFloat(), r.ToFloat()))
	}
	return NewValueNull()
}

// Exp : べき乗
func Exp(l, r *Value) Value {
	f := math.Pow(l.ToFloat(), r.ToFloat())
	return NewValueFloat(f)
}

// EqEq : ==
func EqEq(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() == r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() == r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() == r.ToInt())
	}
	return NewValueBool(false)
}

// NtEq : !=
func NtEq(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() != r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() != r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() != r.ToInt())
	}
	return NewValueBool(false)
}

// Gt : >
func Gt(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() > r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() > r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() > r.ToInt())
	}
	return NewValueBool(false)
}

// GtEq : >=
func GtEq(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() >= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() >= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() >= r.ToInt())
	}
	return NewValueBool(false)
}

// Lt : <
func Lt(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() < r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() < r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() < r.ToInt())
	}
	return NewValueBool(false)
}

// LtEq : <=
func LtEq(l, r *Value) Value {
	// どちらかが文字列同士の計算
	if l.Type == Str || r.Type == Str {
		return NewValueBool(l.ToString() <= r.ToString())
	}
	// どちらかがFloatの計算
	if l.Type == Float || r.Type == Float {
		return NewValueBool(l.ToFloat() <= r.ToFloat())
	}
	// どちらかがIntの計算
	if l.Type == Int || r.Type == Int {
		return NewValueBool(l.ToInt() <= r.ToInt())
	}
	return NewValueBool(false)
}

// And : And
func And(l, r *Value) Value {
	boolLeft := l.ToBool()
	if boolLeft == false {
		return NewValueBool(false)
	}
	boolRight := r.ToBool()
	if boolRight == false {
		return NewValueBool(false)
	}
	return NewValueBool(true)
}

// Or : Or
func Or(l, r *Value) Value {
	boolLeft := l.ToBool()
	boolRight := r.ToBool()
	return NewValueBool(boolLeft || boolRight)
}
