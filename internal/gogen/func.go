package gogen

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

type generator struct {
	prog  *ir.Program
	types *typeInfo
}

// srcIsNumber reports whether a fused operand (→ ir.Src) is known to hold a
// number, so that OpBinaryAt can be written as plain arithmetic.
func (g *generator) srcIsNumber(fi int, src ir.Src, index int) bool {
	switch src {
	case ir.SrcConst:
		if index < 0 || index >= len(g.prog.Consts) {
			return false
		}
		k := g.prog.Consts[index]
		return k.Kind == ir.ConstNumber && !math.IsNaN(k.Num) && !math.IsInf(k.Num, 0)
	case ir.SrcLocal:
		return g.types.localIsNumber(fi, index)
	case ir.SrcGlobal:
		return g.types.globalIsNumber(index)
	}
	return false
}

// operandFloat writes a fused operand as a float64 expression.
func (g *generator) operandFloat(fi int, src ir.Src, index int) string {
	if src == ir.SrcConst && index >= 0 && index < len(g.prog.Consts) {
		if k := g.prog.Consts[index]; k.Kind == ir.ConstNumber &&
			!math.IsNaN(k.Num) && !math.IsInf(k.Num, 0) {
			return goNumber(k.Num)
		}
	}
	if src == ir.SrcLocal && g.types.promotedLocals(fi)[index] {
		return fmt.Sprintf("l%d", index)
	}
	return "rt.ToNumber(" + g.operandExpr(fi, src, index) + ")"
}

// genFunc writes native{i} — and, for a function that uses エラー監視,
// exec{i} alongside it — to out.
func (g *generator) genFunc(out *bytes.Buffer, i int, fn *ir.Func) {
	tries := tryTargets(fn.Code)

	if len(tries) == 0 {
		g.genSimple(out, i, fn)
		return
	}
	g.genWithHandlers(out, i, fn, tries)
}

// tryTargets collects the distinct resume points an OpTry in fn opens, which
// is exactly the set the exec-level dispatch switch (below) needs a case for.
func tryTargets(code []ir.Inst) []int {
	seen := map[int]bool{}
	var out []int
	for _, inst := range code {
		if inst.Op == ir.OpTry && !seen[int(inst.A)] {
			seen[int(inst.A)] = true
			out = append(out, int(inst.A))
		}
	}
	return out
}

// jumpTargets is every code position something actually jumps to: Go requires
// a label be the target of at least one goto, so emitBody only puts a label
// on a position in this set — everywhere else, straight-line fallthrough
// reaches the next instruction with no label at all. 0 is always included:
// it is where the dispatch switch (in a function using エラー監視) sends a
// fresh call, and where genSimple's initial goto lands.
func jumpTargets(code []ir.Inst) map[int]bool {
	targets := map[int]bool{0: true}
	for _, inst := range code {
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIfFalse, ir.OpJumpIfTrue, ir.OpTry,
			ir.OpJumpIfBinaryAt, ir.OpJumpIfNotBinaryAt:
			targets[int(inst.A)] = true
		}
	}
	return targets
}

// constExpr writes Consts[i] as a Go literal rather than a lookup back into
// the embedded program. A constant is known at generation time, so paying for
// a bounds check and a kind switch on every use — which m.ConstValue does —
// is pure overhead in generated code.
//
// Anything the literal form cannot express falls back to m.ConstValue, so a
// constant kind added later still works, just without the speedup.
func (g *generator) constExpr(i int) string {
	if i < 0 || i >= len(g.prog.Consts) {
		return fmt.Sprintf("m.ConstValue(%d)", i)
	}
	k := g.prog.Consts[i]
	switch k.Kind {
	case ir.ConstUndefined:
		return "rt.Undefined()"
	case ir.ConstNull:
		return "rt.Null()"
	case ir.ConstBool:
		return fmt.Sprintf("rt.Bool(%t)", k.Bool)
	case ir.ConstNumber:
		// NaN や ±Inf はGoのリテラルで書けない。数は少ないので、その分だけ
		// 定数プールから引く形に戻す。
		if math.IsNaN(k.Num) || math.IsInf(k.Num, 0) {
			return fmt.Sprintf("m.ConstValue(%d)", i)
		}
		return fmt.Sprintf("rt.Number(%s)", goNumber(k.Num))
	case ir.ConstString:
		return fmt.Sprintf("rt.String(%s)", strconv.Quote(k.Str))
	}
	return fmt.Sprintf("m.ConstValue(%d)", i)
}

// operandExpr writes one operand of a fused instruction as the Go expression
// the Load it replaces would have pushed (→ compiler/peephole.go). A constant
// becomes a literal here, the same as under OpLoadConst.
func (g *generator) operandExpr(fi int, src ir.Src, index int) string {
	switch src {
	case ir.SrcConst:
		return g.constExpr(index)
	case ir.SrcLocal:
		if g.types.promotedLocals(fi)[index] {
			return fmt.Sprintf("rt.Number(l%d)", index)
		}
		return fmt.Sprintf("locals[%d].Get()", index)
	case ir.SrcCapture:
		return fmt.Sprintf("captures[%d].Get()", index)
	default:
		return fmt.Sprintf("globals[%d].Get()", index)
	}
}

