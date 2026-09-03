package compiler

// IRの覗き穴最適化。ASTを歩き終えて命令列ができたあと、隣り合う命令の
// 決まった並びを短いものに置き換える。AST側の畳み込み (fold.go) が「値」を
// 減らすのに対し、こちらは「命令の数」を減らす。
//
// なぜASTではなくIRでやるか。減らしたいのは命令のディスパッチとオペランド
// スタックの上げ下げで、それはASTには現れない。AGENTS.md §6 が言うとおり、
// BenchmarkLoop の時間はディスパッチが約40%、push/pop が約23%を占める。
//
// 入れている規則は8つ。命令列を一度なめるだけの1パスで、まとめた結果を
// もう一度まとめ直すことはしない (そうしてできる並びが今のところない)。
//
//	Load;Load;Binary;StoreLocal → BinaryAtStoreLocal 両辺を直に読み直接代入
//	Load;Load;Binary;JumpIfFalse/True → JumpIf(Not)BinaryAt 両辺を直に比較して分岐
//	LoadLocal;Dup;StoreSpecial;Store → StoreSoreAndLocal/Global ループ変数を直に代入
//	Load;Load;Binary            → BinaryAt          両辺をスタックを経ずに読む
//	Load;Load;IndexGet 1        → IndexGetAt        配列要素を直に読み出す
//	Binary;StoreLocal/Global    → BinaryStoreLocal/Global 計算結果を直に代入
//	Dup;StoreSpecial;Pop        → StoreSpecial
//	Load;Pop                    → (なし)
//
// 下の2つは、命令の呼び出しが必ず戻り値を『それ』に入れる (expr.go の
// compileCall) ことから来る。文として書かれた呼び出しは結果を使わないので、
// 積み直してすぐ捨てる分がまるごと消える。戻り値のない命令 (『表示』など) は
// 決まって『LoadConst 未定義; Pop』で終わるので、こちらは1文につき2命令減る。
//
// 命令を消すと番号がずれるので、飛び先を全部つけ替える。飛び込まれる位置を
// まとめて潰さないよう、先に飛び先の集合を取っておく (jumpTargets)。

import "github.com/kujirahand/nadesiko3go/internal/ir"

// optimize rewrites a function's code in place-equivalent form, returning the
// shorter version. It is called before MaxStack is computed, so the caller
// always measures the code that will actually run.
func optimize(code []ir.Inst) []ir.Inst {
	targets := jumpTargets(code)

	// moved[i] は、元の命令iが新しい命令列のどこへ行ったか。飛び先のつけ替えに
	// 使う。末尾(len(code))も飛び先になりうるので1つ多く取る。
	moved := make([]int, len(code)+1)
	out := make([]ir.Inst, 0, len(code))

	for pc := 0; pc < len(code); {
		moved[pc] = len(out)
		repl, width := fuseAt(code, pc, targets)
		if width == 0 {
			out = append(out, code[pc])
			pc++
			continue
		}
		start := len(out)
		out = append(out, repl...)
		// まとめられた2命令目以降にも位置を与えておく。飛び込まれない位置
		// だと確かめてあるので実際には使われないが、
		// 「番号がずれたまま残る」ことのないようにする。
		for i := 1; i < width; i++ {
			moved[pc+i] = start
		}
		pc += width
	}
	moved[len(code)] = len(out)

	for i := range out {
		switch out[i].Op {
		case ir.OpJump, ir.OpJumpIfFalse, ir.OpJumpIfTrue, ir.OpTry,
			ir.OpJumpIfBinaryAt, ir.OpJumpIfNotBinaryAt:
			out[i].A = int32(moved[out[i].A])
		}
	}
	return out
}

// jumpTargets is every position something jumps to, including the one past the
// end. A group may not start before one of these and swallow it: the jump
// would land in the middle of an instruction that no longer exists.
func jumpTargets(code []ir.Inst) []bool {
	targets := make([]bool, len(code)+1)
	for _, inst := range code {
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIfFalse, ir.OpJumpIfTrue, ir.OpTry,
			ir.OpJumpIfBinaryAt, ir.OpJumpIfNotBinaryAt:
			if inst.A >= 0 && int(inst.A) <= len(code) {
				targets[inst.A] = true
			}
		}
	}
	return targets
}

// fuseAt reports what replaces the run of instructions starting at pc, and how
// many it replaces. A width of 0 means nothing matched; an empty replacement
// means the run goes away entirely.
func fuseAt(code []ir.Inst, pc int, targets []bool) ([]ir.Inst, int) {
	if inst, ok := fuseBinaryAtStoreLocal(code, pc, targets); ok {
		return []ir.Inst{inst}, 4
	}
	if inst, ok := fuseJumpBinaryAt(code, pc, targets); ok {
		return []ir.Inst{inst}, 4
	}
	if inst, ok := fuseStoreSoreAndVar(code, pc, targets); ok {
		return []ir.Inst{inst}, 4
	}
	if inst, ok := fuseBinary(code, pc, targets); ok {
		return []ir.Inst{inst}, 3
	}
	if inst, ok := fuseIndexGet(code, pc, targets); ok {
		return []ir.Inst{inst}, 3
	}
	if inst, ok := fuseStoreSpecial(code, pc, targets); ok {
		return []ir.Inst{inst}, 3
	}
	if inst, ok := fuseBinaryStore(code, pc, targets); ok {
		return []ir.Inst{inst}, 2
	}
	if dropLoadPop(code, pc, targets) {
		return nil, 2
	}
	return nil, 0
}

