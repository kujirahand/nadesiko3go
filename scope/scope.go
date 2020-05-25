package scope

import "github.com/kujirahand/nadesiko3go/value"

// Scope : Scope
type Scope struct {
	Vars *value.THash
}

// NewScope : Create Scope
func NewScope() *Scope {
	s := Scope{}
	s.Vars = &value.THash{}
	return &s
}

// Get : Get Variable
func (s *Scope) Get(key string) *value.Value {
	return s.Vars.Get(key)
}

// Set : Set Variable
func (s *Scope) Set(key string, v *value.Value) {
	s.Vars.Set(key, v)
}

// TScopeList : Scope Object
type TScopeList struct {
	Items []*Scope
}

// NewScopeList : Create ScopeObj
func NewScopeList() *TScopeList {
	p := TScopeList{}
	p.Items = []*Scope{}
	p.Open() // make global scope
	return &p
}

// GetGlobal : Get Global
func (p *TScopeList) GetGlobal() *Scope {
	return p.Items[0]

}

// Open : Open Scope
func (p *TScopeList) Open() {
	s := NewScope()
	p.Items = append(p.Items, s)
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
		v := scope.Vars.Get(key)
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
