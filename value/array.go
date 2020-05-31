package value

import (
	"strings"
)

// NewTArray : TArrayを生成
func NewTArray() TArray {
	return TArray{
		Items:  []*Value{},
		length: 0,
	}
}

// ToString : 文字列に変換
func (p *TArray) ToString() string {
	return p.ToJSONString()
}

// ToJSONString : To JSON String
func (p *TArray) ToJSONString() string {
	a := []string{}
	for _, val := range p.Items {
		a = append(a, val.ToJSONString())
	}
	return "[" + strings.Join(a, ",") + "]"
}

// Length : 配列の長さを返す
func (p *TArray) Length() int {
	return p.length
}

// Set : 配列に値を設定
func (p *TArray) Set(index int, val *Value) {
	// 要素を拡張
	for index >= p.length {
		p.Items = append(p.Items, &Value{Type: Null})
		p.length++
	}
	// 値を設定
	p.Items[index] = val
}

// Append : 配列を追加
func (p *TArray) Append(val *Value) {
	p.Items = append(p.Items, val)
	p.length++
}

// Get : 配列の値を取得
func (p *TArray) Get(index int) *Value {
	if index >= p.length {
		return nil
	}
	return p.Items[index]
}

// SplitString : 文字列から配列を作る
func SplitString(src, splitter string) TArray {
	a := NewTArray()
	sa := strings.Split(src, splitter)
	for _, v := range sa {
		a.Append(NewValueStrPtr(v))
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