// dropLoadPop matches 『Load;Pop』: a value fetched and thrown away without
// being looked at. A Load reads a cell and nothing else (value.Cell.Get is a
// field read), so dropping both instructions changes nothing.
func dropLoadPop(code []ir.Inst, pc int, targets []bool) bool {
	if pc+1 >= len(code) || targets[pc+1] {
		return false
	}
	if _, ok := loadSrc(code[pc]); !ok {
		return false
	}
	return code[pc+1].Op == ir.OpPop
}

// fuseBinaryAtStoreLocal matches 『Load;Load;Binary;StoreLocal』.
//
// It fuses evaluation of a binary expression and assignment to a local variable
// into a single OpBinaryAtStoreLocal instruction, bypassing the operand stack entirely.
func fuseBinaryAtStoreLocal(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+3 >= len(code) || targets[pc+1] || targets[pc+2] || targets[pc+3] {
		return ir.Inst{}, false
	}
	left, ok := loadSrc(code[pc])
	if !ok {
		return ir.Inst{}, false
	}
	right, ok := loadSrc(code[pc+1])
	if !ok {
		return ir.Inst{}, false
	}
	op := code[pc+2]
	if op.Op != ir.OpBinary {
		return ir.Inst{}, false
	}
	store := code[pc+3]
	if store.Op != ir.OpStoreLocal {
		return ir.Inst{}, false
	}
	dst := store.A
	if dst < 0 || dst >= 65536 {
		return ir.Inst{}, false
	}
	return ir.Inst{
		Op:  ir.OpBinaryAtStoreLocal,
		A:   ir.EncodeBinaryAtStoreLocal(ir.BinaryOp(op.A), left, right, dst),
		B:   code[pc].A,
		C:   code[pc+1].A,
		Pos: op.Pos,
	}, true
}

// fuseJumpBinaryAt matches 『Load;Load;Binary;JumpIfFalse』 and 『Load;Load;Binary;JumpIfTrue』.
//
// It fuses a binary comparison and conditional branch into a single instruction,
// bypassing the operand stack completely.
func fuseJumpBinaryAt(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+3 >= len(code) || targets[pc+1] || targets[pc+2] || targets[pc+3] {
		return ir.Inst{}, false
	}
	left, ok := loadSrc(code[pc])
	if !ok {
		return ir.Inst{}, false
	}
	right, ok := loadSrc(code[pc+1])
	if !ok {
		return ir.Inst{}, false
	}
	op := code[pc+2]
	if op.Op != ir.OpBinary {
		return ir.Inst{}, false
	}
	jump := code[pc+3]
	if jump.Op != ir.OpJumpIfFalse && jump.Op != ir.OpJumpIfTrue {
		return ir.Inst{}, false
	}
	rightIdx := code[pc+1].A
	if rightIdx < 0 || rightIdx >= 65536 {
		return ir.Inst{}, false
	}
	c := ir.EncodeJumpBinaryAt(ir.BinaryOp(op.A), left, right, rightIdx)
	newOp := ir.OpJumpIfNotBinaryAt
	if jump.Op == ir.OpJumpIfTrue {
		newOp = ir.OpJumpIfBinaryAt
	}
	return ir.Inst{
		Op:  newOp,
		A:   jump.A,
		B:   code[pc].A,
		C:   c,
		Pos: op.Pos,
	}, true
}

// fuseBinary matches 『Load;Load;Binary』.
//
// It is safe because a Load has no side effect: fetching the two operands in
// one instruction reads the same two cells the two Loads would have read, and
// nothing can run in between to change them.
func fuseBinary(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+2 >= len(code) || targets[pc+1] || targets[pc+2] {
		return ir.Inst{}, false
	}
	left, ok := loadSrc(code[pc])
	if !ok {
		return ir.Inst{}, false
	}
	right, ok := loadSrc(code[pc+1])
	if !ok {
		return ir.Inst{}, false
	}
	op := code[pc+2]
	if op.Op != ir.OpBinary {
		return ir.Inst{}, false
	}
	return ir.Inst{
		Op: ir.OpBinaryAt,
		A:  ir.EncodeBinaryAt(ir.BinaryOp(op.A), left, right),
		B:  code[pc].A,
		C:  code[pc+1].A,
		// 位置は演算子のものを使う。エラーが出るとしたら演算のほうなので
		// (『1に「あ」を足す』など)、報告する行は演算子の行が正しい。
		Pos: op.Pos,
	}, true
}

