package vm

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// run executes a frame, resuming at an error handler when a runtime error is
// raised inside an error-monitored region.
//
// The resume lives here rather than in the interpreter loop because the error
// can come from anywhere: an instruction, a command, or a nested call.
func (m *VM) run(f *frame) value.Value {
	pc := 0
	for {
		result, handled, target := m.protect(f, pc)
		if !handled {
			return result
		}
		pc = target
	}
}

// protect runs the loop from pc. When a runtime error escapes and this frame
// has an error-monitored region open, it reports where to resume instead of
// letting the error through.
func (m *VM) protect(f *frame, pc int) (result value.Value, handled bool, target int) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		np, isNako := r.(nakoPanic)
		if !isNako || len(f.handlers) == 0 {
			panic(r)
		}
		h := f.handlers[len(f.handlers)-1]
		f.handlers = f.handlers[:len(f.handlers)-1]
		f.stack = f.stack[:h.stackDepth]
		// 『エラーメッセージ』で捕まえた内容を読めるようにする
		m.storeGlobalByName("エラーメッセージ", value.String(np.err.Msg))
		result, handled, target = value.Undefined(), true, h.target
	}()
	return m.execute(f, pc), false, 0
}

// execute is the interpreter loop for one frame.
func (m *VM) execute(f *frame, pc int) value.Value {
	for pc < len(f.fn.Code) {
		m.executed++
		if m.options.MaxInstructions > 0 && m.executed > m.options.MaxInstructions {
			m.failAt("実行した命令が多すぎます。終わらない繰り返しになっていませんか。", f.fn.Code[pc].Pos)
		}
		inst := f.fn.Code[pc]
		pc++

		switch inst.Op {
		case ir.OpNop:

		case ir.OpLoadConst:
			f.push(m.constValue(inst.A))

		case ir.OpPop:
			f.pop()

		case ir.OpDup:
			v := f.pop()
			f.push(v)
			f.push(v)

		case ir.OpLoadLocal:
			f.push(f.locals[inst.A].Get())

		case ir.OpStoreLocal:
			m.setCell(f.locals[inst.A], f.pop(), inst)

		case ir.OpInitLocal:
			m.initCell(f.locals[inst.A], f.pop(), inst)

		case ir.OpLoadCapture:
			f.push(f.captures[inst.A].Get())

		case ir.OpStoreCapture:
			m.setCell(f.captures[inst.A], f.pop(), inst)

		case ir.OpLoadGlobal:
			f.push(m.globals[inst.A].Get())

		case ir.OpStoreGlobal:
			m.setCell(m.globals[inst.A], f.pop(), inst)

		case ir.OpInitGlobal:
			m.initCell(m.globals[inst.A], f.pop(), inst)

		case ir.OpLoadSpecial:
			f.push(m.loadSpecial(f, ir.Special(inst.A)))

		case ir.OpStoreSpecial:
			m.storeSpecial(f, ir.Special(inst.A), f.pop())

		case ir.OpBinary:
			b := f.pop()
			a := f.pop()
			f.push(m.binary(ir.BinaryOp(inst.A), a, b))

		case ir.OpUnary:
			f.push(m.unary(ir.UnaryOp(inst.A), f.pop()))

		case ir.OpMakeArray:
			f.push(value.ArrayValue(value.NewArray(f.popN(inst.B)...)))

		case ir.OpMakeDict:
			items := f.popN(inst.B * 2)
			d := value.NewDict()
			for i := 0; i+1 < len(items); i += 2 {
				d.Set(value.ToString(items[i]), items[i+1])
			}
			f.push(value.DictValue(d))

		case ir.OpIndexGet:
			indexes := f.popN(inst.B)
			container := f.pop()
			f.push(m.indexGet(container, indexes, inst.Pos))

		case ir.OpIndexSet:
			v := f.pop()
			indexes := f.popN(inst.B)
			container := f.pop()
			f.push(m.indexSet(container, indexes, v, inst.Pos))

		case ir.OpIterKeys:
			f.push(m.iterKeys(f.pop()))

		case ir.OpLen:
			arr, ok := f.pop().Array()
			if !ok {
				f.push(value.Number(0))
				break
			}
			f.push(value.Number(float64(arr.Len())))

		case ir.OpMakeFunc:
			f.push(value.FuncValue(m.makeClosure(inst.A, f)))

		case ir.OpCallStd:
			args := f.popN(inst.B)
			f.push(m.callStd(inst.A, args, inst.Pos))

		case ir.OpCallUser:
			args := f.popN(inst.B)
			f.push(m.call(inst.A, args))

		case ir.OpCallValue:
			args := f.popN(inst.B)
			callee := f.pop()
			fnRef, ok := callee.Func()
			if !ok {
				m.failAt("関数ではない値を呼び出そうとしました。", inst.Pos)
			}
			f.push(m.callClosure(fnRef.ID, fnRef.Captured, args))

		case ir.OpJump:
			pc = inst.A

		case ir.OpJumpIfFalse:
			if !value.ToBool(f.pop()) {
				pc = inst.A
			}

		case ir.OpJumpIfTrue:
			if value.ToBool(f.pop()) {
				pc = inst.A
			}

		case ir.OpTry:
			f.handlers = append(f.handlers, handler{target: inst.A, stackDepth: len(f.stack)})

		case ir.OpEndTry:
			if n := len(f.handlers); n > 0 {
				f.handlers = f.handlers[:n-1]
			}

		case ir.OpThrow:
			m.failAt(value.ToString(f.pop()), inst.Pos)

		case ir.OpReturn:
			if inst.A == 0 {
				return value.Undefined()
			}
			return f.pop()

		default:
			m.failAt(fmt.Sprintf("未知の命令です: %s", inst.Op), inst.Pos)
		}
	}
	return value.Undefined()
}

