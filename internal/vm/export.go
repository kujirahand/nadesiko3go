package vm

// The gogen backend generates Go source whose functions replace bytecode
// (AGENTS.md §12): SetNative (vm.go) tells callClosure to run one instead of
// interpreting Funcs[index].Code. Everything that same source needs to call —
// operators, indexing, stdlib dispatch, the error-monitor recover — has to
// come from here rather than being reimplemented, or the two backends could
// silently drift apart. Every method below is a thin pass-through to the
// unexported one the interpreter already uses, so there is exactly one
// implementation of each piece of behaviour.
//
// Generated code does not import this package directly — it goes through
// pkg/runtime, which is outside internal/ and so can be imported from a
// generated program built as its own module.

import (
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/ops"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// ConstValue is OpLoadConst's value.
func (m *VM) ConstValue(i int) value.Value { return m.constValue(i) }

// Binary is OpBinary.
func (m *VM) Binary(op ir.BinaryOp, a, b value.Value) value.Value { return ops.Binary(op, a, b) }

// Unary is OpUnary.
func (m *VM) Unary(op ir.UnaryOp, v value.Value) value.Value { return ops.Unary(op, v) }

// IndexGet is OpIndexGet.
func (m *VM) IndexGet(container value.Value, indexes []value.Value, pos int) value.Value {
	return m.indexGet(container, indexes, pos)
}

// IndexSet is OpIndexSet.
func (m *VM) IndexSet(container value.Value, indexes []value.Value, v value.Value, pos int) value.Value {
	return m.indexSet(container, indexes, v, pos)
}

// IterKeys is OpIterKeys.
func (m *VM) IterKeys(v value.Value) value.Value { return m.iterKeys(v) }

// LenOf is OpLen.
func (m *VM) LenOf(v value.Value) value.Value {
	arr, ok := v.Array()
	if !ok {
		return value.Number(0)
	}
	return value.Number(float64(arr.Len()))
}

// MakeArray is OpMakeArray.
func (m *VM) MakeArray(items []value.Value) value.Value {
	return value.ArrayValue(value.NewArray(items...))
}

// MakeDict is OpMakeDict. items alternates key, value, key, value…
func (m *VM) MakeDict(items []value.Value) value.Value {
	d := value.NewDict()
	for i := 0; i+1 < len(items); i += 2 {
		d.Set(value.ToString(items[i]), items[i+1])
	}
	return value.DictValue(d)
}

// MakeClosure is OpMakeFunc, for a generated function that has locals and
// captures but no *frame to read them from.
func (m *VM) MakeClosure(index int, locals, captures []*value.Cell) *value.Func {
	return m.makeClosureFrom(index, locals, captures)
}

// CallStd is OpCallStd.
func (m *VM) CallStd(id int, args []value.Value, pos int) value.Value {
	return m.callStd(id, args, pos)
}

// CallUser is OpCallUser: a plain call, no captured cells.
func (m *VM) CallUser(index int, args []value.Value) value.Value { return m.call(index, args) }

// CallDynamic is OpCallValue: the callee is a value that must hold a function.
func (m *VM) CallDynamic(callee value.Value, args []value.Value, pos int) value.Value {
	fnRef, ok := callee.Func()
	if !ok {
		m.failAt("関数ではない値を呼び出そうとしました。", pos)
	}
	return m.callClosure(fnRef.ID, fnRef.Captured, args)
}

// DefaultSpecials returns a copy of the default system values.
func (m *VM) DefaultSpecials() [ir.SpecialCount]value.Value {
	return m.specials
}

// SpecialValue reads a system value.
func (m *VM) SpecialValue(id ir.Special) value.Value {
	if !id.Valid() {
		return value.Undefined()
	}
	if id.IsFrameSpecial() && m.current != nil {
		return m.current.specials[id]
	}
	return m.specials[id]
}

// SetSpecialValue writes a system value.
func (m *VM) SetSpecialValue(id ir.Special, v value.Value) {
	if !id.Valid() {
		return
	}
	if id.IsFrameSpecial() && m.current != nil {
		m.current.specials[id] = v
		return
	}
	m.specials[id] = v
}

// GlobalCell gives direct access to a module variable's cell, so generated
// code can Get/Set/Init it exactly as a local or captured cell.
func (m *VM) GlobalCell(i int) *value.Cell { return m.globals[i] }

// Globals hands over the whole slice, which a generated function binds once
// on entry so that reading a module variable is a slice index rather than a
// method call per access.
func (m *VM) Globals() []*value.Cell { return m.globals }

// StoreCell is OpStoreLocal / OpStoreCapture / OpStoreGlobal: write a cell,
// failing the way an assignment to a constant does.
func (m *VM) StoreCell(cell *value.Cell, v value.Value, pos int) {
	m.setCell(cell, v, pos)
}

// InitCell is OpInitLocal / OpInitGlobal: write a constant cell the one time
// it is allowed to be written.
func (m *VM) InitCell(cell *value.Cell, v value.Value, pos int) {
	m.initCell(cell, v, pos)
}

// Fail raises a runtime error the way any instruction does, unwinding to the
// nearest エラー監視 in this call or, with none open, out of the program.
func (m *VM) Fail(msg string, pos int) { m.failAt(msg, pos) }

// Recover tells whether a recovered panic is a nadesiko runtime error (an
// エラー監視 candidate) and, if so, its message. Anything else must be
// re-panicked — it is a real bug, not a language-level error.
//
// A generated function cannot type-assert this itself: nakoPanic is
// unexported, on purpose, so that only vm code can manufacture one.
func (m *VM) Recover(r any) (msg string, isNako bool) {
	np, ok := r.(nakoPanic)
	if !ok {
		return "", false
	}
	return np.err.Msg, true
}
