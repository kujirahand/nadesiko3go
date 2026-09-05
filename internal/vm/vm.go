// Package vm executes IR on a value stack (AGENTS.md §6).
package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/event"
	"github.com/kujirahand/nadesiko3go/internal/host"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Host is what the VM needs from its environment. The compat runner supplies
// an implementation that only collects output; the CUI supplies the real one.
type Host interface {
	// Print writes one line of program output.
	Print(s string)
	// Write writes program output without ending the line, which is what a
	// prompt needs.
	Write(s string)
	// ReadLine reads one line of input, for 『尋ねる』.
	ReadLine() (string, error)
	// Exit ends the program with a status.
	Exit(code int)
	// Args reports the arguments the program was started with.
	Args() []string
	// ReadResource reads a file packed into the executable, if there is one.
	ReadResource(name string) ([]byte, bool)
	// Now supplies the initial event-loop clock.
	Now() time.Time
}

// VM runs one program.
type VM struct {
	prog     *ir.Program
	registry *stdlib.Registry
	host     Host

	// globals holds the module variables, indexed by the slot the IR
	// addresses them with.
	globals []*value.Cell
	// specials holds the system values that belong to the program as a whole.
	// 『それ』 is not here: it belongs to the running frame.
	specials [ir.SpecialCount]value.Value
	// globalIndex finds a global by name, for the commands that reach one
	// through stdlib.Context and for reporting values back.
	globalIndex map[string]int
	// funcValues gives every function index a stable value identity, so that
	// two references to the same function compare equal.
	funcValues   map[int]*value.Func
	commandState map[string]value.Value

	// loop orders the timer callbacks and owns the virtual clock.
	loop *event.Loop
	// callbacks maps a scheduled callback to the function and arguments it runs.
	callbacks    map[host.CallbackID]queuedCallback
	nextCallback host.CallbackID

	current            *frame
	pendingTimerTarget float64
	bokanPath          string
	currentPos         int

	// isLeaf flags functions that do not create closures or capture variables,
	// allowing their frames and cells to be reused.
	isLeaf     []bool
	leafFrames []*frame
	// blockEnds gives each function's basic-block start the exclusive end PC.
	// The interpreter charges the whole block at once instead of checking the
	// instruction limit after every dispatch.
	blockEnds [][]int32

	// depth counts the nested calls, and executed counts the instructions
	// run, so that a broken program stops instead of hanging.
	depth    int
	executed uint64
	options  Options

	// natives holds a Go-compiled stand-in for Funcs[index], for the gogen
	// backend (AGENTS.md §12). callClosure calls it instead of interpreting
	// bytecode, but everything around the call — depth limit, capture
	// wiring, argument binding — stays exactly as it is for every other
	// function, so the two backends cannot drift apart on that part.
	natives map[int]NativeFunc
}

// NativeFunc is a function body the gogen backend compiled to Go, standing in
// for a bytecode Func. It receives the same locals, captures, and frame-local
// system values callClosure gives the interpreter. Sharing the specials array
// with the frame keeps generated LoadSpecial/StoreSpecial instructions and
// stdlib Context.SysVar/SetSysVar calls on the same storage.
type NativeFunc func(m *VM, locals, captures []*value.Cell, specials *[ir.SpecialCount]value.Value) value.Value

// SetNative registers fn as Funcs[index]'s implementation. Call it once per
// function before running the program; an index without one still runs its
// bytecode normally, so a program can mix generated and interpreted functions
// (or have none registered at all, which is the plain VM).
func (m *VM) SetNative(index int, fn NativeFunc) {
	if m.natives == nil {
		m.natives = map[int]NativeFunc{}
	}
	m.natives[index] = fn
}

// Options bounds what one run may do. Without them a runaway program would
// hang the whole compat run rather than failing one case.
type Options struct {
	// MaxFrames bounds how deep calls may nest.
	MaxFrames int
	// MaxInstructions bounds how many instructions one run may execute.
	MaxInstructions uint64
	// MaxCallbacks bounds how many timer callbacks one run may dispatch.
	MaxCallbacks int
	// DrainPendingCallbacks runs callbacks left in the event queue after main
	// returns. Normal programs enable it; DocTest disables it to match cnako's
	// synchronous documentation runner.
	DrainPendingCallbacks bool
	// RealSleep makes Wait pause for the real time using time.Sleep,
	// which is what interactive CUI programs need. Tests and compat runners
	// leave it false so that runs stay instant and deterministic.
	RealSleep bool
}