// constValue turns a constant pool entry into a runtime value.
func (m *VM) constValue(i int) value.Value {
	if i < 0 || i >= len(m.prog.Consts) {
		return value.Undefined()
	}
	k := m.prog.Consts[i]
	switch k.Kind {
	case ir.ConstNull:
		return value.Null()
	case ir.ConstBool:
		return value.Bool(k.Bool)
	case ir.ConstNumber:
		return value.Number(k.Num)
	case ir.ConstString:
		return value.String(k.Str)
	}
	return value.Undefined()
}

func (m *VM) constString(i int) string {
	if i < 0 || i >= len(m.prog.Consts) {
		return ""
	}
	return m.prog.Consts[i].Str
}

// setCell writes a variable cell. A constant getting here means the compiler
// let an assignment through, which is broken IR rather than a bad program.
func (m *VM) setCell(cell *value.Cell, v value.Value, inst ir.Inst) {
	if !cell.Set(v) {
		m.failAt("定数へ代入しようとしました。", inst.Pos)
	}
}

// initCell writes a constant cell for the first time.
func (m *VM) initCell(cell *value.Cell, v value.Value, inst ir.Inst) {
	if !cell.Init(v) {
		m.failAt("定数を二度初期化しようとしました。", inst.Pos)
	}
}

// loadSpecial reads a system value. 『それ』 comes from the running frame; the
// rest are shared by the program.
func (m *VM) loadSpecial(f *frame, id ir.Special) value.Value {
	if !id.Valid() {
		return value.Undefined()
	}
	if id.IsFrameSpecial() {
		return f.sore
	}
	return m.specials[id]
}

func (m *VM) storeSpecial(f *frame, id ir.Special, v value.Value) {
	if !id.Valid() {
		return
	}
	if id.IsFrameSpecial() {
		f.sore = v
		return
	}
	m.specials[id] = v
}

// loadGlobalByName reads a value the way a command names it, rather than by
// the slot the IR uses. A system value is found by name too, because that is
// how the commands refer to 『対象』 and 『抽出文字列』.
func (m *VM) loadGlobalByName(name string) value.Value {
	if id, ok := ir.SpecialByName(name); ok && !id.IsFrameSpecial() {
		return m.specials[id]
	}
	if i, ok := m.globalIndex[name]; ok {
		return m.globals[i].Get()
	}
	if v, ok := m.registry.Const(name); ok {
		return v
	}
	return value.Undefined()
}

// storeGlobalByName writes a value a command names.
func (m *VM) storeGlobalByName(name string, v value.Value) {
	if id, ok := ir.SpecialByName(name); ok && !id.IsFrameSpecial() {
		m.specials[id] = v
		return
	}
	if i, ok := m.globalIndex[name]; ok {
		m.globals[i].Set(v)
	}
}

// makeClosure builds a function value, taking a reference to each cell the
// function closes over from the frame creating it.
//
// A capture can come from the creating frame's own locals or from its
// captures, which is how a function nested two deep reaches an outer variable.
//
// A function with nothing to capture gets a shared value, so two references to
// the same plain function compare equal.
func (m *VM) makeClosure(index int, f *frame) *value.Func {
	fn := &m.prog.Funcs[index]
	if len(fn.Captures) == 0 {
		if fv, ok := m.funcValues[index]; ok {
			return fv
		}
		fv := &value.Func{ID: index}
		m.funcValues[index] = fv
		return fv
	}
	captured := make([]*value.Cell, 0, len(fn.Captures))
	for _, cap := range fn.Captures {
		source := f.locals
		if cap.ParentIsCapture {
			source = f.captures
		}
		if cap.FromParent >= 0 && cap.FromParent < len(source) {
			captured = append(captured, source[cap.FromParent])
			continue
		}
		captured = append(captured, value.NewCell(true))
	}
	return &value.Func{ID: index, Captured: captured}
}

