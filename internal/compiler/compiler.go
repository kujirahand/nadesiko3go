// Package compiler turns the syntax tree into IR (AGENTS.md §6).
package compiler

import (
	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// SoreSlot is the local slot every function reserves for 『それ』. Most commands
// assign their result to it, and the loops set it to the current item.
const SoreSlot = 0

// System variable names. Unlike 『それ』 these are shared by every scope,
// because the TypeScript version keeps them in the system variable table.
const (
	SysTarget    = "対象"
	SysTargetKey = "対象キー"
	SysCount     = "回数"
)

// funcCtx is the function currently being compiled.
type funcCtx struct {
	name    string
	code    []ir.Inst
	slots   map[string]int
	numVars int
	// loops stacks the jump targets a 『抜ける』 or 『続ける』 needs.
	loops []*loopCtx
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
	file       string
}

// compileError carries a failure out of the recursive walk.
type compileError struct{ err *errs.NakoError }

// Compile turns a parsed program into IR.
func Compile(tree *ast.Node, filename string, registry *stdlib.Registry) (prog *ir.Program, err error) {
	c := &Compiler{
		registry:   registry,
		userFuncs:  map[string]int{},
		constIndex: map[ir.Const]int{},
		posIndex:   map[ir.SourcePos]int{},
		file:       filename,
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

	main := &funcCtx{name: "main", slots: map[string]int{"それ": SoreSlot}, numVars: 1}
	c.prog.Funcs = append(c.prog.Funcs, ir.Func{Name: "main"})
	c.prog.Main = len(c.prog.Funcs) - 1
	mainIndex := c.prog.Main

	c.fn = main
	c.compileBlockValue(tree)
	c.emit(ir.OpReturn, 0, 0, tree)
	c.prog.Funcs[mainIndex].Code = main.code
	c.prog.Funcs[mainIndex].NumVars = main.numVars

	if err := c.prog.Validate(); err != nil {
		return nil, err
	}
	return &c.prog, nil
}

func (c *Compiler) fail(msg string, n *ast.Node) {
	sm := ast.SourceMap{File: c.file}
	if n != nil {
		sm = n.SourceMap
		if sm.File == "" {
			sm.File = c.file
		}
	}
	panic(compileError{&errs.NakoError{Kind: errs.Runtime, File: sm.File, Line: sm.Line, Msg: msg}})
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

// loadVar pushes the value of a name.
//
// A name the parser resolved to a module-qualified global, a system constant,
// or a system variable is not a local. Everything else is.
func (c *Compiler) loadVar(name string, n *ast.Node) {
	if c.isLocal(name) {
		c.emit(ir.OpLoadLocal, c.fn.slots[name], 0, n)
		return
	}
	c.emit(ir.OpLoadGlobal, c.constString(name), 0, n)
}

// storeVar pops a value into a name.
func (c *Compiler) storeVar(name string, n *ast.Node) {
	if c.isLocal(name) {
		c.emit(ir.OpStoreLocal, c.fn.slots[name], 0, n)
		return
	}
	c.emit(ir.OpStoreGlobal, c.constString(name), 0, n)
}
