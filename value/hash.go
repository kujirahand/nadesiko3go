package value

import (
	"fmt"
	"strings"
)

// THash : ハッシュ型の型
type THash map[string]*Value

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

// ToJSONStringFormat : to json string
func (p *THash) ToJSONStringFormat(level int) string {
	tab := ""
	for i := 0; i < level; i++ {
		tab += "  "
	}
	a := []string{}
	for key, val := range *p {
		sval := val.ToJSONStringFormat(level + 1)
		s := fmt.Sprintf(tab+"  \"%s\": %s", key, strings.TrimSpace(sval))
		a = append(a, s)
	}
	return tab + "{\n" + strings.Join(a, ",\n") + "\n" + tab + "}"
}

// Length : get value count
func (p *THash) Length() int {
	return len(*p)
}

// Keys : get keys
func (p *THash) Keys() []string {
	r := make([]string, 0, p.Length())
	for k := range *p {
		r = append(r, k)
	}
	return r
}

// Clear : Clear data
func (p *THash) Clear() {
	for k, v := range *p {
		Free(v)
		delete(*p, k)
	}
}