// goNumber writes a float64 as a Go literal that reads back as exactly the
// same value. A whole number is written without an exponent, because most
// numbers in a なでしこ program are whole and 『5000000』 is easier to match
// up with the source than 『5e+06』 when reading generated code.
func goNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// emitStore and emitInit write a cell the way internal/vm's setCell and
// initCell do, but as the two lines they actually are rather than through a
// Machine method. Cell.Set and Cell.Init inline; a method on Machine does
// not, and it would also build an ir.Inst per assignment just to carry the
// source position. The failure messages must stay word for word what the
// interpreter raises — AGENTS.md §9 makes those part of the compatibility
// surface.
func emitStore(out *bytes.Buffer, cell, val string, pos int) {
	fmt.Fprintf(out, "\tif !%s.Set(%s) {\n\t\tm.Fail(\"定数へ代入しようとしました。\", %d)\n\t}\n", cell, val, pos)
}

func emitInit(out *bytes.Buffer, cell, val string, pos int) {
	fmt.Fprintf(out, "\tif !%s.Init(%s) {\n\t\tm.Fail(\"定数を二度初期化しようとしました。\", %d)\n\t}\n", cell, val, pos)
}

// label names the goto target for code position pc, folding every pc at or
// past the end of Code onto the shared exit label (a jump compiled to "one
// past the last instruction" is how a patch reaches the end of a function).
func label(pc, codeLen int) string {
	if pc >= codeLen {
		return "Lend"
	}
	return fmt.Sprintf("L%d", pc)
}

// genSimple emits the common case: a function with no エラー監視, so no
// recover, no resumable dispatch, no handler stack — just the unrolled
// instructions with goto standing in for jumps.
func (g *generator) genSimple(out *bytes.Buffer, i int, fn *ir.Func) {
	fmt.Fprintf(out, "func native%d(m *rt.Machine, locals, captures []*rt.Cell, specialsPtr *[rt.SpecialCount]rt.Value) rt.Value {\n", i)
	if len(fn.Code) == 0 {
		out.WriteString("\treturn rt.Undefined()\n}\n\n")
		return
	}
	out.WriteString(stackPrelude(fn.MaxStack))
	out.WriteString(g.localsPrelude(i, fn))
	out.WriteString("\tgoto L0\n")
	g.emitBody(out, i, fn, retSimple, specialsPointer(), "\treturn rt.Undefined()\n")
	out.WriteString("}\n\n")
}

// genWithHandlers emits a function that uses エラー監視: nativeN loops,
// retrying execN at the handler's target every time a runtime error is
// caught, exactly as internal/vm/run.go's run()/protect() do for the
// interpreter (recover cannot resume a live call, so resuming means calling
// execN again, fresh, rather than jumping back into the panicking one).
func (g *generator) genWithHandlers(out *bytes.Buffer, i int, fn *ir.Func, tries []int) {
	fmt.Fprintf(out, "func native%d(m *rt.Machine, locals, captures []*rt.Cell, specialsPtr *[rt.SpecialCount]rt.Value) rt.Value {\n", i)
	out.WriteString("\thandlers := []rt.Handler{}\n")
	out.WriteString("\tpc := 0\n")
	out.WriteString("\tfor {\n")
	fmt.Fprintf(out, "\t\tresult, handled, target := exec%d(m, locals, captures, specialsPtr, &handlers, pc)\n", i)
	out.WriteString("\t\tif !handled {\n\t\t\treturn result\n\t\t}\n")
	out.WriteString("\t\tpc = target\n\t}\n}\n\n")

	fmt.Fprintf(out, "func exec%d(m *rt.Machine, locals, captures []*rt.Cell, specialsPtr *[rt.SpecialCount]rt.Value, handlers *[]rt.Handler, pcStart int) (result rt.Value, handled bool, target int) {\n", i)
	out.WriteString(`	defer func() {
		r := recover()
		if r == nil {
			return
		}
		msg, isNako := m.Recover(r)
		if !isNako || len(*handlers) == 0 {
			panic(r)
		}
		h := (*handlers)[len(*handlers)-1]
		*handlers = (*handlers)[:len(*handlers)-1]
		(*specialsPtr)[rt.SpecialErrorMessage] = rt.String(msg)
		result, handled, target = rt.Undefined(), true, h.Target
	}()
`)
	out.WriteString(stackPrelude(fn.MaxStack))
	out.WriteString("\tpc := pcStart\n")
	out.WriteString("\tgoto Dispatch\n")
	out.WriteString("Dispatch:\n\tswitch pc {\n\tcase 0:\n\t\tgoto L0\n")
	for _, t := range tries {
		fmt.Fprintf(out, "\tcase %d:\n\t\tgoto %s\n", t, label(t, len(fn.Code)))
	}
	out.WriteString("\tdefault:\n\t\tgoto L0\n\t}\n")
	if len(fn.Code) == 0 {
		out.WriteString("\treturn rt.Undefined(), false, 0\n}\n\n")
		return
	}
	g.emitBody(out, i, fn, retHandled, specialsPointer(), "\treturn rt.Undefined(), false, 0\n")
	out.WriteString("}\n\n")
}

