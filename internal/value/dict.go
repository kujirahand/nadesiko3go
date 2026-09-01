package value

// Dict preserves insertion order independently of Go map iteration order.
type Dict struct {
	keys   []string
	values map[string]*Value
}

func NewDict() *Dict {
	return &Dict{values: make(map[string]*Value)}
}

func (d *Dict) Len() int { return len(d.keys) }

func (d *Dict) Set(key string, v Value) {
	if d.values == nil {
		d.values = make(map[string]*Value)
	}
	if existing, exists := d.values[key]; exists {
		*existing = v
		return
	} else {
		d.keys = append(d.keys, key)
	}
	stored := v
	d.values[key] = &stored
}

func (d *Dict) Get(key string) (Value, bool) {
	v, ok := d.values[key]
	if !ok {
		return Undefined(), false
	}
	return *v, true
}

func (d *Dict) Delete(key string) bool {
	if _, exists := d.values[key]; !exists {
		return false
	}
	delete(d.values, key)
	for i, existing := range d.keys {
		if existing == key {
			d.keys = append(d.keys[:i], d.keys[i+1:]...)
			break
		}
	}
	return true
}

func (d *Dict) Keys() []string {
	return append([]string(nil), d.keys...)
}