// DefaultOptions is generous enough for a program that means it, and small
// enough that a mistake fails quickly.
func DefaultOptions() Options {
	return Options{
		MaxFrames:             10000,
		MaxInstructions:       200_000_000,
		MaxCallbacks:          event.DefaultMaxCallbacks,
		DrainPendingCallbacks: true,
	}
}

// New prepares a VM for a program. The loop starts at a fixed instant so that
// two runs of the same program see the same clock.
func New(prog *ir.Program, registry *stdlib.Registry, h Host, options Options) *VM {
	loop := event.New(h.Now())
	loop.MaxCallbacks = options.MaxCallbacks

	constGlobals := map[int]bool{}
	for _, slot := range prog.ConstGlobals {
		constGlobals[slot] = true
	}
	globals := make([]*value.Cell, len(prog.Globals))
	index := make(map[string]int, len(prog.Globals))
	bokan := computeBokanPath(prog.Sources)
	for i, name := range prog.Globals {
		index[name] = i
		globals[i] = value.NewCell(!constGlobals[i])
		// システム定数を持つ名前は、その値から始める
		if v, ok := registry.Const(name); ok {
			globals[i].Value = v
		}
		if name == "名前空間" && len(prog.Sources) > 0 {
			globals[i].Value = value.String(filepath.Base(prog.Sources[0].Name))
		}
		if name == "母艦パス" {
			globals[i].Value = value.String(bokan)
		}
	}

	isLeaf := make([]bool, len(prog.Funcs))
	blockEnds := make([][]int32, len(prog.Funcs))
	for i := range prog.Funcs {
		fn := &prog.Funcs[i]
		blockEnds[i] = findBlockEnds(fn.Code)
		if fn.NumCaptures > 0 {
			continue
		}
		hasMakeFunc := false
		for _, inst := range fn.Code {
			if inst.Op == ir.OpMakeFunc {
				hasMakeFunc = true
				break
			}
		}
		isLeaf[i] = !hasMakeFunc
	}

	m := &VM{
		prog:         prog,
		registry:     registry,
		host:         h,
		globals:      globals,
		globalIndex:  index,
		funcValues:   map[int]*value.Func{},
		commandState: map[string]value.Value{},
		loop:         loop,
		callbacks:    map[host.CallbackID]queuedCallback{},
		isLeaf:       isLeaf,
		blockEnds:    blockEnds,
		options:      options,
		bokanPath:    bokan,
		currentPos:   -1,
	}
	// システム値の初期値。『それ』は空文字列、残りはstdlibの定数に従う。
	for id := ir.Special(0); id < ir.SpecialCount; id++ {
		if v, ok := registry.Const(ir.SpecialNames[id]); ok {
			m.specials[id] = v
			continue
		}
		m.specials[id] = value.String("")
	}
	return m
}

// startTime is where the virtual clock begins. A fixed instant keeps a program
// that prints the time reproducible.
var startTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Vars reads back the final value of the named variables. Names are looked up
// with the prefix first, then bare, so that both module variables and system
// variables can be inspected.
func (m *VM) Vars(prefix string, names []string) map[string]value.Value {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]value.Value, len(names))
	for _, name := range names {
		if i, ok := m.globalIndex[prefix+name]; ok {
			out[name] = m.globals[i].Get()
			continue
		}
		if i, ok := m.globalIndex[name]; ok {
			out[name] = m.globals[i].Get()
			continue
		}
		if id, ok := ir.SpecialByName(name); ok {
			out[name] = m.specials[id]
			continue
		}
		out[name] = value.Undefined()
	}
	return out
}

// nakoPanic carries a nadesiko runtime error out of the interpreter loop.
type nakoPanic struct{ err *errs.NakoError }

// Run executes the program's entry function.
func (m *VM) Run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			np, ok := r.(nakoPanic)
			if !ok {
				panic(r)
			}
			if np.err.Msg == exitMessage {
				err = nil // 『終了』で止めたので、エラーではない
				return
			}
			err = np.err
		}
	}()
	m.call(m.prog.Main, nil)
	// main が終わった後に残っている単発のコールバックを流す
	if !m.options.DrainPendingCallbacks {
		return nil
	}
	return m.runPendingCallbacks()
}

// frame is one function activation.
//
// Locals and captures are separate index spaces, so that the verifier can
// check each on its own and a captured cell cannot be mistaken for a local.
// Both hold cells rather than plain values, because a nested function shares
// a cell with the frame that created it.
type frame struct {
	fn        *ir.Func
	blockEnds []int32
	locals    []*value.Cell
	captures  []*value.Cell
	stack     []value.Value
	// specials holds the system values that belong to this activation:
	// 『それ』, 『対象』, 『対象キー』, 『回数』, 『エラーメッセージ』.
	specials [ir.SpecialCount]value.Value
	// handlers stacks the error-monitored regions this frame has entered.
	handlers []handler
	// cells holds the backing storage for local cells when reused by leaf frames.
	cells []value.Cell
}

