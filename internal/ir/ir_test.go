package ir

import (
	"encoding/json"
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