// callStd runs a standard library command.
func (m *VM) callStd(id int, args []value.Value, pos int) value.Value {
	e := m.registry.Entry(id)
	if e == nil {
		m.failAt(fmt.Sprintf("命令が見つかりません: %d", id), pos)
	}
	if e.Fn == nil {
		m.failAt(fmt.Sprintf("命令『%s』はまだ実装されていません。", e.Name), pos)
	}
	v, err := e.Fn(m, args)
	if err != nil {
		m.failAt(err.Error(), pos)
	}
	return v
}

// --- 演算 ---

func (m *VM) binary(op ir.BinaryOp, a, b value.Value) value.Value {
	switch op {
	case ir.BinAdd:
		return value.Number(value.Add(value.ParseFloat(a), value.ParseFloat(b)))
	case ir.BinSub:
		return value.Number(value.Sub(a, b))
	case ir.BinMul:
		return value.Number(value.Mul(a, b))
	case ir.BinDiv:
		return value.Number(value.Div(a, b))
	case ir.BinIntDiv:
		return value.Number(value.IntDiv(a, b))
	case ir.BinMod:
		return value.Number(value.Mod(a, b))
	case ir.BinPow:
		return value.Number(value.Pow(a, b))
	case ir.BinConcat:
		return value.String(value.Concat(a, b))
	case ir.BinEq:
		return value.Bool(value.LooseEquals(a, b))
	case ir.BinNotEq:
		return value.Bool(!value.LooseEquals(a, b))
	case ir.BinStrictEq:
		return value.Bool(value.StrictEquals(a, b))
	case ir.BinStrictNotEq:
		return value.Bool(!value.StrictEquals(a, b))
	case ir.BinLt:
		return value.Bool(value.LessThan(a, b))
	case ir.BinLtEq:
		return value.Bool(value.LessEqual(a, b))
	case ir.BinGt:
		return value.Bool(value.GreaterThan(a, b))
	case ir.BinGtEq:
		return value.Bool(value.GreaterEqual(a, b))
	case ir.BinShiftL:
		return value.Number(value.ShiftLeft(a, b))
	case ir.BinShiftR:
		return value.Number(value.ShiftRight(a, b))
	case ir.BinShiftR0:
		return value.Number(value.ShiftRightZero(a, b))
	}
	return value.Undefined()
}

func (m *VM) unary(op ir.UnaryOp, v value.Value) value.Value {
	switch op {
	case ir.UnaryNot:
		return value.Bool(!value.ToBool(v))
	case ir.UnaryNeg:
		return value.Number(-value.ToNumber(v))
	}
	return value.Undefined()
}

// --- 添字アクセス ---

// indexGet reads through a chain of indexes, so that 『A[1][2]』 is one
// instruction with two indexes.
func (m *VM) indexGet(container value.Value, indexes []value.Value, pos int) value.Value {
	cur := container
	for _, idx := range indexes {
		switch cur.Kind() {
		case value.KindArray:
			arr, _ := cur.Array()
			cur = arr.Get(indexToInt(idx))
		case value.KindDict:
			d, _ := cur.Dict()
			v, _ := d.Get(value.ToString(idx))
			cur = v
		case value.KindString:
			s, _ := cur.String()
			cur = value.String(runeAt(s, indexToInt(idx)))
		default:
			return value.Undefined()
		}
	}
	return cur
}

// indexSet writes through a chain of indexes and returns the outermost
// container, which the caller stores back into the variable.
//
// A missing intermediate container is created, so 『A[0][1]=値』 works even when
// A held nothing.
func (m *VM) indexSet(container value.Value, indexes []value.Value, v value.Value, pos int) value.Value {
	if len(indexes) == 0 {
		return v
	}
	cur := m.ensureContainer(container, indexes[0])
	root := cur
	for i := 0; i < len(indexes)-1; i++ {
		next := m.indexGet(cur, indexes[i:i+1], pos)
		next = m.ensureContainer(next, indexes[i+1])
		m.storeOne(cur, indexes[i], next)
		cur = next
	}
	m.storeOne(cur, indexes[len(indexes)-1], v)
	return root
}

// ensureContainer returns v when it can already hold the index, and a fresh
// array or dictionary otherwise.
func (m *VM) ensureContainer(v value.Value, index value.Value) value.Value {
	switch v.Kind() {
	case value.KindArray, value.KindDict:
		return v
	}
	if index.Kind() == value.KindString {
		return value.DictValue(value.NewDict())
	}
	return value.ArrayValue(value.NewArray())
}

