// Package ir defines the versioned, serializable boundary between compiler and
// execution backends.
package ir

const CurrentVersion = 1

type Program struct {
	Version int     `json:"version"`
	Consts  []Const `json:"consts"`
	Funcs   []Func  `json:"funcs"`
	Main    int     `json:"main"`
	// Globals names each global slot. LoadGlobal and StoreGlobal address them
	// by index, so the name is only needed to report a value back by name.
	Globals   []string     `json:"globals"`
	Sources   []SourceFile `json:"sources"`
	Positions []SourcePos  `json:"positions"`
}

type ConstKind uint8

const (
	ConstUndefined ConstKind = iota
	ConstNull
	ConstBool
	ConstNumber
	ConstString
)

type Const struct {
	Kind ConstKind `json:"kind"`
	Bool bool      `json:"bool,omitempty"`
	Num  float64   `json:"num,omitempty"`
	Str  string    `json:"str,omitempty"`
}

type Func struct {
	Name    string  `json:"name"`
	Params  []Param `json:"params"`
	NumVars int     `json:"numVars"`
	Code    []Inst  `json:"code"`
	Async   bool    `json:"async"`
	Pure    bool    `json:"pure"`
	// Captures lists the enclosing function's variables this one closes over.
	Captures []Capture `json:"captures,omitempty"`
	// MaxStack is the deepest the operand stack gets in this function. The
	// validator recomputes it, so a mismatch means the IR is inconsistent.
	MaxStack int `json:"maxStack"`
}

// Capture threads one variable from the enclosing function into a nested one.
// The two share the same storage, so an assignment on either side is visible
// to the other — which is what makes a counter closure work.
type Capture struct {
	FromParent int `json:"fromParent"` // 外側の関数のスロット番号
	ToSlot     int `json:"toSlot"`     // 内側の関数のスロット番号
}

type Param struct {
	Name      string   `json:"name"`
	Particles []string `json:"particles"`
	// Slot is the local the caller puts this argument in. Arguments never
	// reach the body through the operand stack, so a function body always
	// starts with an empty one (docs/vm.md §3.3).
	Slot int `json:"slot"`
}

type Inst struct {
	Op  Op  `json:"op"`
	A   int `json:"a"`
	B   int `json:"b"`
	Pos int `json:"pos"`
}

type SourceFile struct {
	Name string `json:"name"`
}

type SourcePos struct {
	Source int `json:"source"`
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}
