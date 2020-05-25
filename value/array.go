package value

import (
	"strings"
)

// ToString : 文字列に変換
func (p *ValueArray) ToString() string {
	return p.ToJSONString()
}

// ToJSONString : To JSON String
func (p *ValueArray) ToJSONString() string {
	a := []string{}
	for _, val := range *p {
		a = append(a, val.ToJSONString())
	}
	return "[" + strings.Join(a, ",") + "]"
}

// Length : 配列の長さを返す
func (p *ValueArray) Length() int {
	return len(*p)
}