func (m *VM) storeOne(container value.Value, index value.Value, v value.Value) {
	switch container.Kind() {
	case value.KindArray:
		arr, _ := container.Array()
		arr.Set(indexToInt(index), v)
	case value.KindDict:
		d, _ := container.Dict()
		d.Set(value.ToString(index), v)
	}
}

// iterKeys lists what 『反復』 walks: the indexes of an array, or the keys of a
// dictionary in insertion order.
func (m *VM) iterKeys(v value.Value) value.Value {
	switch v.Kind() {
	case value.KindArray:
		arr, _ := v.Array()
		keys := make([]value.Value, arr.Len())
		for i := range keys {
			keys[i] = value.Number(float64(i))
		}
		return value.ArrayValue(value.NewArray(keys...))
	case value.KindDict:
		d, _ := v.Dict()
		names := d.Keys()
		keys := make([]value.Value, len(names))
		for i, k := range names {
			keys[i] = value.String(k)
		}
		return value.ArrayValue(value.NewArray(keys...))
	}
	return value.ArrayValue(value.NewArray())
}

// indexToInt converts an index to an array position. A non-numeric index
// becomes -1, which reads as undefined and refuses to store.
func indexToInt(v value.Value) int {
	n := value.ToNumber(v)
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return -1
	}
	return int(n)
}

// runeAt reads one character of a string by rune index, never by byte.
func runeAt(s string, i int) string {
	if i < 0 {
		return ""
	}
	for pos, r := range s {
		_ = pos
		if i == 0 {
			return string(r)
		}
		i--
	}
	return ""
}

// --- エラー ---

func (m *VM) fail(msg string, pos ir.SourcePos) {
	panic(nakoPanic{&errs.NakoError{
		Kind: errs.Runtime, File: m.fileName(pos.Source), Line: pos.Line, Msg: msg,
	}})
}

func (m *VM) failAt(msg string, posIndex int) {
	pos := ir.SourcePos{}
	if posIndex >= 0 && posIndex < len(m.prog.Positions) {
		pos = m.prog.Positions[posIndex]
	}
	m.fail(msg, pos)
}

func (m *VM) fileName(i int) string {
	if i >= 0 && i < len(m.prog.Sources) {
		return m.prog.Sources[i].Name
	}
	return ""
}

// --- stdlib.Context ---

func (m *VM) Print(s string) {
	if _, ok := m.globalIndex["表示ログ"]; ok {
		log := value.ToString(m.loadGlobalByName("表示ログ"))
		if log == "" {
			log = s
		} else {
			log += "\n" + s
		}
		m.storeGlobalByName("表示ログ", value.String(log))
	}
	m.host.Print(s)
}

func (m *VM) Write(s string) { m.host.Write(s) }

func (m *VM) ReadLine() (string, error) { return m.host.ReadLine() }

func (m *VM) Args() []string { return m.host.Args() }

func (m *VM) ReadResource(name string) ([]byte, bool) { return m.host.ReadResource(name) }

// Exit ends the program. The host decides what that means: the CUI stops the
// process, the compat runner just stops the program.
func (m *VM) Exit(code int) {
	m.host.Exit(code)
	panic(nakoPanic{&errs.NakoError{Kind: errs.Runtime, File: m.fileName(0), Msg: exitMessage}})
}

// exitMessage marks the panic that 『終了』 raises, so Run can tell an intended
// stop from a real error.
const exitMessage = "\x00終了"

func (m *VM) SysVar(name string) value.Value { return m.loadGlobalByName(name) }

func (m *VM) SetSysVar(name string, v value.Value) { m.storeGlobalByName(name, v) }

// CallFunc runs a function value on behalf of a command, converting a nadesiko
// error into an ordinary error so the command can decide what to do.
func (m *VM) CallFunc(fn *value.Func, args []value.Value) (result value.Value, err error) {
	if fn == nil {
		return value.Undefined(), errors.New("関数が指定されていません。")
	}
	defer func() {
		if r := recover(); r != nil {
			np, ok := r.(nakoPanic)
			if !ok {
				panic(r)
			}
			result, err = value.Undefined(), np.err
		}
	}()
	return m.callClosure(fn.ID, fn.Captured, args), nil
}

func (m *VM) FindFunc(name string) *value.Func {
	for i := range m.prog.Funcs {
		fn := &m.prog.Funcs[i]
		if fn.Name == name || strings.HasSuffix(fn.Name, "__"+name) {
			if len(fn.Captures) != 0 {
				return nil
			}
			return m.makeClosure(i, nil)
		}
	}
	return nil
}

func (m *VM) CommandState(name string) value.Value { return m.commandState[name] }

func (m *VM) SetCommandState(name string, v value.Value) { m.commandState[name] = v }