// localsPrelude declares the locals kept in ordinary Go variables rather than
// in a *rt.Cell (→ typeInfo.promotedLocals). A parameter's value arrives in
// its cell, so it is copied out once on entry; every other promoted slot is
// assigned before it is read (definiteAssigned), so zero is never observed.
func (g *generator) localsPrelude(fi int, fn *ir.Func) string {
	promoted := g.types.promotedLocals(fi)
	if len(promoted) == 0 {
		return ""
	}
	params := map[int]bool{}
	for _, p := range fn.Params {
		params[p.Slot] = true
	}
	var b bytes.Buffer
	for slot := 0; slot < fn.NumVars; slot++ {
		if !promoted[slot] {
			continue
		}
		if params[slot] {
			fmt.Fprintf(&b, "\tl%d := rt.ToNumber(locals[%d].Get())\n\t_ = l%d\n", slot, slot, slot)
			continue
		}
		fmt.Fprintf(&b, "\tvar l%d float64\n\t_ = l%d\n", slot, slot)
	}
	return b.String()
}

// stackPrelude declares the operand stack. It is not a slice: ir.ComputeDepths
// proves that every instruction sees the stack at one fixed depth however it
// was reached, so each depth can be a plain Go variable and push/pop become
// ordinary assignments (→ types.go, gogen のパッケージコメント).
//
// Each depth gets two variables. sN holds a boxed rt.Value; fN holds a raw
// float64 for the depths the type analysis proved are always numbers. Only one
// of the two is live at a time, and which one is decided per instruction by
// the analysis, so there is no tag to check at run time.
func stackPrelude(maxStack int) string {
	var b bytes.Buffer
	b.WriteString("\tglobals := m.Globals()\n\t_ = globals\n")
	for i := 0; i < maxStack; i++ {
		fmt.Fprintf(&b, "\tvar s%d rt.Value\n\tvar f%d float64\n\t_, _ = s%d, f%d\n", i, i, i, i)
	}
	return b.String()
}

// retKind picks how OpReturn and the fallthrough at Lend hand a value back:
// genSimple's native function returns rt.Value directly, while genWithHandlers'
// execN returns the (result, handled, target) triple protect()/execute() do.
type retKind int

const (
	retSimple retKind = iota
	retHandled
)

// specialsAccess writes accesses to the VM frame's system-value array. Both
// generated instructions and stdlib Context calls use this same storage.
type specialsAccess struct{}

func specialsPointer() specialsAccess { return specialsAccess{} }

func (s specialsAccess) get(id int) string {
	return fmt.Sprintf("(*specialsPtr)[%d]", id)
}

func (s specialsAccess) set(id int, val string) string {
	return fmt.Sprintf("(*specialsPtr)[%d] = %s", id, val)
}

// fnEmit carries what emitting one function needs: where each instruction sees
// the operand stack (depth and which slots are raw float64), and how this
// function returns.
type fnEmit struct {
	g        *generator
	out      *bytes.Buffer
	fi       int
	fn       *ir.Func
	codeLen  int
	depths   []int
	stacks   [][]bool
	promoted map[int]bool
	ret      retKind
	specials specialsAccess
}

// slotValue reads stack slot i as a boxed rt.Value, boxing it if the slot is
// currently a raw float64.
func (e *fnEmit) slotValue(st []bool, i int) string {
	if i < len(st) && st[i] {
		return fmt.Sprintf("rt.Number(f%d)", i)
	}
	return fmt.Sprintf("s%d", i)
}

// slotFloat reads stack slot i as a float64. A slot the analysis did not prove
// numeric still has to go through ToNumber, which is what the interpreter does.
func (e *fnEmit) slotFloat(st []bool, i int) string {
	if i < len(st) && st[i] {
		return fmt.Sprintf("f%d", i)
	}
	return fmt.Sprintf("rt.ToNumber(s%d)", i)
}

// in returns the stack state on entry to pc.
func (e *fnEmit) in(pc int) []bool {
	if pc < 0 || pc >= len(e.stacks) {
		return nil
	}
	return e.stacks[pc]
}

// depth returns the stack depth on entry to pc.
func (e *fnEmit) depth(pc int) int {
	if pc < 0 || pc >= len(e.depths) || e.depths[pc] == ir.Unvisited {
		return 0
	}
	return e.depths[pc]
}

