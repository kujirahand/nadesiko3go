package gogen

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

type generator struct {
	prog *ir.Program
}

// genFunc writes native{i} — and, for a function that uses エラー監視,
// exec{i} alongside it — to out.
func (g *generator) genFunc(out *bytes.Buffer, i int, fn *ir.Func) {
	tries := tryTargets(fn.Code)
	hasSore := usesSore(fn.Code)

	if len(tries) == 0 {
		g.genSimple(out, i, fn, hasSore)
		return
	}
	g.genWithHandlers(out, i, fn, hasSore, tries)
}

// tryTargets collects the distinct resume points an OpTry in fn opens, which
// is exactly the set the exec-level dispatch switch (below) needs a case for.
func tryTargets(code []ir.Inst) []int {
	seen := map[int]bool{}
	var out []int
	for _, inst := range code {
		if inst.Op == ir.OpTry && !seen[inst.A] {
			seen[inst.A] = true
			out = append(out, inst.A)
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
		case ir.OpJump, ir.OpJumpIfFalse, ir.OpJumpIfTrue, ir.OpTry:
			targets[inst.A] = true
		}
	}
	return targets
}

// usesSore reports whether fn reads or writes 『それ』, which decides whether
// the generated function needs a local variable for it at all (an unused one
// is a Go compile error, not just a lint warning).
func usesSore(code []ir.Inst) bool {
	for _, inst := range code {
		switch inst.Op {
		case ir.OpLoadSpecial, ir.OpStoreSpecial:
			if ir.Special(inst.A) == ir.SpecialSore {
				return true
			}
		}
	}
	return false
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
func emitStore(out *bytes.Buffer, cell string, pos int) {
	fmt.Fprintf(out, "\tif !%s.Set(pop()) {\n\t\tm.Fail(\"定数へ代入しようとしました。\", %d)\n\t}\n", cell, pos)
}

func emitInit(out *bytes.Buffer, cell string, pos int) {
	fmt.Fprintf(out, "\tif !%s.Init(pop()) {\n\t\tm.Fail(\"定数を二度初期化しようとしました。\", %d)\n\t}\n", cell, pos)
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
func (g *generator) genSimple(out *bytes.Buffer, i int, fn *ir.Func, hasSore bool) {
	fmt.Fprintf(out, "func native%d(m *rt.Machine, locals, captures []*rt.Cell) rt.Value {\n", i)
	if hasSore {
		out.WriteString("\tsore := rt.String(\"\")\n\t_ = sore\n")
	}
	if len(fn.Code) == 0 {
		out.WriteString("\treturn rt.Undefined()\n}\n\n")
		return
	}
	out.WriteString(stackPrelude(fn.MaxStack))
	out.WriteString("\tgoto L0\n")
	g.emitBody(out, fn, retSimple, soreLocal(), "\treturn rt.Undefined()\n")
	out.WriteString("}\n\n")
}

// genWithHandlers emits a function that uses エラー監視: nativeN loops,
// retrying execN at the handler's target every time a runtime error is
// caught, exactly as internal/vm/run.go's run()/protect() do for the
// interpreter (recover cannot resume a live call, so resuming means calling
// execN again, fresh, rather than jumping back into the panicking one).
func (g *generator) genWithHandlers(out *bytes.Buffer, i int, fn *ir.Func, hasSore bool, tries []int) {
	fmt.Fprintf(out, "func native%d(m *rt.Machine, locals, captures []*rt.Cell) rt.Value {\n", i)
	out.WriteString("\tsore := rt.String(\"\")\n\t_ = sore\n")
	out.WriteString("\thandlers := []rt.Handler{}\n")
	out.WriteString("\tpc := 0\n")
	out.WriteString("\tfor {\n")
	fmt.Fprintf(out, "\t\tresult, handled, target := exec%d(m, locals, captures, &sore, &handlers, pc)\n", i)
	out.WriteString("\t\tif !handled {\n\t\t\treturn result\n\t\t}\n")
	out.WriteString("\t\tpc = target\n\t}\n}\n\n")

	fmt.Fprintf(out, "func exec%d(m *rt.Machine, locals, captures []*rt.Cell, sorePtr *rt.Value, handlers *[]rt.Handler, pcStart int) (result rt.Value, handled bool, target int) {\n", i)
	_ = hasSore // sore always declared in the wrapper above; execN just uses the pointer
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
		m.SetSysVar("エラーメッセージ", rt.String(msg))
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
	g.emitBody(out, fn, retHandled, sorePointer(), "\treturn rt.Undefined(), false, 0\n")
	out.WriteString("}\n\n")
}

// stackPrelude declares the operand stack and the push/pop helpers every
// instruction below is written against — the same four operations
// internal/vm/vm.go's frame offers the interpreter (push, pop, popN, and a
// peek for OpDup).
func stackPrelude(maxStack int) string {
	if maxStack < 0 {
		maxStack = 0
	}
	return fmt.Sprintf(`	globals := m.Globals()
	_ = globals
	st := make([]rt.Value, 0, %d)
	pop := func() rt.Value {
		v := st[len(st)-1]
		st = st[:len(st)-1]
		return v
	}
	popN := func(n int) []rt.Value {
		if n <= 0 {
			return nil
		}
		out := append([]rt.Value(nil), st[len(st)-n:]...)
		st = st[:len(st)-n]
		return out
	}
	push := func(v rt.Value) { st = append(st, v) }
	top := func() rt.Value { return st[len(st)-1] }
	_ = pop
	_ = popN
	_ = push
	_ = top
`, maxStack)
}

// retKind picks how OpReturn and the fallthrough at Lend hand a value back:
// genSimple's native function returns rt.Value directly, while genWithHandlers'
// execN returns the (result, handled, target) triple protect()/execute() do.
type retKind int

const (
	retSimple retKind = iota
	retHandled
)

// soreAccess says how the current function's translated code reads and
// writes 『それ』: genSimple keeps it as a plain local (sore), while
// genWithHandlers' execN receives it by pointer (sorePtr) so it survives
// being recreated on every resume after エラー監視 catches something.
type soreAccess struct {
	get, set string
}

func soreLocal() soreAccess   { return soreAccess{get: "sore", set: "sore = pop()"} }
func sorePointer() soreAccess { return soreAccess{get: "*sorePtr", set: "*sorePtr = pop()"} }

// emitBody writes fn.Code as straight-line Go statements, labelling only the
// positions something actually jumps to (jumpTargets) — Go requires every
// label be goto's destination at least once, so a position nothing jumps to
// stays unlabelled and is simply reached by falling out of the instruction
// before it. tail is the function's final return, written last, labelled
// with Lend only if some jump patches past the last instruction to reach it.
func (g *generator) emitBody(out *bytes.Buffer, fn *ir.Func, ret retKind, sore soreAccess, tail string) {
	codeLen := len(fn.Code)
	targets := jumpTargets(fn.Code)
	for pc, inst := range fn.Code {
		if targets[pc] {
			fmt.Fprintf(out, "%s:\n", label(pc, codeLen))
		}
		g.emitInst(out, inst, pc, codeLen, ret, sore)
	}
	if targets[codeLen] {
		out.WriteString("Lend:\n")
	}
	out.WriteString(tail)
}

// emitInst is internal/vm/run.go's execute() switch, statement for statement,
// as Go source instead of interpreted cases (see the package doc for why
// that keeps the two backends from drifting apart).
func (g *generator) emitInst(out *bytes.Buffer, inst ir.Inst, pc, codeLen int, ret retKind, sore soreAccess) {
	switch inst.Op {
	case ir.OpNop:
		out.WriteString("\t_ = 0\n")

	case ir.OpLoadConst:
		fmt.Fprintf(out, "\tpush(%s)\n", g.constExpr(inst.A))

	case ir.OpPop:
		out.WriteString("\tpop()\n")

	case ir.OpDup:
		out.WriteString("\tpush(top())\n")

	case ir.OpLoadLocal:
		fmt.Fprintf(out, "\tpush(locals[%d].Get())\n", inst.A)

	case ir.OpStoreLocal:
		emitStore(out, fmt.Sprintf("locals[%d]", inst.A), inst.Pos)

	case ir.OpInitLocal:
		emitInit(out, fmt.Sprintf("locals[%d]", inst.A), inst.Pos)

	case ir.OpLoadCapture:
		fmt.Fprintf(out, "\tpush(captures[%d].Get())\n", inst.A)

	case ir.OpStoreCapture:
		emitStore(out, fmt.Sprintf("captures[%d]", inst.A), inst.Pos)

	case ir.OpLoadGlobal:
		fmt.Fprintf(out, "\tpush(globals[%d].Get())\n", inst.A)

	case ir.OpStoreGlobal:
		emitStore(out, fmt.Sprintf("globals[%d]", inst.A), inst.Pos)

	case ir.OpInitGlobal:
		emitInit(out, fmt.Sprintf("globals[%d]", inst.A), inst.Pos)

	case ir.OpLoadSpecial:
		if ir.Special(inst.A) == ir.SpecialSore {
			fmt.Fprintf(out, "\tpush(%s)\n", sore.get)
		} else {
			fmt.Fprintf(out, "\tpush(m.SpecialValue(rt.Special(%d)))\n", inst.A)
		}

	case ir.OpStoreSpecial:
		if ir.Special(inst.A) == ir.SpecialSore {
			fmt.Fprintf(out, "\t%s\n", sore.set)
		} else {
			fmt.Fprintf(out, "\tm.SetSpecialValue(rt.Special(%d), pop())\n", inst.A)
		}

	case ir.OpBinary:
		fmt.Fprintf(out, "\t{\n\t\tb := pop()\n\t\ta := pop()\n\t\tpush(rt.Binary(rt.BinaryOp(%d), a, b))\n\t}\n", inst.A)

	case ir.OpUnary:
		fmt.Fprintf(out, "\tpush(rt.Unary(rt.UnaryOp(%d), pop()))\n", inst.A)

	case ir.OpMakeArray:
		fmt.Fprintf(out, "\tpush(m.MakeArray(popN(%d)))\n", inst.B)

	case ir.OpMakeDict:
		fmt.Fprintf(out, "\tpush(m.MakeDict(popN(%d)))\n", inst.B*2)

	case ir.OpIndexGet:
		fmt.Fprintf(out, "\t{\n\t\tindexes := popN(%d)\n\t\tcontainer := pop()\n\t\tpush(m.IndexGet(container, indexes, %d))\n\t}\n", inst.B, inst.Pos)

	case ir.OpIndexSet:
		fmt.Fprintf(out, "\t{\n\t\tv := pop()\n\t\tindexes := popN(%d)\n\t\tcontainer := pop()\n\t\tpush(m.IndexSet(container, indexes, v, %d))\n\t}\n", inst.B, inst.Pos)

	case ir.OpIterKeys:
		out.WriteString("\tpush(m.IterKeys(pop()))\n")

	case ir.OpLen:
		out.WriteString("\tpush(m.LenOf(pop()))\n")

	case ir.OpMakeFunc:
		fmt.Fprintf(out, "\tpush(rt.FuncValue(m.MakeClosure(%d, locals, captures)))\n", inst.A)

	case ir.OpCallStd:
		fmt.Fprintf(out, "\tpush(m.CallStd(%d, popN(%d), %d))\n", inst.A, inst.B, inst.Pos)

	case ir.OpCallUser:
		fmt.Fprintf(out, "\tpush(m.CallUser(%d, popN(%d)))\n", inst.A, inst.B)

	case ir.OpCallValue:
		fmt.Fprintf(out, "\t{\n\t\targs := popN(%d)\n\t\tcallee := pop()\n\t\tpush(m.CallDynamic(callee, args, %d))\n\t}\n", inst.B, inst.Pos)

	case ir.OpJump:
		fmt.Fprintf(out, "\tgoto %s\n", label(inst.A, codeLen))

	case ir.OpJumpIfFalse:
		fmt.Fprintf(out, "\tif !rt.ToBool(pop()) {\n\t\tgoto %s\n\t}\n", label(inst.A, codeLen))

	case ir.OpJumpIfTrue:
		fmt.Fprintf(out, "\tif rt.ToBool(pop()) {\n\t\tgoto %s\n\t}\n", label(inst.A, codeLen))

	case ir.OpTry:
		fmt.Fprintf(out, "\t*handlers = append(*handlers, rt.Handler{Target: %d})\n", inst.A)

	case ir.OpEndTry:
		out.WriteString("\tif n := len(*handlers); n > 0 {\n\t\t*handlers = (*handlers)[:n-1]\n\t}\n")

	case ir.OpThrow:
		fmt.Fprintf(out, "\tm.Fail(rt.ToString(pop()), %d)\n", inst.Pos)

	case ir.OpReturn:
		g.emitReturn(out, inst, ret)

	default:
		// 検証器を通ったIRにここへ来る命令はない。来たら、それはgogenが
		// 新しい命令に追随できていないという意味なので、はっきり止める。
		fmt.Fprintf(out, "\tpanic(\"gogen: 未対応の命令です: %s\")\n", inst.Op)
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
