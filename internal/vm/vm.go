// Package vm executes IR on a value stack (AGENTS.md §6).
package vm

import (
	"fmt"
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

	// depth counts the nested calls, and executed counts the instructions
	// run, so that a broken program stops instead of hanging.
	depth    int
	executed uint64
	options  Options
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
		options:      options,
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
	fn       *ir.Func
	locals   []*value.Cell
	captures []*value.Cell
	stack    []value.Value
	// sore is 『それ』, which belongs to this activation.
	sore value.Value
	// handlers stacks the error-monitored regions this frame has entered.
	handlers []handler
}

// handler remembers where to resume when a region raises.
type handler struct {
	target     int // 飛び先
	stackDepth int // 例外時にスタックを戻す深さ
}

type queuedCallback struct {
	fn   *value.Func
	args []value.Value
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

// call runs a named function with no captured variables.
func (m *VM) call(index int, args []value.Value) value.Value {
	return m.callClosure(index, nil, args)
}

// callClosure runs one function and returns its value. captured holds the
// cells the closure shares with the frame that created it.
func (m *VM) callClosure(index int, captured []*value.Cell, args []value.Value) value.Value {
	if index < 0 || index >= len(m.prog.Funcs) {
		m.fail(fmt.Sprintf("関数の呼び出し先がありません: %d", index), ir.SourcePos{})
	}
	m.depth++
	if m.options.MaxFrames > 0 && m.depth > m.options.MaxFrames {
		m.depth--
		m.fail("関数の呼び出しが深すぎます。再帰が止まらなくなっていませんか。", ir.SourcePos{})
	}
	defer func() { m.depth-- }()

	fn := &m.prog.Funcs[index]
	constVars := map[int]bool{}
	for _, slot := range fn.ConstVars {
		constVars[slot] = true
	}
	f := &frame{fn: fn, sore: value.String("")}
	f.locals = make([]*value.Cell, fn.NumVars)
	for i := range f.locals {
		f.locals[i] = value.NewCell(!constVars[i])
	}
	// 捕捉したセルは、外側のフレームと同じものを指す
	f.captures = make([]*value.Cell, fn.NumCaptures)
	for i := range f.captures {
		if i < len(captured) && captured[i] != nil {
			f.captures[i] = captured[i]
			continue
		}
		f.captures[i] = value.NewCell(true)
	}
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
	return m.run(f)
}
