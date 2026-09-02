// Package compiler turns the syntax tree into IR (AGENTS.md §6).
package compiler

import (
	"sort"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// The system values are addressed by LoadSpecial and StoreSpecial rather than
// by a variable slot, so no scope has to reserve room for them.

// funcCtx is the function currently being compiled.
type funcCtx struct {
	name    string
	code    []ir.Inst
	slots   map[string]int
	numVars int
	// constSlots marks the locals that hold a constant.
	constSlots map[int]bool
	// constLocals holds the value of the locals whose 『定数』 declaration is
	// on the straight line of this function, so a later use can be folded.
	constLocals map[int]value.Value
	// captureIndex names this function's captures, and captures says where
	// each one comes from in the enclosing function.
	captureIndex map[string]int
	captures     []ir.Capture
	// loops stacks the jump targets a 『抜ける』 or 『続ける』 needs.
	loops []*loopCtx
}

func newFuncCtx(name string) *funcCtx {
	return &funcCtx{
		name:         name,
		slots:        map[string]int{},
		constSlots:   map[int]bool{},
		constLocals:  map[int]value.Value{},
		captureIndex: map[string]int{},
	}
}

// refKind says where a name lives.
type refKind int

const (
	refSpecial refKind = iota // それ・対象など
	refLocal
	refCapture
	refGlobal
)

// varRef is a resolved name: where it lives, and at which index.
type varRef struct {
	kind    refKind
	index   int
	special ir.Special
	isConst bool
}

// loopCtx records where to jump out of, or back into, the enclosing loop.
type loopCtx struct {
	breaks    []int // 後で飛び先を埋める Jump のアドレス
	continues []int
}

// Compiler walks the tree and emits IR.
type Compiler struct {
	prog     ir.Program
	registry *stdlib.Registry
	fn       *funcCtx
	fnStack  []*funcCtx
	// userFuncs maps a function name to its index in prog.Funcs.
	userFuncs map[string]int
	// constIndex dedupes the constant pool.
	constIndex map[ir.Const]int
	posIndex   map[ir.SourcePos]int
	// globalIndex maps a global's name to its slot, and constGlobals marks
	// the ones that hold a constant.
	globalIndex  map[string]int
	constGlobals map[int]bool
	// constGlobalValues holds the value of the module 『定数』 declared on the
	// straight line of main. → fold.go
	constGlobalValues map[int]value.Value
	// branchDepth counts the conditionals and loops enclosing the statement
	// being compiled. A 『定数』 declared inside one may never run, so only a
	// declaration at depth 0 is recorded for propagation.
	branchDepth int
	file        string
}

// compileError carries a failure out of the recursive walk.
type compileError struct{ err *errs.NakoError }

// Compile turns a parsed program into IR.
func Compile(tree *ast.Node, filename string, registry *stdlib.Registry) (prog *ir.Program, err error) {
	c := &Compiler{
		registry:          registry,
		userFuncs:         map[string]int{},
		constIndex:        map[ir.Const]int{},
		posIndex:          map[ir.SourcePos]int{},
		globalIndex:       map[string]int{},
		constGlobals:      map[int]bool{},
		constGlobalValues: map[int]value.Value{},
		file:              filename,
	}
	c.prog.Version = ir.CurrentVersion
	c.prog.Sources = []ir.SourceFile{{Name: filename}}

	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			prog, err = nil, ce.err
		}
	}()

	// 先に関数定義を集める。相互再帰する関数どうしが互いを呼べるようにするため。
	c.declareFuncs(tree)

	main := newFuncCtx("main")
	c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: "main"})
	c.prog.Main = len(c.prog.Funcs) - 1
	mainIndex := c.prog.Main

	c.fn = main
	// 『それ』の初期値は空文字列。未定義ではない。
	c.emit(ir.OpLoadConst, c.constString(""), 0, tree)
	c.emit(ir.OpStoreSpecial, int(ir.SpecialSore), 0, tree)
	c.compileBlockValue(tree)
	c.emit(ir.OpReturn, 0, 0, tree)
	mainCode := optimize(main.code)
	c.prog.Funcs[mainIndex].Code = mainCode
	c.prog.Funcs[mainIndex].NumVars = main.numVars
	c.prog.Funcs[mainIndex].MaxStack = c.maxStack(mainIndex, mainCode)
	c.prog.Funcs[mainIndex].ConstVars = sortedSlots(main.constSlots)
	c.prog.ConstGlobals = sortedSlots(c.constGlobals)

	if err := c.prog.Validate(); err != nil {
		return nil, err
	}
	return &c.prog, nil
}

