package ir

import "fmt"

// InvalidIRError reports IR that cannot be executed safely. It means the
// compiler produced something broken, not that the program was wrong.
type InvalidIRError struct {
	Func int
	Inst int
	Msg  string
}

func (e *InvalidIRError) Error() string {
	return fmt.Sprintf("IRが壊れています: Funcs[%d].Code[%d]: %s", e.Func, e.Inst, e.Msg)
}

// StackDelta reports how much an instruction changes the operand stack, and
// how many values it needs to be there already.
//
// Keeping this in one place lets the compiler and the validator agree, and
// gives the verifier something to check the generated code against.
func StackDelta(inst Inst) (needs int, delta int) {
	switch inst.Op {
	case OpNop, OpJump, OpTry, OpEndTry:
		return 0, 0
	case OpLoadConst, OpLoadLocal, OpLoadCapture, OpLoadGlobal, OpLoadSpecial, OpMakeFunc:
		return 0, +1
	case OpBinaryAt:
		// 両辺をスタックを経ずに読むので、積むだけ
		return 0, +1
	case OpPop, OpThrow,
		OpStoreLocal, OpInitLocal, OpStoreCapture,
		OpStoreGlobal, OpInitGlobal, OpStoreSpecial:
		return 1, -1
	case OpJumpIfFalse, OpJumpIfTrue:
		return 1, -1
	case OpDup:
		return 1, +1
	case OpUnary, OpIterKeys, OpLen:
		return 1, 0
	case OpBinary:
		return 2, -1
	case OpMakeArray:
		return inst.B, 1 - inst.B
	case OpMakeDict:
		return inst.B * 2, 1 - inst.B*2
	case OpIndexGet:
		// container と B個の添字を取り、値を1つ積む
		return inst.B + 1, -inst.B
	case OpIndexSet:
		// container、B個の添字、値を取り、書き戻した container を積む
		return inst.B + 2, -(inst.B + 1)
	case OpCallStd, OpCallUser:
		return inst.B, 1 - inst.B
	case OpCallValue:
		// 呼び出す値とB個の引数を取り、戻り値を1つ積む
		return inst.B + 1, -inst.B
	case OpReturn:
		return inst.A, -inst.A
	}
	return 0, 0
}

// Validate checks the structure of a program: that every index is in range and
// every function ends properly.
func (p Program) Validate() error {
	if p.Version != CurrentVersion {
		return fmt.Errorf("非対応のIRバージョンです: %d (対応: %d)", p.Version, CurrentVersion)
	}
	if p.Main < 0 || p.Main >= len(p.Funcs) {
		return fmt.Errorf("Mainが関数範囲外です: %d", p.Main)
	}
	constGlobals := map[int]bool{}
	for _, slot := range p.ConstGlobals {
		if slot < 0 || slot >= len(p.Globals) {
			return fmt.Errorf("定数グローバルが範囲外です: %d", slot)
		}
		if constGlobals[slot] {
			return fmt.Errorf("定数グローバルが重複しています: %d", slot)
		}
		constGlobals[slot] = true
	}
	for fi := range p.Funcs {
		if err := p.validateFunc(fi, constGlobals); err != nil {
			return err
		}
	}
	for i, position := range p.Positions {
		if position.Source < 0 || position.Source >= len(p.Sources) {
			return fmt.Errorf("Positions[%d].Sourceが範囲外です: %d", i, position.Source)
		}
		if position.Line < 0 || position.Column < 0 || position.Offset < 0 {
			return fmt.Errorf("Positions[%d]に負のソース位置があります", i)
		}
	}
	return p.VerifyStacks()
}

