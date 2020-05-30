package value

import (
	"strings"
)

// ToString : 文字列に変換
func (p *TArray) ToString() string {
	return p.ToJSONString()
}

// ToJSONString : To JSON String
func (p *TArray) ToJSONString() string {
	a := []string{}
	for _, val := range *p {
		a = append(a, val.ToJSONString())
	}
	return "[" + strings.Join(a, ",") + "]"
}

// Length : 配列の長さを返す
func (p *TArray) Length() int {
	return len(*p)
}

// Set : 配列に値を設定
func (p *TArray) Set(index int, val *Value) {
	// 要素を拡張
	for index >= len(*p) {
		*p = append(*p, NewValueNullPtr())
	}
	// 値を設定
	(*p)[index] = val
}

// Get : 配列の値を取得
func (p *TArray) Get(index int) *Value {
	if index >= len(*p) {
		return nil
	}
	return (*p)[index]
}

// SplitString : 文字列から配列を作る
func SplitString(src, splitter string) TArray {
	a := TArray{}
	sa := strings.Split(src, splitter)
	for _, v := range sa {
		vv := NewValueStr(v)
		a = append(a, &vv)
	}
	return a
}

// NewValueArrayFromStr : 文字列から配列型のValueを作る
func NewValueArrayFromStr(src, splitter string) Value {
	a := SplitString(src, splitter)
	nv := NewValueArray()
	nv.Value = a
	return nv
}