// handler remembers where to resume when a region raises.
type handler struct {
	target     int // 飛び先
	stackDepth int // 例外時にスタックを戻す深さ
}

type queuedCallback struct {
	fn      *value.Func
	args    []value.Value
	isTimer bool
	timerID host.CallbackID
}

func (f *frame) push(v value.Value) { f.stack = append(f.stack, v) }

func (f *frame) pop() value.Value {
	if len(f.stack) == 0 {
		return value.Undefined()
	}
	v := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	return v
}

// popN takes the top n values in the order they were pushed.
func (f *frame) popN(n int) []value.Value {
	if n <= 0 {
		return nil
	}
	if n > len(f.stack) {
		n = len(f.stack)
	}
	out := make([]value.Value, n)
	copy(out, f.stack[len(f.stack)-n:])
	f.stack = f.stack[:len(f.stack)-n]
	return out
}

// borrowN is popN without the copy: the result points into this frame's own
// stack, which stays untouched while a call runs (the callee gets its own
// frame). Only for arguments to a なでしこ function — callClosure copies each
// one into a cell straight away and keeps nothing.
//
// A stdlib command must keep using popN: an Impl may hold on to the slice it
// is given (『配列作成』 hands it to value.NewArray, which does not copy), and
// the next thing pushed onto this stack would then change a value it kept.
func (f *frame) borrowN(n int) []value.Value {
	if n <= 0 {
		return nil
	}
	if n > len(f.stack) {
		n = len(f.stack)
	}
	at := len(f.stack) - n
	out := f.stack[at:len(f.stack):len(f.stack)]
	f.stack = f.stack[:at]
	return out
}

// call runs a named function with no captured variables.
func (m *VM) call(index int, args []value.Value) value.Value {
	return m.callClosure(index, nil, args)
}

const defaultFrameVarCap = 8
const defaultFrameStackCap = 8

func (m *VM) allocLeafFrame(fn *ir.Func, blockEnds []int32, specials *[ir.SpecialCount]value.Value, stackNeed int) *frame {
	var f *frame
	n := len(m.leafFrames)
	if n > 0 {
		f = m.leafFrames[n-1]
		m.leafFrames = m.leafFrames[:n-1]
		f.fn = fn
		f.blockEnds = blockEnds
		f.specials = *specials

		if cap(f.stack) < stackNeed {
			sz := stackNeed
			if sz < defaultFrameStackCap {
				sz = defaultFrameStackCap
			}
			f.stack = make([]value.Value, 0, sz)
		} else {
			f.stack = f.stack[:0]
		}

		if cap(f.cells) < fn.NumVars {
			sz := fn.NumVars
			if sz < defaultFrameVarCap {
				sz = defaultFrameVarCap
			}
			f.cells = make([]value.Cell, sz)
			f.locals = make([]*value.Cell, sz)
		}
		f.cells = f.cells[:fn.NumVars]
		f.locals = f.locals[:fn.NumVars]
	} else {
		var stack []value.Value
		if stackNeed > 0 {
			stackCap := stackNeed
			if stackCap < defaultFrameStackCap {
				stackCap = defaultFrameStackCap
			}
			stack = make([]value.Value, 0, stackCap)
		}
		varCap := fn.NumVars
		if varCap < defaultFrameVarCap {
			varCap = defaultFrameVarCap
		}
		f = &frame{
			fn:        fn,
			blockEnds: blockEnds,
			specials:  *specials,
			stack:     stack,
			cells:     make([]value.Cell, varCap),
			locals:    make([]*value.Cell, varCap),
		}
		f.cells = f.cells[:fn.NumVars]
		f.locals = f.locals[:fn.NumVars]
	}

	for i := 0; i < fn.NumVars; i++ {
		f.cells[i].Value = value.Undefined()
		f.cells[i].Mutable = true
		f.cells[i].Initialized = false
		f.locals[i] = &f.cells[i]
	}
	for _, slot := range fn.ConstVars {
		if slot >= 0 && slot < fn.NumVars {
			f.cells[slot].Mutable = false
		}
	}

	f.handlers = f.handlers[:0]
	return f
}