func (p Program) validateFunc(fi int, constGlobals map[int]bool) error {
	f := p.Funcs[fi]
	bad := func(i int, format string, args ...any) error {
		return &InvalidIRError{Func: fi, Inst: i, Msg: fmt.Sprintf(format, args...)}
	}
	if f.NumVars < len(f.Params) {
		return bad(0, "NumVars(%d)がParams(%d)より小さいです", f.NumVars, len(f.Params))
	}
	if len(f.Captures) != f.NumCaptures {
		return bad(0, "NumCaptures(%d)とCaptures(%d)が食い違っています", f.NumCaptures, len(f.Captures))
	}
	for _, c := range f.Captures {
		if c.FromParent < 0 {
			return bad(0, "捕捉元のスロットが負です: %d", c.FromParent)
		}
	}
	constVars := map[int]bool{}
	for _, slot := range f.ConstVars {
		if slot < 0 || slot >= f.NumVars {
			return bad(0, "定数スロットが範囲外です: %d", slot)
		}
		if constVars[slot] {
			return bad(0, "定数スロットが重複しています: %d", slot)
		}
		constVars[slot] = true
	}
	for i, p := range f.Params {
		if p.Slot < 0 || p.Slot >= f.NumVars {
			return bad(0, "Params[%d].Slotが範囲外です: %d", i, p.Slot)
		}
	}
	for i, inst := range f.Code {
		if inst.Pos < 0 || inst.Pos >= len(p.Positions) {
			return bad(i, "Posが範囲外です: %d", inst.Pos)
		}
		switch inst.Op {
		case OpLoadConst:
			if inst.A < 0 || inst.A >= len(p.Consts) {
				return bad(i, "定数の添字が範囲外です: %d", inst.A)
			}
		case OpLoadLocal, OpStoreLocal, OpInitLocal:
			if inst.A < 0 || inst.A >= f.NumVars {
				return bad(i, "ローカルスロットが範囲外です: %d", inst.A)
			}
			// 定数セルへの通常の書き込みと、変数セルの初期化を弾く
			if inst.Op == OpStoreLocal && constVars[inst.A] {
				return bad(i, "定数スロット%dへStoreLocalしています", inst.A)
			}
			if inst.Op == OpInitLocal && !constVars[inst.A] {
				return bad(i, "変数スロット%dへInitLocalしています", inst.A)
			}
		case OpLoadCapture, OpStoreCapture:
			if inst.A < 0 || inst.A >= f.NumCaptures {
				return bad(i, "捕捉スロットが範囲外です: %d", inst.A)
			}
		case OpLoadGlobal, OpStoreGlobal, OpInitGlobal:
			if inst.A < 0 || inst.A >= len(p.Globals) {
				return bad(i, "グローバルスロットが範囲外です: %d", inst.A)
			}
			if inst.Op == OpStoreGlobal && constGlobals[inst.A] {
				return bad(i, "定数グローバル%dへStoreGlobalしています", inst.A)
			}
			if inst.Op == OpInitGlobal && !constGlobals[inst.A] {
				return bad(i, "変数グローバル%dへInitGlobalしています", inst.A)
			}
		case OpLoadSpecial, OpStoreSpecial:
			if !Special(inst.A).Valid() {
				return bad(i, "システム値の番号が範囲外です: %d", inst.A)
			}
		case OpBinaryAt:
			_, left, right := DecodeBinaryAt(inst.A)
			if err := p.checkSrc(fi, i, f, left, inst.B); err != nil {
				return err
			}
			if err := p.checkSrc(fi, i, f, right, inst.C); err != nil {
				return err
			}
		case OpCallUser:
			if inst.A < 0 || inst.A >= len(p.Funcs) {
				return bad(i, "関数の添字が範囲外です: %d", inst.A)
			}
			callee := &p.Funcs[inst.A]
			if callee.NumCaptures > 0 || len(callee.Captures) > 0 {
				return bad(i, "捕捉を必要とする関数%dをCallUserで直接呼び出しています", inst.A)
			}
		case OpMakeFunc:
			if inst.A < 0 || inst.A >= len(p.Funcs) {
				return bad(i, "関数の添字が範囲外です: %d", inst.A)
			}
		case OpJump, OpJumpIfFalse, OpJumpIfTrue, OpTry:
			if inst.A < 0 || inst.A > len(f.Code) {
				return bad(i, "飛び先が命令範囲外です: %d", inst.A)
			}
		}
		if inst.B < 0 {
			return bad(i, "Bが負です: %d", inst.B)
		}
		if inst.C < 0 {
			return bad(i, "Cが負です: %d", inst.C)
		}
	}
	// 実行が命令列の末尾を走り抜けないよう、必ず Return で終わる
	if len(f.Code) == 0 || f.Code[len(f.Code)-1].Op != OpReturn {
		return bad(len(f.Code), "関数がReturnで終わっていません")
	}
	return nil
}