// convert writes the assignments that make the live state match what the
// instruction at "to" expects. The analysis merges with AND, so a slot can
// only need boxing (raw → boxed), never the other way round.
func (e *fnEmit) convert(out []bool, to int) {
	want := e.in(to)
	for i := 0; i < len(out) && i < len(want); i++ {
		if out[i] && !want[i] {
			fmt.Fprintf(e.out, "\ts%d = rt.Number(f%d)\n", i, i)
		}
	}
}

// emitBody writes fn.Code as straight-line Go statements, labelling only the
// positions something actually jumps to (jumpTargets) — Go requires every
// label be goto's destination at least once, so a position nothing jumps to
// stays unlabelled and is simply reached by falling out of the instruction
// before it. tail is the function's final return, written last, labelled
// with Lend only if some jump patches past the last instruction to reach it.
//
// Unreachable instructions are skipped: they have no stack depth, so there is
// no slot to name, and nothing can jump to them (a jump would make them
// reachable).
func (g *generator) emitBody(out *bytes.Buffer, fi int, fn *ir.Func, ret retKind, specials specialsAccess, tail string) {
	e := &fnEmit{
		g: g, out: out, fi: fi, fn: fn, codeLen: len(fn.Code),
		depths: g.types.depths[fi], stacks: g.types.stacks[fi],
		promoted: g.types.promotedLocals(fi),
		ret:      ret, specials: specials,
	}
	targets := jumpTargets(fn.Code)
	for pc, inst := range fn.Code {
		if e.depths[pc] == ir.Unvisited {
			continue
		}
		if targets[pc] {
			fmt.Fprintf(out, "%s:\n", label(pc, e.codeLen))
		}
		e.emitInst(inst, pc)
	}
	if targets[e.codeLen] {
		out.WriteString("Lend:\n")
	}
	out.WriteString(tail)
}

