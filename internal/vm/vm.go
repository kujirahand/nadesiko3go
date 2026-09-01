// Package vm executes IR on a value stack (AGENTS.md §6).
package vm

import (
	"fmt"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/event"
	"github.com/kujirahand/nadesiko3go/internal/host"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Host is what the VM needs from its environment. The compat runner supplies an
// implementation that only collects output.
type Host interface {
	Print(s string)
}

// VM runs one program.
type VM struct {
	prog     *ir.Program
	registry *stdlib.Registry
	host     Host

	// globals holds module variables and the system variables the loops set.
	globals map[string]value.Value
	// funcValues gives every function index a stable value identity, so that
	// two references to the same function compare equal.
	funcValues map[int]*value.Func

	// loop orders the timer callbacks and owns the virtual clock.
	loop *event.Loop
	// callbacks maps a scheduled callback to the function it runs.
	callbacks    map[host.CallbackID]*value.Func
	nextCallback host.CallbackID

	// depth guards against unbounded recursion.
	depth int
}

// MaxDepth bounds how deep user function calls may nest before the VM reports
// a runtime error instead of exhausting the Go stack.
const MaxDepth = 10000

// New prepares a VM for a program. The loop starts at a fixed instant so that
// two runs of the same program see the same clock.
func New(prog *ir.Program, registry *stdlib.Registry, h Host) *VM {
	return &VM{
		prog:       prog,
		registry:   registry,
		host:       h,
		globals:    map[string]value.Value{},
		funcValues: map[int]*value.Func{},
		loop:       event.New(startTime),
		callbacks:  map[host.CallbackID]*value.Func{},
	}
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
		if v, ok := m.globals[prefix+name]; ok {
			out[name] = v
			continue
		}
		if v, ok := m.globals[name]; ok {
			out[name] = v
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
			err = np.err
		}
	}()
	m.call(m.prog.Main, nil)
	// main が終わった後に残っている単発のコールバックを流す
	return m.runPendingCallbacks()
}

// frame is one function activation.
//
// Slots are cells rather than plain values so that a nested function can share
// one with the frame that created it. That sharing is what makes a closure
// see later assignments to a captured variable.
type frame struct {
	fn    *ir.Func
	slots []*value.Value
	stack []value.Value
	// handlers stacks the error-monitored regions this frame has entered.
	handlers []handler
}

// handler remembers where to resume when a region raises.
type handler struct {
	target     int // 飛び先
	stackDepth int // 例外時にスタックを戻す深さ
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
func (m *VM) callClosure(index int, captured []*value.Value, args []value.Value) value.Value {
	if index < 0 || index >= len(m.prog.Funcs) {
		m.fail(fmt.Sprintf("関数の呼び出し先がありません: %d", index), ir.SourcePos{})
	}
	m.depth++
	if m.depth > MaxDepth {
		m.depth--
		m.fail("関数の呼び出しが深すぎます。再帰が止まらなくなっていませんか。", ir.SourcePos{})
	}
	defer func() { m.depth-- }()

	fn := &m.prog.Funcs[index]
	f := &frame{fn: fn, slots: make([]*value.Value, fn.NumVars)}
	for i := range f.slots {
		v := value.Undefined()
		f.slots[i] = &v
	}
	// 捕捉した変数は、外側のフレームと同じセルを指す
	for i, cap := range fn.Captures {
		if i < len(captured) && cap.ToSlot >= 0 && cap.ToSlot < len(f.slots) {
			f.slots[cap.ToSlot] = captured[i]
		}
	}
	// 引数は左から積まれた形で渡される。関数本体が順に取り出す。
	f.stack = append(f.stack, args...)
	return m.run(f)
}
