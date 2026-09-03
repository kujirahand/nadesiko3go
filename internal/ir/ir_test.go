package ir

import (
	"encoding/json"
	"fmt"
	"testing"
)

func testProgram() Program {
	return Program{
		Version: CurrentVersion,
		Funcs: []Func{{
			Name: "main",
			Code: []Inst{{Op: OpReturn, Pos: 0}},
		}},
		Main:      0,
		Sources:   []SourceFile{{Name: "main.nako3"}},
		Positions: []SourcePos{{Source: 0, Line: 0, Column: 0}},
	}
}

func TestProgramIsSerializableAndValid(t *testing.T) {
	p := testProgram()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Program
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProgramRejectsOtherVersion(t *testing.T) {
	p := testProgram()
	p.Version++
	if err := p.Validate(); err == nil {
		t.Fatal("未知のIRバージョンを受理しました")
	}
}

func TestPacked16BitIndexesRoundTrip(t *testing.T) {
	for _, index := range []int32{0, 32767, 32768, 65535} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			a := EncodeBinaryAtStoreLocal(BinAdd, SrcLocal, SrcConst, index)
			op, left, right, dst := DecodeBinaryAtStoreLocal(a)
			if op != BinAdd || left != SrcLocal || right != SrcConst || dst != index {
				t.Fatalf("BinaryAtStoreLocal round trip = (%v, %v, %v, %d), want (%v, %v, %v, %d)",
					op, left, right, dst, BinAdd, SrcLocal, SrcConst, index)
			}

			c := EncodeJumpBinaryAt(BinLtEq, SrcGlobal, SrcCapture, index)
			op, left, right, got := DecodeJumpBinaryAt(c)
			if op != BinLtEq || left != SrcGlobal || right != SrcCapture || got != index {
				t.Fatalf("JumpBinaryAt round trip = (%v, %v, %v, %d), want (%v, %v, %v, %d)",
					op, left, right, got, BinLtEq, SrcGlobal, SrcCapture, index)
			}
		})
	}
}