// checkSrc range-checks one operand of a fused instruction. The plain
// Load instructions are checked above; a fused operand names the same cells,
// so it has to pass the same test.
func (p Program) checkSrc(fi, i int, f Func, src Src, index int) error {
	bad := func(format string, args ...any) error {
		return &InvalidIRError{Func: fi, Inst: i, Msg: fmt.Sprintf(format, args...)}
	}
	limit := 0
	switch src {
	case SrcConst:
		limit = len(p.Consts)
	case SrcLocal:
		limit = f.NumVars
	case SrcCapture:
		limit = f.NumCaptures
	case SrcGlobal:
		limit = len(p.Globals)
	default:
		return bad("被演算子の取得元が範囲外です: %d", src)
	}
	if index < 0 || index >= limit {
		return bad("%sの添字が範囲外です: %d", src, index)
	}
	return nil
}

// VerifyStacks walks each function and checks that the operand stack is
// consistent: nothing pops from an empty stack, and two paths that meet agree
// on how deep it is.
//
// It also recomputes MaxStack and requires the recorded value to match, which
// catches a compiler that emitted code its own bookkeeping did not expect.
func (p Program) VerifyStacks() error {
	for fi, f := range p.Funcs {
		maxDepth, err := ComputeMaxStack(fi, f)
		if err != nil {
			return err
		}
		if f.MaxStack != maxDepth {
			return &InvalidIRError{Func: fi, Inst: 0,
				Msg: fmt.Sprintf("MaxStack=%d だが実際は %d です", f.MaxStack, maxDepth)}
		}
	}
	return nil
}

// ComputeMaxStack walks a function and reports how deep its operand stack
// gets, failing when the stack is used inconsistently.
//
// The compiler and the verifier both call it, so the recorded MaxStack is a
// genuine cross-check rather than a restatement.
func ComputeMaxStack(fi int, f Func) (int, error) {
	const unvisited = -1
	maxDepth := 0
	depths := make([]int, len(f.Code)+1)
	for i := range depths {
		depths[i] = unvisited
	}
	bad := func(i int, format string, args ...any) error {
		return &InvalidIRError{Func: fi, Inst: i, Msg: fmt.Sprintf(format, args...)}
	}

	type todo struct{ at, depth int }
	work := []todo{{0, 0}}
	for len(work) > 0 {
		item := work[len(work)-1]
		work = work[:len(work)-1]
		if item.at < 0 || item.at > len(f.Code) {
			return 0, bad(item.at, "飛び先が命令範囲外です")
		}
		if depths[item.at] != unvisited {
			if depths[item.at] != item.depth {
				return 0, bad(item.at, "合流点のスタック深さが違います: %d と %d",
					depths[item.at], item.depth)
			}
			continue
		}
		depths[item.at] = item.depth
		if item.at == len(f.Code) {
			continue
		}
		if item.depth > maxDepth {
			maxDepth = item.depth
		}

		inst := f.Code[item.at]
		needs, delta := StackDelta(inst)
		if item.depth < needs {
			return 0, bad(item.at, "%s に必要な値が足りません: 深さ%d, 必要%d",
				inst.Op, item.depth, needs)
		}
		next := item.depth + delta
		if next > maxDepth {
			maxDepth = next
		}

		switch inst.Op {
		case OpReturn:
			continue // ここで関数を抜ける
		case OpJump:
			work = append(work, todo{inst.A, next})
			continue
		case OpJumpIfFalse, OpJumpIfTrue:
			work = append(work, todo{inst.A, next})
		case OpTry:
			// 例外で飛び込むときは、Tryを積んだ時点の深さに戻る
			work = append(work, todo{inst.A, item.depth})
		}
		work = append(work, todo{item.at + 1, next})
	}
	return maxDepth, nil
}
