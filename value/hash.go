package value

import (
	"fmt"
	"strings"
)

// ValueHash : ValueHash struct
type ValueHash struct {
	hash map[string]*Value
}

func NewValueHash() *ValueHash {
	p := ValueHash{}
	p.hash = map[string]*Value{}
	return &p
}

func (p *ValueHash) Set(key string, v *Value) {
	p.hash[key] = v
}

func (p *ValueHash) Get(key string) *Value {
	return p.hash[key]
}

func (p *ValueHash) ToString() string {
	return p.ToJSONString()
}

func (p *ValueHash) ToJSONString() string {
	a := []string{}
	for key, val := range p.hash {
		s := fmt.Sprintf("\"%s\":%s", key, val.ToJSONString())
		a = append(a, s)
	}
	return "{" + strings.Join(a, ",") + "}"
}

func (p *ValueHash) Length() int {
	return len(p.hash)
}
