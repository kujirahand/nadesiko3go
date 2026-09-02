// Package ir defines the versioned, serializable boundary between compiler and
// execution backends.
package ir

// CurrentVersion は、直列化されたIRの世代。バージョン2で Inst.C と
// OpBinaryAt (スーパー命令) が加わった。
const CurrentVersion = 2

type Program struct {
	Version int     `json:"version"`
	Consts  []Const `json:"consts"`
	Funcs   []Func  `json:"funcs"`
	Main    int     `json:"main"`
	// Globals names each global slot. LoadGlobal and StoreGlobal address them
	// by index, so the name is only needed to report a value back by name.
	Globals []string `json:"globals"`
	// ConstGlobals lists the global slots that hold a constant. They are
	// written once with InitGlobal and refuse an ordinary store afterwards.
	ConstGlobals []int        `json:"constGlobals,omitempty"`
	Sources      []SourceFile `json:"sources"`
	Positions    []SourcePos  `json:"positions"`
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
	// ConstVars lists the local slots that hold a constant.
	ConstVars []int `json:"constVars,omitempty"`
	// NumCaptures is how many cells this function closes over. Captures are
	// addressed separately from locals so that the two cannot be confused.
	NumCaptures int    `json:"numCaptures,omitempty"`
	Code        []Inst `json:"code"`
	Async       bool   `json:"async"`
	Pure        bool   `json:"pure"`
	// Captures lists the enclosing function's variables this one closes over.
	Captures []Capture `json:"captures,omitempty"`
	// MaxStack is the deepest the operand stack gets in this function. The
	// validator recomputes it, so a mismatch means the IR is inconsistent.
	MaxStack int `json:"maxStack"`
}

// Capture threads one variable from the enclosing function into a nested one.
// The two share the same cell, so an assignment on either side is visible to
// the other — which is what makes a counter closure work.
//
// Its position in Func.Captures is the index the nested function uses.
type Capture struct {
	// FromParent is where to take the cell from in the enclosing function.
	FromParent int `json:"fromParent"`
	// ParentIsCapture says whether that index is one of the enclosing
	// function's own captures rather than one of its locals. A function
	// nested two deep reaches an outer variable this way.
	ParentIsCapture bool `json:"parentIsCapture,omitempty"`
}

type Param struct {
	Name      string   `json:"name"`
	Particles []string `json:"particles"`
	// Slot is the local the caller puts this argument in. Arguments never
	// reach the body through the operand stack, so a function body always
	// starts with an empty one (docs/vm.md §3.3).
	Slot int `json:"slot"`
}

// Inst is one instruction. Most instructions use A alone; B carries a count
// (→ Op のコメント). C is used only by the fused instructions the peephole pass
// makes, which need one operand more than A と B に収まる。
type Inst struct {
	Op  Op  `json:"op"`
	A   int `json:"a"`
	B   int `json:"b"`
	C   int `json:"c,omitempty"`
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
