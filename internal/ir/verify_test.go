package ir_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// program builds a one-function program around the given code, with enough
// surrounding structure to be valid.
func program(code []ir.Inst, maxStack int) ir.Program {
	return ir.Program{
		Version:   ir.CurrentVersion,
		Consts:    []ir.Const{{Kind: ir.ConstNumber, Num: 1}},
		Globals:   []string{"A"},
		Sources:   []ir.SourceFile{{Name: "main.nako3"}},
		Positions: []ir.SourcePos{{}},
		Funcs:     []ir.Func{{Name: "main", NumVars: 1, Code: code, MaxStack: maxStack}},
	}
}

func TestStackDelta(t *testing.T) {
	tests := []struct {
		inst         ir.Inst
		needs, delta int
	}{
		{ir.Inst{Op: ir.OpConst}, 0, +1},
		{ir.Inst{Op: ir.OpPop}, 1, -1},
		{ir.Inst{Op: ir.OpDup}, 1, +1},
		{ir.Inst{Op: ir.OpBinary}, 2, -1},
		{ir.Inst{Op: ir.OpMakeArray, B: 3}, 3, -2},
		{ir.Inst{Op: ir.OpMakeDict, B: 2}, 4, -3},
		// container と添字を取って値を1つ積む
		{ir.Inst{Op: ir.OpIndexGet, B: 2}, 3, -2},
		// container、添字、値を取って container を積む
		{ir.Inst{Op: ir.OpIndexSet, B: 1}, 3, -2},
		{ir.Inst{Op: ir.OpCallStd, B: 2}, 2, -1},
		{ir.Inst{Op: ir.OpCallValue, B: 2}, 3, -2},
		{ir.Inst{Op: ir.OpReturn, A: 1}, 1, -1},
		{ir.Inst{Op: ir.OpReturn, A: 0}, 0, 0},
	}
	for _, tt := range tests {
		needs, delta := ir.StackDelta(tt.inst)
		if needs != tt.needs || delta != tt.delta {
			t.Errorf("%s(A=%d,B=%d) = (needs %d, delta %d), want (%d, %d)",
				tt.inst.Op, tt.inst.A, tt.inst.B, needs, delta, tt.needs, tt.delta)
		}
	}
}

func TestValidateAcceptsGoodProgram(t *testing.T) {
	p := program([]ir.Inst{
		{Op: ir.OpConst, A: 0},
		{Op: ir.OpStoreGlobal, A: 0},
		{Op: ir.OpReturn, A: 0},
	}, 1)
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate = %v, want nil", err)
	}
}

// TestVerifyCatchesUnderflow pins the check that a broken compiler cannot ship
// code that pops from an empty stack.
func TestVerifyCatchesUnderflow(t *testing.T) {
	p := program([]ir.Inst{
		{Op: ir.OpPop},
		{Op: ir.OpReturn, A: 0},
	}, 0)
	var invalid *ir.InvalidIRError
	err := p.Validate()
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate = %v, want InvalidIRError", err)
	}
	if !strings.Contains(err.Error(), "必要な値が足りません") {
		t.Errorf("メッセージ = %q", err.Error())
	}
}

// TestVerifyCatchesJoinMismatch pins the check that two paths meeting must
// agree on how deep the stack is.
func TestVerifyCatchesJoinMismatch(t *testing.T) {
	// 片方の道だけ値を1つ積んでから合流する
	p := program([]ir.Inst{
		{Op: ir.OpConst, A: 0},       // 0
		{Op: ir.OpJumpIfFalse, A: 3}, // 1: 偽なら3へ(深さ0)
		{Op: ir.OpConst, A: 0},       // 2: 深さ1にして落ちる
		{Op: ir.OpReturn, A: 0},      // 3: 合流点
	}, 1)
	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "合流点のスタック深さ") {
		t.Fatalf("Validate = %v, want 合流点の不一致", err)
	}
}

func TestVerifyCatchesBadMaxStack(t *testing.T) {
	p := program([]ir.Inst{
		{Op: ir.OpConst, A: 0},
		{Op: ir.OpPop},
		{Op: ir.OpReturn, A: 0},
	}, 99)
	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "MaxStack") {
		t.Fatalf("Validate = %v, want MaxStackの不一致", err)
	}
}

func TestValidateCatchesBadIndexes(t *testing.T) {
	tests := []struct {
		name string
		code []ir.Inst
		want string
	}{
		{"定数", []ir.Inst{{Op: ir.OpConst, A: 9}, {Op: ir.OpPop}, {Op: ir.OpReturn}}, "定数の添字"},
		{"ローカル", []ir.Inst{{Op: ir.OpLoadLocal, A: 9}, {Op: ir.OpPop}, {Op: ir.OpReturn}}, "ローカルスロット"},
		{"グローバル", []ir.Inst{{Op: ir.OpLoadGlobal, A: 9}, {Op: ir.OpPop}, {Op: ir.OpReturn}}, "グローバルスロット"},
		{"関数", []ir.Inst{{Op: ir.OpCallUser, A: 9}, {Op: ir.OpPop}, {Op: ir.OpReturn}}, "関数の添字"},
		{"飛び先", []ir.Inst{{Op: ir.OpJump, A: 99}, {Op: ir.OpReturn}}, "飛び先が命令範囲外"},
		{"末尾", []ir.Inst{{Op: ir.OpNop}}, "Returnで終わっていません"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := program(tt.code, 1).Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate = %v, want %q を含む", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsWrongVersion(t *testing.T) {
	p := program([]ir.Inst{{Op: ir.OpReturn}}, 0)
	p.Version = ir.CurrentVersion + 1
	if err := p.Validate(); err == nil {
		t.Fatal("違うバージョンのIRを受理しました")
	}
}
