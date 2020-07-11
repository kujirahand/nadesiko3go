package scope

import (
	"github.com/kujirahand/nadesiko3go/value"
)

const (
	// DefRegCap : レジスタ初期サイズ
	DefRegCap = 256
)

// Scope : Scope
type Scope struct {
	// values : slice of Value
	values *value.TArray
	// names : link to Values
	names map[string]int
	// Reg : レジスタ
	Reg *value.TArray
	// Index : レジスタ末尾管理用
	Index int
	// Level : スコープのレベル
	Level int
}

// NewScope : Create Scope
func NewScope() *Scope {
	s := Scope{
		values: value.NewTArray(),
		names:  map[string]int{},
		Reg:    value.NewTArray(),
		Index:  0,
	}
	return &s
}

// Get : Get Variable
func (s *Scope) Get(key string) *value.Value {
	i, ok := s.names[key]
	if ok {
		return s.values.Get(i)
	}
	return nil
}

// Set : Set Variable
func (s *Scope) Set(key string, v *value.Value) int {
	i, ok := s.names[key]
	if ok {
		s.values.Set(i, v)
		return i
	}
	index := s.Length()
	s.names[key] = index
	s.values.Append(v)
	return index
}

// GetByIndex : Get Value By Index
func (s *Scope) GetByIndex(index int) *value.Value {
	return s.values.Get(index)
}

// SetByIndex : Set Value
func (s *Scope) SetByIndex(index int, val *value.Value) {
	for index >= s.values.Length() {
		s.values.Append(value.NewNullPtr())
	}
	s.values.Set(index, val)
}

// GetNameByIndex : Get Value By Index
func (s *Scope) GetNameByIndex(index int) string {
	for i, v := range s.names {
		if v == index {
			return i
		}
	}
	return ""
}

// GetIndexByName : Get Value Index By Name
func (s *Scope) GetIndexByName(name string) int {
	i, ok := s.names[name]
	if ok {
		return i
	}
	return -1
}

// Length : Get var count
func (s *Scope) Length() int {
	return s.values.Length()
}

// ToStringRegs : To string
func (s *Scope) ToStringRegs() string {
	res := s.Reg.ToJSONString()
	return res
}

// GetHash : 変数と値をハッシュ形式で得る(但しパフォーマンスを考慮していないのでGet/Setメソッドで使うこと)
func (s *Scope) GetHash() value.THash {
	h := value.THash{}
	for name, i := range s.names {
		h[name] = s.values.Get(i)
	}
	return h
}

// ToStringValues : Get reg
func (s *Scope) ToStringValues() string {
	h := s.GetHash()
	return h.ToJSONString()
}

// TScopeList : Scope Object
type TScopeList struct {
	Items []*Scope
}

// NewScopeList : Create ScopeObj
func NewScopeList() TScopeList {
	p := TScopeList{}
	p.Items = []*Scope{}
	p.Open() // make global scope
	return p
}

// GetGlobal : Get Global
func (p *TScopeList) GetGlobal() *Scope {
	return p.Items[0]
}

// Open : Open Scope
func (p *TScopeList) Open() *Scope {
	s := NewScope()
	s.Level = len(p.Items)
	p.Items = append(p.Items, s)
	return s
}

// Close : Close Scope
func (p *TScopeList) Close() *Scope {
	s := p.Items[len(p.Items)-1]
	p.Items = p.Items[0 : len(p.Items)-1]
	return s
}

// Find : Find
func (p *TScopeList) Find(key string) (*value.Value, int) {
	i := len(p.Items) - 1
	for i >= 0 {
		scope := p.Items[i]
		v := scope.Get(key)
		if v != nil {
			return v, i
		}
		i--
	}
	return nil, -1
}

// Get : Get Variable's Value
func (p *TScopeList) Get(key string) *value.Value {
	v, _ := p.Find(key)
	return v
}

// GetTopScope : Get Top Scope
func (p *TScopeList) GetTopScope() *Scope {
	return p.Items[len(p.Items)-1]
}

// SetTopVars : SetTopVars
func (p *TScopeList) SetTopVars(key string, v *value.Value) {
	scope := p.GetTopScope()
	scope.Set(key, v)
}

// IsTopGlobal : Only Global?
func (p *TScopeList) IsTopGlobal() bool {
	return (len(p.Items) == 1)
}
