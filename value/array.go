package value

import (
	"strings"
)

// NewTArray : TArrayを生成
func NewTArray() *TArray {
	return &TArray{
		items: TArrayItems{},
	}
}

// NewTArrayFromStrings : TArrayを生成
func NewTArrayFromStrings(s []string) *TArray {
	p := NewTArray()
	for _, v := range s {
		p.Append(NewValueStrPtr(v))
	}
	return p
}

// NewTArrayDef : TArrayを生成
func NewTArrayDef(items TArrayItems) TArray {
	return TArray{
		items: items,
	}
}

// GetItems : スライスを返す
func (p *TArray) GetItems() TArrayItems {
	return p.items
}

// Clear : Clear Data
func (p *TArray) Clear() {
	for _, v := range p.items {
		if v != nil {
			Free(v)
		}
	}
	p.items = TArrayItems{}
}

// ToString : 文字列に変換
func (p *TArray) ToString() string {
	return p.ToJSONString()
}

// ToJSONString : To JSON String
func (p *TArray) ToJSONString() string {
	a := []string{}
	for _, val := range p.items {
		a = append(a, val.ToJSONString())
	}
	return "[" + strings.Join(a, ",") + "]"
}

// ToJSONStringFormat : To JSON String
func (p *TArray) ToJSONStringFormat(level int) string {
	tab := ""
	for i := 0; i < level; i++ {
		tab += "  "
	}
	a := []string{}
	allsimple := true
	for _, val := range p.items {
		a = append(a, tab+val.ToJSONStringFormat(level+1))
		if !val.IsSimpleValue() {
			allsimple = false
		}
	}
	res := tab + "[\n" + strings.Join(a, ",\n") + "\n" + tab + "]"
	// simple?
	if len(res) < 60 && allsimple {
		return tab + p.ToJSONString()
	}
	return res
}

// Length : 配列の長さを返す
func (p *TArray) Length() int {
	return len(p.items)
}

// Set : 配列に値を設定
func (p *TArray) Set(index int, val *Value) {
	// 要素を拡張
	for index >= p.Length() {
		p.items = append(p.items, NewValueNullPtr())
	}
	// 値を設定
	p.items[index] = val
}

// Append : 配列を追加
func (p *TArray) Append(val *Value) {
	p.items = append(p.items, val)
}

// Push : 配列を追加 (Appendのエイリアス)
func (p *TArray) Push(val *Value) {
	p.Append(val)
}

// Get : 配列の値を取得
func (p *TArray) Get(index int) *Value {
	if index >= p.Length() {
		// panic("TArray.Get.index" + IntToStr(index) + "/" + IntToStr(p.length))
		return nil
	}
	return p.items[index]
}

// Pop : 末尾のデータを取り出す
func (p *TArray) Pop() *Value {
	plen := p.Length()
	if plen == 0 {
		return nil
	}
	result := p.items[plen-1]
	p.items = p.items[:plen-1]
	return result
}

// SplitString : 文字列から配列を作る
func SplitString(src, splitter string) *TArray {
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
