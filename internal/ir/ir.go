// Package ir defines the versioned, serializable boundary between compiler and
// execution backends.
package ir

import "fmt"

const CurrentVersion = 1

type Program struct {
	Version   int          `json:"version"`
	Consts    []Const      `json:"consts"`
	Funcs     []Func       `json:"funcs"`
	Main      int          `json:"main"`
	Sources   []SourceFile `json:"sources"`
	Positions []SourcePos  `json:"positions"`
}

type ConstKind uint8

const (
	ConstUndefined ConstKind = iota
	ConstNull
	ConstBool
	ConstNumber
	ConstString
)

type Const struct {
	Kind ConstKind `json:"kind"`
	Bool bool      `json:"bool,omitempty"`
	Num  float64   `json:"num,omitempty"`
	Str  string    `json:"str,omitempty"`
}

type Func struct {
	Name    string  `json:"name"`
	Params  []Param `json:"params"`
	NumVars int     `json:"numVars"`
	Code    []Inst  `json:"code"`
	Async   bool    `json:"async"`
	Pure    bool    `json:"pure"`
}

type Param struct {
	Name      string   `json:"name"`
	Particles []string `json:"particles"`
}

type Inst struct {
	Op  Op  `json:"op"`
	A   int `json:"a"`
	B   int `json:"b"`
	Pos int `json:"pos"`
}

type SourceFile struct {
	Name string `json:"name"`
}

type SourcePos struct {
	Source int `json:"source"`
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (p Program) Validate() error {
	if p.Version != CurrentVersion {
		return fmt.Errorf("非対応のIRバージョンです: %d (対応: %d)", p.Version, CurrentVersion)
	}
	if p.Main < 0 || p.Main >= len(p.Funcs) {
		return fmt.Errorf("Mainが関数範囲外です: %d", p.Main)
	}
	for functionIndex, function := range p.Funcs {
		for instructionIndex, instruction := range function.Code {
			if instruction.Pos < 0 || instruction.Pos >= len(p.Positions) {
				return fmt.Errorf("Funcs[%d].Code[%d]のPosが範囲外です: %d", functionIndex, instructionIndex, instruction.Pos)
			}
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
	return nil
}
