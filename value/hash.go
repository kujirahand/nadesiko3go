package value

import (
	"fmt"
	"strings"
)

func NewValueHashObj() *ValueHash {
	p := ValueHash{}
	return &p
}

func (p *ValueHash) Set(key string, v *Value) {
	h := *p
	h[key] = v
}

func (p *ValueHash) Get(key string) *Value {
	h := *p
	return h[key]
}

func (p *ValueHash) ToString() string {
	return p.ToJSONString()
}

func (p *ValueHash) ToJSONString() string {
	a := []string{}
	for key, val := range *p {
		s := fmt.Sprintf("\"%s\":%s", key, val.ToJSONString())
		a = append(a, s)
	}
	return "{" + strings.Join(a, ",") + "}"
}

func (p *ValueHash) Length() int {
	return len(*p)
}
