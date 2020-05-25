package value

import (
	"fmt"
	"strings"
)

// Set : THash.Set
func (p *THash) Set(key string, v *Value) {
	h := *p
	h[key] = v
}

// Get : THash.Get
func (p *THash) Get(key string) *Value {
	h := *p
	return h[key]
}

// ToString : to string
func (p *THash) ToString() string {
	return p.ToJSONString()
}

// ToJSONString : to json string
func (p *THash) ToJSONString() string {
	a := []string{}
	for key, val := range *p {
		s := fmt.Sprintf("\"%s\":%s", key, val.ToJSONString())
		a = append(a, s)
	}
	return "{" + strings.Join(a, ",") + "}"
}

// Length : get value count
func (p *THash) Length() int {
	return len(*p)
}