// loadSrc says which fused operand source an instruction is equivalent to.
// 『それ』を読む OpLoadSpecial は入っていない (→ ir.Src)。
func loadSrc(inst ir.Inst) (ir.Src, bool) {
	switch inst.Op {
	case ir.OpLoadConst:
		return ir.SrcConst, true
	case ir.OpLoadLocal:
		return ir.SrcLocal, true
	case ir.OpLoadCapture:
		return ir.SrcCapture, true
	case ir.OpLoadGlobal:
		return ir.SrcGlobal, true
	}
	return 0, false
}

// fuseStoreSpecial matches 『Dup;StoreSpecial;Pop』, which is what a command
// call written as a statement compiles to: the value is copied so it can go
// into 『それ』, and then thrown away unused.
func fuseStoreSpecial(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+2 >= len(code) || targets[pc+1] || targets[pc+2] {
		return ir.Inst{}, false
	}
	if code[pc].Op != ir.OpDup || code[pc+1].Op != ir.OpStoreSpecial || code[pc+2].Op != ir.OpPop {
		return ir.Inst{}, false
	}
	return code[pc+1], true
}

// fuseBinaryStore matches 『Binary;StoreLocal』 and 『Binary;StoreGlobal』.
//
// It fuses evaluating a binary expression on stack operands and assigning to a
// variable directly, bypassing stack push and subsequent pop.
func fuseBinaryStore(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+1 >= len(code) || targets[pc+1] {
		return ir.Inst{}, false
	}
	if code[pc].Op != ir.OpBinary {
		return ir.Inst{}, false
	}
	store := code[pc+1]
	if store.Op == ir.OpStoreLocal {
		return ir.Inst{
			Op:  ir.OpBinaryStoreLocal,
			A:   code[pc].A, // BinaryOp
			B:   store.A,    // dstLocal
			Pos: code[pc].Pos,
		}, true
	}
	if store.Op == ir.OpStoreGlobal {
		return ir.Inst{
			Op:  ir.OpBinaryStoreGlobal,
			A:   code[pc].A, // BinaryOp
			B:   store.A,    // dstGlobal
			Pos: code[pc].Pos,
		}, true
	}
	return ir.Inst{}, false
}

// fuseStoreSoreAndVar matches 『LoadLocal;Dup;StoreSpecial(Sore);StoreLocal』
// and 『LoadLocal;Dup;StoreSpecial(Sore);StoreGlobal』.
//
// In loop headers (such as `Iを1からNまで繰り返す`), nadesiko updates both
// 『それ』 and loop variable `I` with the loop counter. This fuses all 4
// instructions into a single store that reads the counter directly and sets
// both variables without touching the operand stack.
func fuseStoreSoreAndVar(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+3 >= len(code) || targets[pc+1] || targets[pc+2] || targets[pc+3] {
		return ir.Inst{}, false
	}
	if code[pc].Op != ir.OpLoadLocal ||
		code[pc+1].Op != ir.OpDup ||
		code[pc+2].Op != ir.OpStoreSpecial ||
		code[pc+2].A != int32(ir.SpecialSore) {
		return ir.Inst{}, false
	}
	store := code[pc+3]
	if store.Op == ir.OpStoreLocal {
		return ir.Inst{
			Op:  ir.OpStoreSoreAndLocal,
			A:   code[pc].A, // srcLocal
			B:   store.A,    // dstLocal
			Pos: store.Pos,
		}, true
	}
	if store.Op == ir.OpStoreGlobal {
		return ir.Inst{
			Op:  ir.OpStoreSoreAndGlobal,
			A:   code[pc].A, // srcLocal
			B:   store.A,    // dstGlobal
			Pos: store.Pos,
		}, true
	}
	return ir.Inst{}, false
}

// fuseIndexGet matches 『Load;Load;IndexGet 1』.
//
// It fuses loading a container and a 1-D index directly from their slots and
// retrieving the element, bypassing 2 stack pushes and 2 pops.
func fuseIndexGet(code []ir.Inst, pc int, targets []bool) (ir.Inst, bool) {
	if pc+2 >= len(code) || targets[pc+1] || targets[pc+2] {
		return ir.Inst{}, false
	}
	arrSrc, ok := loadSrc(code[pc])
	if !ok {
		return ir.Inst{}, false
	}
	idxSrc, ok := loadSrc(code[pc+1])
	if !ok {
		return ir.Inst{}, false
	}
	get := code[pc+2]
	if get.Op != ir.OpIndexGet || get.B != 1 {
		return ir.Inst{}, false
	}
	return ir.Inst{
		Op:  ir.OpIndexGetAt,
		A:   ir.EncodeIndexGetAt(arrSrc, idxSrc),
		B:   code[pc].A,
		C:   code[pc+1].A,
		Pos: get.Pos,
	}, true
}
