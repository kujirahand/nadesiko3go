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
