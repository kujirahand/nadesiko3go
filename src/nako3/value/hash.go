package value

type ValueHash struct {
	hash map[string]*Value
}

func NewValueHash() *ValueHash {
	p := ValueHash{}
	p.hash = map[string]*Value{}
	return &p
}

func (p *ValueHash) Set(key string, v Value) {
	p.hash[key] = &v
}

func (p *ValueHash) Get(key string) *Value {
	return p.hash[key]
}
