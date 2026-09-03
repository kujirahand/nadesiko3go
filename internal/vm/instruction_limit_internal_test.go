package vm

import (
	"errors"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestFindBlockEnds(t *testing.T) {
	code := []ir.Inst{
		{Op: ir.OpNop},
		{Op: ir.OpCallUser},
		{Op: ir.OpNop},
		{Op: ir.OpJumpIfFalse, A: 6},
		{Op: ir.OpNop},
		{Op: ir.OpJump, A: 2},
		{Op: ir.OpReturn},
	}
	want := []int32{2, 0, 4, 0, 6, 0, 7}
	got := findBlockEnds(code)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("blockEnds[%d] = %d, want %d (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestInstructionLimitKeepsExactBoundary(t *testing.T) {
	prog := &ir.Program{
		Version: ir.CurrentVersion,
		Consts:  []ir.Const{{Kind: ir.ConstNumber, Num: 42}},
		Globals: []string{"X"},
		Funcs: []ir.Func{{
			NumVars:  0,
			MaxStack: 1,
			Code: []ir.Inst{
				{Op: ir.OpLoadConst, A: 0, Pos: 0},
				{Op: ir.OpStoreGlobal, A: 0, Pos: 1},
				{Op: ir.OpNop, Pos: 2},
				{Op: ir.OpReturn, Pos: 3},
			},
		}},
		Main:      0,
		Sources:   []ir.SourceFile{{Name: "limit.nako3"}},
		Positions: []ir.SourcePos{{Source: 0, Line: 0}, {Source: 0, Line: 1}, {Source: 0, Line: 2}, {Source: 0, Line: 3}},
	}
	opts := DefaultOptions()
	opts.MaxInstructions = 2
	machine := New(prog, stdlib.NewRegistry(), &Collector{}, opts)
	err := machine.Run()
	if err == nil {
		t.Fatal("instruction limit did not stop the program")
	}
	var nakoErr *errs.NakoError
	if !errors.As(err, &nakoErr) {
		t.Fatalf("error type = %T, want *errs.NakoError", err)
	}
	if got, want := nakoErr.Line, 2; got != want {
		t.Errorf("error line = %d, want %d", got, want)
	}
	if got, want := machine.executed, uint64(3); got != want {
		t.Errorf("executed = %d, want %d", got, want)
	}
	if got := machine.globals[0].Get(); !value.StrictEquals(got, value.Number(42)) {
		t.Errorf("last permitted store did not run: X = %s", value.ToString(got))
	}
}
