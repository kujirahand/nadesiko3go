package gogen

// 数値型の推論。生成コードが `value.Value` に包まずに生の float64 で
// 計算してよい場所を、IRを読んで決める。値1つが56バイトあるので、
// 包まずに済ませられるかどうかは速度に直結する。
//
// なぜ推論できるか。なでしこの算術演算子は、**両辺が何であっても必ず数値を
// 返します** (internal/ops の binaryGeneral: 加減乗除・剰余・累乗・整数割り・
// シフトはすべて value.Number(...) を返す)。文字列を足しても NaN という
// 数値になるだけで、数値以外にはなりません。つまり「この式の結果は数値だ」は
// 両辺の型を知らなくても言えます。
//
// あとは変数です。あるスロットへの代入がすべて数値なら、読み出しも数値です。
// 『S=0』『S=S+I』の S はこれで数値だと分かります。命令の戻り値や添字
// アクセスが一度でも入るスロットは分かりません。
//
// 推論は**証明できたときだけ特殊化する**形です (AGENTS.md 12節)。分からな
// ければ今までどおり rt.Value のまま一般経路を通るので、間違える方向では
// なく、速くならない方向にしか外れません。
//
// # スタックの型はデータフローで出す
//
// 「代入の直前の命令が値を作った命令だ」と決めつけると壊れます。『かつ』
// 『または』は分岐で値を合流させるので、代入される値を作った命令は2つ
// ありえます。そこで、各命令の入口におけるスタック各段の型を、前向きの
// データフロー解析で求めます。合流点では両方が数値のときだけ数値
// (AND) にします。楽観的に「全部数値」から始めて落としていくので、
// 必ず止まります。
//
// スタックの深さが命令ごとに1つに決まることは ir.ComputeDepths が
// 保証しています (合流点で深さが違えばIR不正)。段ごとに型が言えるのは
// そのおかげです。
//
// # 推論しない関数
//
//   - エラー監視を含む関数。パニックで途中から再開するので、
//     どのスロットが書かれ終わっているか追えない
//   - 入れ子の関数を作る関数、および捕捉を持つ関数。共有セルへ
//     相手側が別の型を書きうる
//
// # 引数
//
// 引数の型は呼び出し側から決まります。ある関数のすべての `OpCallUser` が
// k番目に数値を渡しているなら、k番目の引数は数値です。ただし関数が値として
// 取り出されうる (`OpMakeFunc` の対象になっている) 場合は、その値を通して
// 何を渡されるか分からないので、引数の推論はしません。
//
// これが効くのは、『●(Nの)コラッツステップとは』のように引数を起点にして
// 計算する関数です。引数が分からないと、そこから代入される変数も芋づるで
// 分からなくなります。
//
// # 代入前に読まれるスロットは数値ではない
//
// 「代入がすべて数値なら読み出しも数値」と言えるのは、**読む時点で必ず
// 代入済みのとき**だけです。まだ代入されていないローカルを読むと undefined
// が返り、これは数値ではありません。数値だと思って ToNumber を通すと NaN に
// なってしまい、『Aを表示』が undefined ではなく NaN と出ます。
//
// そこで、どのスロットが「その位置に来た時点で必ず代入済みか」を別の
// データフローで求め (definiteAssigned)、一度でも代入前に読まれるスロットは
// 推論から外します。合流点では両方の経路で代入済みのときだけ代入済み
// (AND) とします。引数は呼び出し側が入れるので最初から代入済みです。

import "github.com/kujirahand/nadesiko3go/internal/ir"

// numericOp reports the operators whose result is a number whatever the
// operands are. 『&』(連結) は文字列、比較は真偽値になるので入っていない。
func numericOp(op ir.BinaryOp) bool {
	switch op {
	case ir.BinAdd, ir.BinSub, ir.BinMul, ir.BinDiv, ir.BinIntDiv,
		ir.BinMod, ir.BinPow, ir.BinShiftL, ir.BinShiftR, ir.BinShiftR0:
		return true
	}
	return false
}

