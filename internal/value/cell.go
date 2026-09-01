package value

// Cell is one variable's storage.
//
// Variables live in cells rather than in a plain slice so that a nested
// function can share one with the frame that created it: that sharing is what
// makes a closure see later assignments (AGENTS.md §6, docs/vm.md §3.2).
//
// A constant's cell is not mutable. It accepts exactly one Init and refuses
// every Set afterwards, including through a closure that captured it.
type Cell struct {
	Value       Value
	Mutable     bool
	Initialized bool
}

// NewCell creates a cell holding undefined.
func NewCell(mutable bool) *Cell {
	return &Cell{Value: Undefined(), Mutable: mutable}
}

// Get reads the cell.
func (c *Cell) Get() Value { return c.Value }

// Set writes a variable cell. It reports false for a constant, which the VM
// turns into a broken-IR error rather than a language error: the compiler is
// supposed to have refused the assignment already.
func (c *Cell) Set(v Value) bool {
	if !c.Mutable {
		return false
	}
	c.Value = v
	c.Initialized = true
	return true
}

// Init writes a constant cell for the first time. It reports false on a second
// attempt.
func (c *Cell) Init(v Value) bool {
	if c.Initialized {
		return false
	}
	c.Value = v
	c.Initialized = true
	return true
}