func (m *VM) freeLeafFrame(f *frame) {
	// pop/borrowN shorten the slice without clearing the removed slots. Clear
	// the whole backing array so returned values and intermediate objects do not
	// stay reachable for as long as the frame remains in the pool. A frame that
	// only ever ran gogen native functions has no backing array at all
	// (callClosure asks for no operand stack), so this costs it nothing.
	clear(f.stack[:cap(f.stack)])
	f.stack = f.stack[:0]
	for i := range f.cells {
		f.cells[i].Value = value.Undefined()
	}
	// Frame-local system values can also hold arrays, dictionaries, and
	// functions. None of them belong to an idle pooled frame.
	clear(f.specials[:])
	m.leafFrames = append(m.leafFrames, f)
}

// callClosure runs one function and returns its value. captured holds the
// cells the closure shares with the frame that created it.
//
// Leaf functions (NumCaptures == 0 && no OpMakeFunc) reuse their frames,
// stacks, locals, and cells from a pool, achieving zero allocations on recursive
// and repeated calls.
func (m *VM) callClosure(index int, captured []*value.Cell, args []value.Value) value.Value {
	if index < 0 || index >= len(m.prog.Funcs) {
		m.fail(fmt.Sprintf("関数の呼び出し先がありません: %d", index), ir.SourcePos{})
	}
	m.depth++
	if m.options.MaxFrames > 0 && m.depth > m.options.MaxFrames {
		m.depth--
		m.fail("関数の呼び出しが深すぎます。再帰が止まらなくなっていませんか。", ir.SourcePos{})
	}

	fn := &m.prog.Funcs[index]
	// A callee starts with the caller's dynamic system values, but owns a
	// copy so changes made by the callee cannot leak back into the caller.
	// The array is 6 values wide, so it is passed by pointer and copied
	// exactly once, into the new frame.
	specials := &m.specials
	if m.current != nil {
		specials = &m.current.specials
	}

	// A gogen native function holds its operands in Go variables, so its
	// frame needs no operand stack — neither allocated nor, on the way back
	// to the pool, cleared.
	native, isNative := m.natives[index]
	stackNeed := fn.MaxStack
	if isNative {
		stackNeed = 0
	}

	isLeaf := index < len(m.isLeaf) && m.isLeaf[index]
	var f *frame
	if isLeaf {
		f = m.allocLeafFrame(fn, m.blockEnds[index], specials, stackNeed)
	} else {
		f = &frame{
			fn:        fn,
			blockEnds: m.blockEnds[index],
			specials:  *specials,
		}
		// 深さは命令列から分かっている。伸ばしながら積み直さない。
		if stackNeed > 0 {
			f.stack = make([]value.Value, 0, stackNeed)
		}
		f.locals = make([]*value.Cell, fn.NumVars)
		if fn.NumVars > 0 {
			cells := make([]value.Cell, fn.NumVars)
			for i := range cells {
				cells[i].Value = value.Undefined()
				cells[i].Mutable = true
				f.locals[i] = &cells[i]
			}
			for _, slot := range fn.ConstVars {
				if slot >= 0 && slot < len(cells) {
					cells[slot].Mutable = false
				}
			}
		}
		if fn.NumCaptures > 0 {
			f.captures = make([]*value.Cell, fn.NumCaptures)
			for i := range f.captures {
				if i < len(captured) && captured[i] != nil {
					f.captures[i] = captured[i]
					continue
				}
				f.captures[i] = value.NewCell(true)
			}
		}
	}

	f.specials[ir.SpecialSore] = value.String("")
	if m.pendingTimerTarget != 0 {
		f.specials[ir.SpecialTarget] = value.Number(m.pendingTimerTarget)
		m.pendingTimerTarget = 0
	}
	prev := m.current
	m.current = f
	defer func() {
		if isLeaf {
			m.freeLeafFrame(f)
		}
		m.depth--
		m.current = prev
	}()

	// 引数はスタックではなくスロットへ直接入れる。関数本体は空のスタックで
	// 始まるので、検証器が入口の深さを0と決められる。
	for i, p := range fn.Params {
		if i >= len(args) {
			break
		}
		if p.Slot >= 0 && p.Slot < len(f.locals) {
			f.locals[p.Slot].Value = args[i]
			f.locals[p.Slot].Initialized = true
		}
	}

	var ret value.Value
	if isNative {
		ret = native(m, f.locals, f.captures, &f.specials)
	} else {
		ret = m.run(f)
	}
	return ret
}

func computeBokanPath(sources []ir.SourceFile) string {
	if len(sources) > 0 && sources[0].Name != "" && sources[0].Name != "main.nako3" && sources[0].Name != "gui.nako3" && sources[0].Name != "-" {
		dir := filepath.Dir(sources[0].Name)
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
		return dir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}