func (c *Compiler) fail(msg string, n *ast.Node) {
	panic(compileError{&errs.NakoError{
		Kind: errs.Runtime, File: c.fileOf(n), Line: c.lineOf(n), Msg: msg,
	}})
}

// failConst reports an assignment to a constant, with the message
// nako_gen.mts gives.
func (c *Compiler) failConst(name, reason string, n *ast.Node) {
	panic(compileError{&errs.NakoError{
		Kind: errs.Syntax, File: c.fileOf(n), Line: c.lineOf(n),
		Msg: "定数『" + name + "』" + reason,
	}})
}

func (c *Compiler) fileOf(n *ast.Node) string {
	if n != nil && n.File != "" {
		return n.File
	}
	return c.file
}

func (c *Compiler) lineOf(n *ast.Node) int {
	if n != nil {
		return n.Line
	}
	return 0
}

// --- 出力の補助 ---

func (c *Compiler) emit(op ir.Op, a, b int, n *ast.Node) int {
	c.fn.code = append(c.fn.code, ir.Inst{Op: op, A: a, B: b, Pos: c.pos(n)})
	return len(c.fn.code) - 1
}

// pos interns a source position so instructions can reference it by index.
func (c *Compiler) pos(n *ast.Node) int {
	p := ir.SourcePos{Source: 0}
	if n != nil {
		p.Line, p.Column, p.Offset = n.Line, n.Column, n.Offset
	}
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Column < 0 {
		p.Column = 0
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if i, ok := c.posIndex[p]; ok {
		return i
	}
	c.prog.Positions = append(c.prog.Positions, p)
	i := len(c.prog.Positions) - 1
	c.posIndex[p] = i
	return i
}

// constant interns a value in the constant pool.
func (c *Compiler) constant(k ir.Const) int {
	if i, ok := c.constIndex[k]; ok {
		return i
	}
	c.prog.Consts = append(c.prog.Consts, k)
	i := len(c.prog.Consts) - 1
	c.constIndex[k] = i
	return i
}

func (c *Compiler) constString(s string) int {
	return c.constant(ir.Const{Kind: ir.ConstString, Str: s})
}

func (c *Compiler) constNumber(n float64) int {
	return c.constant(ir.Const{Kind: ir.ConstNumber, Num: n})
}

// globalSlot returns the slot of a global, allocating one the first time the
// name is used.
func (c *Compiler) globalSlot(name string) int {
	if i, ok := c.globalIndex[name]; ok {
		return i
	}
	c.prog.Globals = append(c.prog.Globals, name)
	i := len(c.prog.Globals) - 1
	c.globalIndex[name] = i
	return i
}

// maxStack reports the deepest the operand stack gets, using the same walk the
// verifier uses so that the two cannot drift apart.
func (c *Compiler) maxStack(index int, code []ir.Inst) int {
	depth, err := ir.ComputeMaxStack(index, ir.Func{Code: code})
	if err != nil {
		c.fail(err.Error(), nil)
	}
	return depth
}

// patch fills in the jump target of an instruction emitted earlier.
func (c *Compiler) patch(at int, target int) { c.fn.code[at].A = target }

func (c *Compiler) here() int { return len(c.fn.code) }

// --- 変数 ---

// slot returns the local slot of a name, allocating one if this function has
// not used it yet.
func (c *Compiler) slot(name string) int {
	if i, ok := c.fn.slots[name]; ok {
		return i
	}
	i := c.fn.numVars
	c.fn.slots[name] = i
	c.fn.numVars++
	return i
}

// isLocal reports whether a name already has a slot in this function.
func (c *Compiler) isLocal(name string) bool {
	_, ok := c.fn.slots[name]
	return ok
}

// resolve finds where a name lives, capturing it from an enclosing function
// when a nested function refers to it.
func (c *Compiler) resolve(name string) varRef {
	if special, ok := ir.SpecialByName(name); ok {
		return varRef{kind: refSpecial, special: special}
	}
	if slot, ok := c.fn.slots[name]; ok {
		return varRef{kind: refLocal, index: slot, isConst: c.fn.constSlots[slot]}
	}
	if index, ok := c.captureInto(len(c.fnStack), name); ok {
		return varRef{kind: refCapture, index: index}
	}
	slot := c.globalSlot(name)
	return varRef{kind: refGlobal, index: slot, isConst: c.constGlobals[slot]}
}

// captureInto makes name available in the function at the given depth of the
// nesting chain, threading a capture through every level in between.
//
// Depth len(fnStack) is the function being compiled; 0 is the outermost. The
// outermost is main, whose variables are module globals, so the search stops
// before it.
func (c *Compiler) captureInto(depth int, name string) (int, bool) {
	fn := c.funcAt(depth)
	if index, ok := fn.captureIndex[name]; ok {
		return index, true
	}
	if depth <= 1 {
		return 0, false // main の変数はグローバルなので捕捉しない
	}

	parent := c.funcAt(depth - 1)
	capture := ir.Capture{}
	if slot, ok := parent.slots[name]; ok {
		capture.FromParent = slot
	} else {
		// 親も持っていなければ、親に捕捉させてからそれを引き継ぐ。
		// 二段以上入れ子になった関数はこの経路で外側へ届く。
		parentIndex, ok := c.captureInto(depth-1, name)
		if !ok {
			return 0, false
		}
		capture.FromParent = parentIndex
		capture.ParentIsCapture = true
	}
	index := len(fn.captures)
	fn.captures = append(fn.captures, capture)
	fn.captureIndex[name] = index
	return index, true
}

// funcAt returns the function at a depth of the nesting chain.
func (c *Compiler) funcAt(depth int) *funcCtx {
	if depth >= len(c.fnStack) {
		return c.fn
	}
	return c.fnStack[depth]
}

// loadVar pushes the value of a name.
func (c *Compiler) loadVar(name string, n *ast.Node) {
	switch ref := c.resolve(name); ref.kind {
	case refSpecial:
		c.emit(ir.OpLoadSpecial, int(ref.special), 0, n)
	case refLocal:
		c.emit(ir.OpLoadLocal, ref.index, 0, n)
	case refCapture:
		c.emit(ir.OpLoadCapture, ref.index, 0, n)
	default:
		c.emit(ir.OpLoadGlobal, ref.index, 0, n)
	}
}

// storeVar pops a value into a name. Assigning to a constant is refused here,
// before any IR is emitted for it.
func (c *Compiler) storeVar(name string, n *ast.Node) {
	ref := c.resolve(name)
	if ref.isConst {
		c.failConst(name, "は既に定義済みなので、値を代入することはできません。", n)
	}
	switch ref.kind {
	case refSpecial:
		c.emit(ir.OpStoreSpecial, int(ref.special), 0, n)
	case refLocal:
		c.emit(ir.OpStoreLocal, ref.index, 0, n)
	case refCapture:
		c.emit(ir.OpStoreCapture, ref.index, 0, n)
	default:
		c.emit(ir.OpStoreGlobal, ref.index, 0, n)
	}
}

// declareConst marks a name as a constant and emits its one-time
// initialization.
func (c *Compiler) declareConst(name string, n *ast.Node) {
	if ref := c.resolve(name); ref.isConst {
		c.failConst(name, "は既に定義済みなので、値を代入することはできません。", n)
	}
	if c.fn.name != "main" {
		slot := c.slot(name)
		c.fn.constSlots[slot] = true
		c.emit(ir.OpInitLocal, slot, 0, n)
		return
	}
	slot := c.globalSlot(name)
	c.constGlobals[slot] = true
	c.emit(ir.OpInitGlobal, slot, 0, n)
}

// checkWritable refuses a constant where a variable has to be written, such as
// a loop variable.
func (c *Compiler) checkWritable(name, reason string, n *ast.Node) {
	if name == "" {
		return
	}
	if ref := c.resolve(name); ref.isConst {
		c.failConst(name, reason, n)
	}
}

// sortedSlots turns a set of slot numbers into the ascending list the IR holds.
func sortedSlots(set map[int]bool) []int {
	if len(set) == 0 {
		return nil
	}
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