// typeInfo says which storage a generated function may keep as a raw float64.
type typeInfo struct {
	// numericGlobals[slot] means every store to that global, anywhere in the
	// program, writes a number.
	numericGlobals map[int]bool
	// numericLocals[fi][slot] is the same for one function's local.
	numericLocals map[int]map[int]bool
	// depths[fi][pc] and stacks[fi][pc] are the operand stack's depth, and
	// which of its slots hold a raw float64, on entry to each instruction.
	depths map[int][]int
	stacks map[int][][]bool
	// seeded marks the functions whose optimistic starting set has been built.
	seeded map[int]bool
	// escapes[fi] means the function is taken as a value somewhere, so its
	// arguments can come from a call this analysis cannot see.
	escapes map[int]bool
	// promoted[fi][slot] means the local lives in a Go variable, not a cell.
	promoted map[int]map[int]bool
}

// analyze runs the whole-program pass. Globals have to be settled together
// with locals: any function can write a global, and a local's type can depend
// on a global it was loaded from, so the two shrink each other until neither
// moves.
func analyze(prog *ir.Program) *typeInfo {
	info := &typeInfo{
		numericGlobals: map[int]bool{},
		numericLocals:  map[int]map[int]bool{},
		depths:         map[int][]int{},
		stacks:         map[int][][]bool{},
		seeded:         map[int]bool{},
		escapes:        map[int]bool{},
		promoted:       map[int]map[int]bool{},
	}
	for fi := range prog.Funcs {
		for _, inst := range prog.Funcs[fi].Code {
			if inst.Op == ir.OpMakeFunc {
				info.escapes[inst.A] = true
			}
		}
	}
	for slot := range prog.Globals {
		info.numericGlobals[slot] = true
	}
	for fi := range prog.Funcs {
		_, depths, err := ir.ComputeDepths(fi, prog.Funcs[fi])
		if err != nil {
			// 検証を通ったIRならここへは来ない。来たなら推論はあきらめる
			depths = make([]int, len(prog.Funcs[fi].Code)+1)
			for i := range depths {
				depths[i] = ir.Unvisited
			}
		}
		info.depths[fi] = depths
		info.numericLocals[fi] = map[int]bool{}
	}

	for {
		changed := false
		for fi := range prog.Funcs {
			if info.refineFunc(prog, fi) {
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	// 最後にもう一度回して、確定した型でスタックの状態を作り直す
	for fi := range prog.Funcs {
		fn := &prog.Funcs[fi]
		info.stacks[fi] = info.dataflow(prog, fi, fn, info.numericLocals[fi])
		info.promoted[fi] = computePromoted(info.numericLocals[fi], fn)
	}
	return info
}

// refineFunc drops the locals and globals this function shows are not always
// numbers. It reports whether anything was dropped.
func (info *typeInfo) refineFunc(prog *ir.Program, fi int) bool {
	fn := &prog.Funcs[fi]
	locals := info.numericLocals[fi]
	changed := false

	if !unsafeToInfer(fn) && !info.seeded[fi] {
		// 初回だけ、代入される非引数スロットを楽観的に数値としておく
		for _, slot := range writtenLocals(fn) {
			locals[slot] = true
		}
		// 引数は呼び出し側から決まる。値として取り出されうる関数は、
		// 見えない呼び出しがありうるので推論しない
		for _, p := range fn.Params {
			if info.escapes[fi] {
				delete(locals, p.Slot)
			} else {
				locals[p.Slot] = true
			}
		}
		info.seeded[fi] = true
		changed = len(locals) > 0
	}

	depths := info.depths[fi]

	// 代入前に読まれるスロットを外す。型が決まる前に済ませてよい
	// (代入の有無だけを見るので、型に依らない)。
	assignedL, assignedG := definiteAssigned(fn, depths, len(prog.Globals))
	dropLocal := func(pc, slot int) {
		if locals[slot] && !assignedL[pc][slot] {
			delete(locals, slot)
			changed = true
		}
	}
	dropGlobal := func(pc, slot int) {
		if slot < 0 || slot >= len(prog.Globals) {
			return
		}
		if info.numericGlobals[slot] && !assignedG[pc][slot] {
			info.numericGlobals[slot] = false
			changed = true
		}
	}
	for pc, inst := range fn.Code {
		if depths[pc] == ir.Unvisited {
			continue
		}
		switch inst.Op {
		case ir.OpLoadLocal:
			dropLocal(pc, inst.A)
		case ir.OpLoadGlobal:
			dropGlobal(pc, inst.A)
		case ir.OpBinaryAt:
			// 覗き穴最適化がまとめた Load が中に隠れている
			_, left, right := ir.DecodeBinaryAt(inst.A)
			if left == ir.SrcLocal {
				dropLocal(pc, inst.B)
			} else if left == ir.SrcGlobal {
				dropGlobal(pc, inst.B)
			}
			if right == ir.SrcLocal {
				dropLocal(pc, inst.C)
			} else if right == ir.SrcGlobal {
				dropGlobal(pc, inst.C)
			}
		}
	}

	stacks := info.dataflow(prog, fi, fn, locals)
	info.stacks[fi] = stacks

	for pc, inst := range fn.Code {
		if depths[pc] == ir.Unvisited || depths[pc] == 0 {
			continue
		}
		top := stacks[pc][depths[pc]-1]
		switch inst.Op {
		case ir.OpStoreLocal, ir.OpInitLocal:
			if locals[inst.A] && !top {
				delete(locals, inst.A)
				changed = true
			}
		case ir.OpStoreGlobal, ir.OpInitGlobal:
			if info.numericGlobals[inst.A] && !top {
				info.numericGlobals[inst.A] = false
				changed = true
			}
		}
	}
	// 呼び出しから、呼ばれる側の引数の型を絞る
	for pc, inst := range fn.Code {
		if inst.Op != ir.OpCallUser || depths[pc] == ir.Unvisited {
			continue
		}
		if info.narrowParams(prog, inst.A, inst.B, stacks[pc], depths[pc]) {
			changed = true
		}
	}
	return changed
}

// narrowParams drops the callee's parameters this call site shows can receive
// something other than a number. The arguments sit in the top argc slots.
func (info *typeInfo) narrowParams(prog *ir.Program, callee, argc int, st []bool, d int) bool {
	if callee < 0 || callee >= len(prog.Funcs) {
		return false
	}
	params := prog.Funcs[callee].Params
	if argc != len(params) {
		return false // 数が合わないなら、どれがどれか分からないので触らない
	}
	target := info.numericLocals[callee]
	changed := false
	for j, p := range params {
		slot := d - argc + j
		if slot < 0 || slot >= len(st) {
			continue
		}
		if target[p.Slot] && !st[slot] {
			delete(target, p.Slot)
			changed = true
		}
	}
	return changed
}

// writtenLocals lists the local slots a function assigns to. A slot that is
// never written stays undefined, which is not a number.
func writtenLocals(fn *ir.Func) []int {
	seen := map[int]bool{}
	var out []int
	for _, inst := range fn.Code {
		if inst.Op == ir.OpStoreLocal || inst.Op == ir.OpInitLocal {
			if !seen[inst.A] {
				seen[inst.A] = true
				out = append(out, inst.A)
			}
		}
	}
	return out
}

// unsafeToInfer reports the functions this analysis stays out of.
func unsafeToInfer(fn *ir.Func) bool {
	if fn.NumCaptures > 0 {
		return true // 親とセルを共有している
	}
	for _, inst := range fn.Code {
		switch inst.Op {
		case ir.OpTry:
			return true // 途中から再開するので、書かれ終わりを追えない
		case ir.OpMakeFunc:
			return true // 入れ子の関数が捕捉したセルへ書きうる
		}
	}
	return false
}

// dataflow computes, for every instruction, which operand stack slots hold a
// value that is certainly a number when control reaches it.
func (info *typeInfo) dataflow(prog *ir.Program, fi int, fn *ir.Func, locals map[int]bool) [][]bool {
	depths := info.depths[fi]
	n := len(fn.Code)
	in := make([][]bool, n+1)
	visited := make([]bool, n+1)

	allFalse := unsafeToInfer(fn)

	// merge folds a state into a position, keeping only the slots both agree
	// are numbers. It reports whether the position changed.
	merge := func(at int, st []bool) bool {
		if at < 0 || at > n || depths[at] == ir.Unvisited {
			return false
		}
		if !visited[at] {
			visited[at] = true
			in[at] = append([]bool(nil), st...)
			return true
		}
		changed := false
		for i := range in[at] {
			if i < len(st) && in[at][i] && !st[i] {
				in[at][i] = false
				changed = true
			}
		}
		return changed
	}

	work := []int{0}
	merge(0, nil)
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		if pc >= n || depths[pc] == ir.Unvisited {
			continue
		}
		inst := fn.Code[pc]
		out := info.transfer(prog, inst, in[pc], locals, allFalse)

		switch inst.Op {
		case ir.OpReturn:
			continue
		case ir.OpJump:
			if merge(inst.A, out) {
				work = append(work, inst.A)
			}
			continue
		case ir.OpJumpIfFalse, ir.OpJumpIfTrue:
			if merge(inst.A, out) {
				work = append(work, inst.A)
			}
		case ir.OpTry:
			// 例外で飛び込むときは、Tryを積んだ時点の深さに戻る
			if merge(inst.A, in[pc]) {
				work = append(work, inst.A)
			}
		}
		if merge(pc+1, out) {
			work = append(work, pc+1)
		}
	}

	// 到達しなかった位置にも、長さの合った状態を置いておく
	for pc := 0; pc <= n; pc++ {
		if in[pc] == nil {
			d := depths[pc]
			if d < 0 {
				d = 0
			}
			in[pc] = make([]bool, d)
		}
	}
	return in
}

// transfer applies one instruction to a stack state.
func (info *typeInfo) transfer(prog *ir.Program, inst ir.Inst, st []bool, locals map[int]bool, allFalse bool) []bool {
	needs, delta := ir.StackDelta(inst)
	pushes := needs + delta

	out := append([]bool(nil), st...)
	if needs > len(out) {
		needs = len(out)
	}
	popped := out[len(out)-needs:]
	out = out[:len(out)-needs]

	if inst.Op == ir.OpDup {
		// 同じ値が2つ積まれる
		v := len(popped) > 0 && popped[0]
		return append(out, v, v)
	}
	for i := 0; i < pushes; i++ {
		out = append(out, !allFalse && info.pushIsNumber(prog, inst, locals))
	}
	return out
}

// pushIsNumber reports whether the value an instruction pushes is certainly a
// number. Anything not listed here is treated as unknown.
func (info *typeInfo) pushIsNumber(prog *ir.Program, inst ir.Inst, locals map[int]bool) bool {
	switch inst.Op {
	case ir.OpLoadConst:
		return inst.A >= 0 && inst.A < len(prog.Consts) &&
			prog.Consts[inst.A].Kind == ir.ConstNumber
	case ir.OpBinary:
		return numericOp(ir.BinaryOp(inst.A))
	case ir.OpBinaryAt:
		op, _, _ := ir.DecodeBinaryAt(inst.A)
		return numericOp(op)
	case ir.OpUnary:
		return ir.UnaryOp(inst.A) == ir.UnaryNeg
	case ir.OpLen:
		return true
	case ir.OpLoadLocal:
		return locals[inst.A]
	case ir.OpLoadGlobal:
		return info.numericGlobals[inst.A]
	}
	return false
}

// localIsNumber and globalIsNumber are what the code generator asks.
func (info *typeInfo) localIsNumber(fi, slot int) bool {
	return info.numericLocals[fi][slot]
}

// promotedLocals lists the slots a generated function may keep in an ordinary
// Go float64 variable instead of a *value.Cell. That is only allowed because
// the analysis already refused every function whose cells anyone else can see
// (捕捉・入れ子の関数・エラー監視 → unsafeToInfer): with no other holder of the
// cell, where the value lives is nobody else's business.
//
// 定数のスロットは外します。二度初期化したときのエラーはセルの
// Initialized で見ているので、セルを外すとその検査が消えてしまいます。
func (info *typeInfo) promotedLocals(fi int) map[int]bool {
	return info.promoted[fi]
}

func computePromoted(numeric map[int]bool, fn *ir.Func) map[int]bool {
	out := map[int]bool{}
	constVars := map[int]bool{}
	for _, slot := range fn.ConstVars {
		constVars[slot] = true
	}
	for slot := range numeric {
		if !constVars[slot] {
			out[slot] = true
		}
	}
	return out
}

func (info *typeInfo) globalIsNumber(slot int) bool {
	return info.numericGlobals[slot]
}

// definiteAssigned reports, for every instruction, which local and global
// slots are certainly assigned by the time control reaches it. Parameters
// count as assigned: the caller fills their cells before the body starts.
// Globals do not — a function has no way to know what ran before it, so a
// global only counts once this function has written it.
//
// Merging is intersection, so a slot only counts as assigned when every path
// into the instruction assigned it.
func definiteAssigned(fn *ir.Func, depths []int, numGlobals int) (locals, globals [][]bool) {
	n := len(fn.Code)
	width := fn.NumVars + numGlobals
	in := make([][]bool, n+1)
	visited := make([]bool, n+1)

	start := make([]bool, width)
	for _, p := range fn.Params {
		if p.Slot >= 0 && p.Slot < fn.NumVars {
			start[p.Slot] = true
		}
	}

	merge := func(at int, st []bool) bool {
		if at < 0 || at > n || depths[at] == ir.Unvisited {
			return false
		}
		if !visited[at] {
			visited[at] = true
			in[at] = append([]bool(nil), st...)
			return true
		}
		changed := false
		for i := range in[at] {
			if in[at][i] && !st[i] {
				in[at][i] = false
				changed = true
			}
		}
		return changed
	}

	merge(0, start)
	work := []int{0}
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		if pc >= n || depths[pc] == ir.Unvisited {
			continue
		}
		inst := fn.Code[pc]
		out := append([]bool(nil), in[pc]...)
		switch inst.Op {
		case ir.OpStoreLocal, ir.OpInitLocal:
			if inst.A >= 0 && inst.A < fn.NumVars {
				out[inst.A] = true
			}
		case ir.OpStoreGlobal, ir.OpInitGlobal:
			if i := fn.NumVars + inst.A; inst.A >= 0 && i < width {
				out[i] = true
			}
		}

		switch inst.Op {
		case ir.OpReturn:
			continue
		case ir.OpJump:
			if merge(inst.A, out) {
				work = append(work, inst.A)
			}
			continue
		case ir.OpJumpIfFalse, ir.OpJumpIfTrue:
			if merge(inst.A, out) {
				work = append(work, inst.A)
			}
		case ir.OpTry:
			if merge(inst.A, in[pc]) {
				work = append(work, inst.A)
			}
		}
		if merge(pc+1, out) {
			work = append(work, pc+1)
		}
	}

	for pc := 0; pc <= n; pc++ {
		if in[pc] == nil {
			in[pc] = make([]bool, width)
		}
	}
	locals = make([][]bool, n+1)
	globals = make([][]bool, n+1)
	for pc := 0; pc <= n; pc++ {
		locals[pc] = in[pc][:fn.NumVars]
		globals[pc] = in[pc][fn.NumVars:]
	}
	return locals, globals
}