// emitInst is internal/vm/run.go's execute() switch, statement for statement,
// as Go source instead of interpreted cases (see the package doc for why that
// keeps the two backends from drifting apart). The difference is that operands
// are named stack slots rather than push/pop calls.
func (e *fnEmit) emitInst(inst ir.Inst, pc int) {
	out := e.out
	st := e.in(pc)
	d := e.depth(pc)
	// top(k) は上から k 番目 (0が先頭) のスロット番号
	top := func(k int) int { return d - 1 - k }
	// push は結果を積む先。積んだあとの状態は after で表す。
	after := func(n int) []bool {
		res := append([]bool(nil), st...)
		if n > len(res) {
			n = len(res)
		}
		return res[:len(res)-n]
	}
	// setValue writes a boxed value into slot i and marks it boxed.
	setValue := func(state []bool, i int, expr string) []bool {
		fmt.Fprintf(out, "\ts%d = %s\n", i, expr)
		return append(state, false)
	}
	setFloat := func(state []bool, i int, expr string) []bool {
		fmt.Fprintf(out, "\tf%d = %s\n", i, expr)
		return append(state, true)
	}
	// wantsFloat reports whether the analysis keeps the value this instruction
	// writes into slot i as a raw float64. The slot is not always the top of
	// the entry stack: an instruction that pops writes lower down.
	wantsFloat := func(i int) bool {
		res := e.in(pc + 1)
		return i >= 0 && i < len(res) && res[i]
	}

	var live []bool

	switch inst.Op {
	case ir.OpNop:
		live = append([]bool(nil), st...)

	case ir.OpLoadConst:
		k := e.g.prog.Consts[inst.A]
		if wantsFloat(d) && k.Kind == ir.ConstNumber && !math.IsNaN(k.Num) && !math.IsInf(k.Num, 0) {
			live = setFloat(after(0), d, goNumber(k.Num))
		} else {
			live = setValue(after(0), d, e.g.constExpr(int(inst.A)))
		}

	case ir.OpPop:
		live = after(1)

	case ir.OpDup:
		src := top(0)
		if src < len(st) && st[src] {
			live = setFloat(after(0), d, fmt.Sprintf("f%d", src))
		} else {
			live = setValue(after(0), d, fmt.Sprintf("s%d", src))
		}

	case ir.OpLoadLocal:
		if e.promoted[int(inst.A)] {
			if wantsFloat(d) {
				live = setFloat(after(0), d, fmt.Sprintf("l%d", inst.A))
			} else {
				live = setValue(after(0), d, fmt.Sprintf("rt.Number(l%d)", inst.A))
			}
			break
		}
		live = e.emitLoad(after(0), d, fmt.Sprintf("locals[%d]", inst.A), wantsFloat(d))

	case ir.OpStoreLocal:
		if e.promoted[int(inst.A)] {
			fmt.Fprintf(out, "\tl%d = %s\n", inst.A, e.slotFloat(st, top(0)))
			live = after(1)
			break
		}
		emitStore(out, fmt.Sprintf("locals[%d]", inst.A), e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpInitLocal:
		emitInit(out, fmt.Sprintf("locals[%d]", inst.A), e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpLoadCapture:
		live = e.emitLoad(after(0), d, fmt.Sprintf("captures[%d]", inst.A), false)

	case ir.OpStoreCapture:
		emitStore(out, fmt.Sprintf("captures[%d]", inst.A), e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpLoadGlobal:
		live = e.emitLoad(after(0), d, fmt.Sprintf("globals[%d]", inst.A), wantsFloat(d))

	case ir.OpStoreGlobal:
		emitStore(out, fmt.Sprintf("globals[%d]", inst.A), e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpInitGlobal:
		emitInit(out, fmt.Sprintf("globals[%d]", inst.A), e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpLoadSpecial:
		expr := e.specials.get(int(inst.A))
		if !ir.Special(inst.A).IsFrameSpecial() {
			expr = fmt.Sprintf("m.SpecialValue(rt.Special(%d))", inst.A)
		}
		live = setValue(after(0), d, expr)

	case ir.OpStoreSpecial:
		val := e.slotValue(st, top(0))
		if ir.Special(inst.A).IsFrameSpecial() {
			fmt.Fprintf(out, "\t%s\n", e.specials.set(int(inst.A), val))
		} else {
			fmt.Fprintf(out, "\tm.SetSpecialValue(rt.Special(%d), %s)\n", inst.A, val)
		}
		live = after(1)

	case ir.OpStoreSoreAndLocal, ir.OpStoreSoreAndGlobal:
		srcExpr := e.g.operandExpr(e.fi, ir.SrcLocal, int(inst.A))
		fmt.Fprintf(out, "\t%s\n", e.specials.set(int(ir.SpecialSore), srcExpr))
		dst := int(inst.B)
		if inst.Op == ir.OpStoreSoreAndLocal && e.promoted[dst] {
			if e.g.srcIsNumber(e.fi, ir.SrcLocal, int(inst.A)) {
				fmt.Fprintf(out, "\tl%d = %s\n", dst, e.g.operandFloat(e.fi, ir.SrcLocal, int(inst.A)))
			} else {
				fmt.Fprintf(out, "\tl%d = rt.ToNumber(%s)\n", dst, srcExpr)
			}
		} else {
			targetName := fmt.Sprintf("locals[%d]", dst)
			if inst.Op == ir.OpStoreSoreAndGlobal {
				targetName = fmt.Sprintf("globals[%d]", dst)
			}
			emitStore(out, targetName, srcExpr, int(inst.Pos))
		}
		live = append([]bool(nil), st...)

	case ir.OpBinary:
		live = e.emitBinary(after(2), top(1), ir.BinaryOp(inst.A),
			e.slotValue(st, top(1)), e.slotValue(st, top(0)),
			e.slotFloat(st, top(1)), e.slotFloat(st, top(0)),
			st[top(1)] && st[top(0)], wantsFloat(top(1)))

	case ir.OpBinaryStoreLocal, ir.OpBinaryStoreGlobal:
		op := ir.BinaryOp(inst.A)
		dst := int(inst.B)
		bothNum := st[top(1)] && st[top(0)]
		aExpr := e.slotValue(st, top(1))
		bExpr := e.slotValue(st, top(0))
		aFloat := e.slotFloat(st, top(1))
		bFloat := e.slotFloat(st, top(0))

		if inst.Op == ir.OpBinaryStoreLocal && e.promoted[dst] {
			if bothNum {
				if expr, ok := floatExpr(op, aFloat, bFloat); ok {
					fmt.Fprintf(out, "\tl%d = %s\n", dst, expr)
					live = after(2)
					break
				}
			}
			call := fmt.Sprintf("rt.Binary(rt.BinaryOp(%d), %s, %s)", op, aExpr, bExpr)
			fmt.Fprintf(out, "\tl%d = rt.ToNumber(%s)\n", dst, call)
		} else {
			var valExpr string
			if bothNum {
				if expr, ok := floatExpr(op, aFloat, bFloat); ok {
					valExpr = fmt.Sprintf("rt.Number(%s)", expr)
				} else if expr, ok := boolExpr(op, aFloat, bFloat); ok {
					valExpr = fmt.Sprintf("rt.Bool(%s)", expr)
				}
			}
			if valExpr == "" {
				valExpr = fmt.Sprintf("rt.Binary(rt.BinaryOp(%d), %s, %s)", op, aExpr, bExpr)
			}
			targetName := fmt.Sprintf("locals[%d]", dst)
			if inst.Op == ir.OpBinaryStoreGlobal {
				targetName = fmt.Sprintf("globals[%d]", dst)
			}
			emitStore(out, targetName, valExpr, int(inst.Pos))
		}
		live = after(2)

	case ir.OpBinaryAt:
		op, left, right := ir.DecodeBinaryAt(inst.A)
		// 両辺とも定数だとGoのソースに『1 / 0』と書くことになり、Goは
		// 定数どうしのゼロ除算をコンパイルエラーにする。定数どうしで
		// 畳み込めるものは compiler/fold.go が既に畳み込んでいて、ここへ
		// 残るのは NaN・±Inf になる式だけなので、一般経路へ流して困らない。
		bothConst := left == ir.SrcConst && right == ir.SrcConst
		bothNum := !bothConst &&
			e.g.srcIsNumber(e.fi, left, int(inst.B)) && e.g.srcIsNumber(e.fi, right, int(inst.C))
		live = e.emitBinary(after(0), d, op,
			e.g.operandExpr(e.fi, left, int(inst.B)), e.g.operandExpr(e.fi, right, int(inst.C)),
			e.g.operandFloat(e.fi, left, int(inst.B)), e.g.operandFloat(e.fi, right, int(inst.C)),
			bothNum, wantsFloat(d))

	case ir.OpBinaryAtStoreLocal:
		op, left, right, dst := ir.DecodeBinaryAtStoreLocal(inst.A)
		bothConst := left == ir.SrcConst && right == ir.SrcConst
		bothNum := !bothConst &&
			e.g.srcIsNumber(e.fi, left, int(inst.B)) && e.g.srcIsNumber(e.fi, right, int(inst.C))
		aExpr := e.g.operandExpr(e.fi, left, int(inst.B))
		bExpr := e.g.operandExpr(e.fi, right, int(inst.C))
		aFloat := e.g.operandFloat(e.fi, left, int(inst.B))
		bFloat := e.g.operandFloat(e.fi, right, int(inst.C))

		if e.promoted[int(dst)] {
			if bothNum {
				if expr, ok := floatExpr(op, aFloat, bFloat); ok {
					fmt.Fprintf(out, "\tl%d = %s\n", dst, expr)
					live = append([]bool(nil), st...)
					break
				}
			}
			call := fmt.Sprintf("rt.Binary(rt.BinaryOp(%d), %s, %s)", op, aExpr, bExpr)
			fmt.Fprintf(out, "\tl%d = rt.ToNumber(%s)\n", dst, call)
		} else {
			var valExpr string
			if bothNum {
				if expr, ok := floatExpr(op, aFloat, bFloat); ok {
					valExpr = fmt.Sprintf("rt.Number(%s)", expr)
				} else if expr, ok := boolExpr(op, aFloat, bFloat); ok {
					valExpr = fmt.Sprintf("rt.Bool(%s)", expr)
				}
			}
			if valExpr == "" {
				valExpr = fmt.Sprintf("rt.Binary(rt.BinaryOp(%d), %s, %s)", op, aExpr, bExpr)
			}
			emitStore(out, fmt.Sprintf("locals[%d]", dst), valExpr, int(inst.Pos))
		}
		live = append([]bool(nil), st...)

	case ir.OpUnary:
		if ir.UnaryOp(inst.A) == ir.UnaryNeg && wantsFloat(top(0)) {
			live = setFloat(after(1), top(0), "-"+e.slotFloat(st, top(0)))
		} else {
			live = setValue(after(1), top(0),
				fmt.Sprintf("rt.Unary(rt.UnaryOp(%d), %s)", inst.A, e.slotValue(st, top(0))))
		}

	case ir.OpMakeArray:
		live = setValue(after(int(inst.B)), d-int(inst.B),
			fmt.Sprintf("m.MakeArray(%s)", e.argSlice(st, d, int(inst.B))))

	case ir.OpMakeDict:
		n := int(inst.B) * 2
		live = setValue(after(n), d-n,
			fmt.Sprintf("m.MakeDict(%s)", e.argSlice(st, d, n)))

	case ir.OpIndexGet:
		base := d - int(inst.B) - 1
		live = setValue(after(int(inst.B)+1), base,
			fmt.Sprintf("m.IndexGet(%s, %s, %d)",
				e.slotValue(st, base), e.argSlice(st, d, int(inst.B)), int(inst.Pos)))

	case ir.OpIndexGetAt:
		arrSrc, idxSrc := ir.DecodeIndexGetAt(inst.A)
		arrExpr := e.g.operandExpr(e.fi, arrSrc, int(inst.B))
		idxExpr := e.g.operandExpr(e.fi, idxSrc, int(inst.C))
		live = setValue(after(0), d,
			fmt.Sprintf("m.IndexGet(%s, []rt.Value{%s}, %d)", arrExpr, idxExpr, int(inst.Pos)))

	case ir.OpIndexSet:
		base := d - int(inst.B) - 2
		live = setValue(after(int(inst.B)+2), base,
			fmt.Sprintf("m.IndexSet(%s, %s, %s, %d)",
				e.slotValue(st, base),
				e.argSliceRange(st, base+1, int(inst.B)),
				e.slotValue(st, top(0)), int(inst.Pos)))

	case ir.OpIterKeys:
		live = setValue(after(1), top(0), fmt.Sprintf("m.IterKeys(%s)", e.slotValue(st, top(0))))

	case ir.OpLen:
		if wantsFloat(top(0)) {
			live = setFloat(after(1), top(0), fmt.Sprintf("rt.ToNumber(m.LenOf(%s))", e.slotValue(st, top(0))))
		} else {
			live = setValue(after(1), top(0), fmt.Sprintf("m.LenOf(%s)", e.slotValue(st, top(0))))
		}

	case ir.OpMakeFunc:
		live = setValue(after(0), d,
			fmt.Sprintf("rt.FuncValue(m.MakeClosure(%d, locals, captures))", inst.A))

	case ir.OpCallStd:
		live = setValue(after(int(inst.B)), d-int(inst.B),
			fmt.Sprintf("m.CallStd(%d, %s, %d)", inst.A, e.argSlice(st, d, int(inst.B)), int(inst.Pos)))

	case ir.OpCallUser:
		live = setValue(after(int(inst.B)), d-int(inst.B),
			fmt.Sprintf("m.CallUser(%d, %s)", inst.A, e.argSlice(st, d, int(inst.B))))

	case ir.OpCallValue:
		base := d - int(inst.B) - 1
		live = setValue(after(int(inst.B)+1), base,
			fmt.Sprintf("m.CallDynamic(%s, %s, %d)",
				e.slotValue(st, base), e.argSliceRange(st, base+1, int(inst.B)), int(inst.Pos)))

	case ir.OpJump:
		e.convert(after(0), int(inst.A))
		fmt.Fprintf(out, "\tgoto %s\n", label(int(inst.A), e.codeLen))
		return

	case ir.OpJumpIfFalse, ir.OpJumpIfTrue:
		cond := fmt.Sprintf("rt.ToBool(%s)", e.slotValue(st, top(0)))
		if st[top(0)] {
			// 数値と分かっているなら、真偽判定は 0 でも NaN でもないこと
			cond = fmt.Sprintf("f%d != 0 && f%d == f%d", top(0), top(0), top(0))
		}
		if inst.Op == ir.OpJumpIfFalse {
			cond = "!(" + cond + ")"
		}
		fmt.Fprintf(out, "\tif %s {\n", cond)
		e.convertIndent(after(1), int(inst.A))
		fmt.Fprintf(out, "\t\tgoto %s\n\t}\n", label(int(inst.A), e.codeLen))
		live = after(1)

	case ir.OpJumpIfBinaryAt, ir.OpJumpIfNotBinaryAt:
		op, left, right, rightIdx := ir.DecodeJumpBinaryAt(inst.C)
		bothConst := left == ir.SrcConst && right == ir.SrcConst
		bothNum := !bothConst &&
			e.g.srcIsNumber(e.fi, left, int(inst.B)) && e.g.srcIsNumber(e.fi, right, int(rightIdx))
		aExpr := e.g.operandExpr(e.fi, left, int(inst.B))
		bExpr := e.g.operandExpr(e.fi, right, int(rightIdx))
		aFloat := e.g.operandFloat(e.fi, left, int(inst.B))
		bFloat := e.g.operandFloat(e.fi, right, int(rightIdx))

		var cond string
		if bothNum {
			if expr, ok := boolExpr(op, aFloat, bFloat); ok {
				cond = expr
			}
		}
		if cond == "" {
			cond = fmt.Sprintf("rt.ToBool(rt.Binary(rt.BinaryOp(%d), %s, %s))", op, aExpr, bExpr)
		}
		if inst.Op == ir.OpJumpIfNotBinaryAt {
			cond = "!(" + cond + ")"
		}
		fmt.Fprintf(out, "\tif %s {\n", cond)
		e.convertIndent(after(0), int(inst.A))
		fmt.Fprintf(out, "\t\tgoto %s\n\t}\n", label(int(inst.A), e.codeLen))
		live = append([]bool(nil), st...)

	case ir.OpTry:
		fmt.Fprintf(out, "\t*handlers = append(*handlers, rt.Handler{Target: %d})\n", inst.A)
		live = append([]bool(nil), st...)

	case ir.OpEndTry:
		out.WriteString("\tif n := len(*handlers); n > 0 {\n\t\t*handlers = (*handlers)[:n-1]\n\t}\n")
		live = append([]bool(nil), st...)

	case ir.OpThrow:
		fmt.Fprintf(out, "\tm.Fail(rt.ToString(%s), %d)\n", e.slotValue(st, top(0)), int(inst.Pos))
		live = after(1)

	case ir.OpReturn:
		value := "rt.Undefined()"
		if inst.A != 0 {
			value = e.slotValue(st, top(0))
		}
		switch e.ret {
		case retHandled:
			fmt.Fprintf(out, "\treturn %s, false, 0\n", value)
		default:
			fmt.Fprintf(out, "\treturn %s\n", value)
		}
		return

	default:
		// 検証器を通ったIRにここへ来る命令はない。来たら、それはgogenが
		// 新しい命令に追随できていないという意味なので、はっきり止める。
		fmt.Fprintf(out, "\tpanic(\"gogen: 未対応の命令です: %s\")\n", inst.Op)
		live = append([]bool(nil), st...)
	}

	e.convert(live, pc+1)
}

// emitLoad reads a cell into stack slot i, keeping it unboxed when both the
// cell and the slot are known to be numbers.
func (e *fnEmit) emitLoad(state []bool, i int, cell string, asFloat bool) []bool {
	if asFloat {
		fmt.Fprintf(e.out, "\tf%d = rt.ToNumber(%s.Get())\n", i, cell)
		return append(state, true)
	}
	fmt.Fprintf(e.out, "\ts%d = %s.Get()\n", i, cell)
	return append(state, false)
}

// emitBinary writes one binary operation. When both operands are known to be
// numbers it writes the arithmetic straight out, which is the same expression
// ops.binaryNumbers evaluates — the general path stays for everything else so
// that an unproven operand still gets なでしこ's full conversion rules.
func (e *fnEmit) emitBinary(state []bool, i int, op ir.BinaryOp, aVal, bVal, aNum, bNum string, bothNum, wantFloat bool) []bool {
	if bothNum {
		if expr, ok := floatExpr(op, aNum, bNum); ok {
			if wantFloat {
				fmt.Fprintf(e.out, "\tf%d = %s\n", i, expr)
				return append(state, true)
			}
			fmt.Fprintf(e.out, "\ts%d = rt.Number(%s)\n", i, expr)
			return append(state, false)
		}
		if expr, ok := boolExpr(op, aNum, bNum); ok {
			fmt.Fprintf(e.out, "\ts%d = rt.Bool(%s)\n", i, expr)
			return append(state, false)
		}
	}
	call := fmt.Sprintf("rt.Binary(rt.BinaryOp(%d), %s, %s)", op, aVal, bVal)
	if wantFloat {
		fmt.Fprintf(e.out, "\tf%d = rt.ToNumber(%s)\n", i, call)
		return append(state, true)
	}
	fmt.Fprintf(e.out, "\ts%d = %s\n", i, call)
	return append(state, false)
}

// floatExpr writes the arithmetic ops.binaryNumbers shortcuts, as a Go
// expression on two float64s. The ones it leaves out (『&』の連結、シフト、
// 整数割り、累乗) keep going through rt.Binary so that there is exactly one
// implementation of them.
func floatExpr(op ir.BinaryOp, a, b string) (string, bool) {
	switch op {
	case ir.BinAdd:
		return a + " + " + b, true
	case ir.BinSub:
		return a + " - " + b, true
	case ir.BinMul:
		return a + " * " + b, true
	case ir.BinDiv:
		return a + " / " + b, true
	case ir.BinMod:
		return "rt.Mod(" + a + ", " + b + ")", true
	}
	return "", false
}

// boolExpr writes the comparisons ops.binaryNumbers shortcuts. NaN が絡む
// 比較はGoもJavaScriptも false になるので、順序判定を分ける必要はない。
func boolExpr(op ir.BinaryOp, a, b string) (string, bool) {
	switch op {
	case ir.BinLt:
		return a + " < " + b, true
	case ir.BinLtEq:
		return a + " <= " + b, true
	case ir.BinGt:
		return a + " > " + b, true
	case ir.BinGtEq:
		return a + " >= " + b, true
	case ir.BinEq, ir.BinStrictEq:
		return a + " == " + b, true
	case ir.BinNotEq, ir.BinStrictNotEq:
		return a + " != " + b, true
	}
	return "", false
}

// argSlice builds the []rt.Value a call or collection literal takes, from the
// n slots below depth d.
func (e *fnEmit) argSlice(st []bool, d, n int) string {
	return e.argSliceRange(st, d-n, n)
}

// argSliceRange builds a []rt.Value from n slots starting at base.
func (e *fnEmit) argSliceRange(st []bool, base, n int) string {
	if n <= 0 {
		return "nil"
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = e.slotValue(st, base+i)
	}
	return "[]rt.Value{" + strings.Join(parts, ", ") + "}"
}

// convertIndent is convert() one tab deeper, for the inside of an if.
func (e *fnEmit) convertIndent(out []bool, to int) {
	want := e.in(to)
	for i := 0; i < len(out) && i < len(want); i++ {
		if out[i] && !want[i] {
			fmt.Fprintf(e.out, "\t\ts%d = rt.Number(f%d)\n", i, i)
		}
	}
}

func (g *generator) emitReturn(out *bytes.Buffer, inst ir.Inst, ret retKind) {
	value := "rt.Undefined()"
	if inst.A != 0 {
		value = "pop()"
	}
	switch ret {
	case retHandled:
		fmt.Fprintf(out, "\treturn %s, false, 0\n", value)
	default:
		fmt.Fprintf(out, "\treturn %s\n", value)
	}
}
