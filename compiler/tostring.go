package compiler

import (
	"fmt"
)

var codeNames = map[int]string{
	NOP:             "NOP",
	MoveR:           "MoveR",
	ConstO:          "ConstO",
	SetLocal:        "SetLocal",
	GetLocal:        "GetLocal",
	SetSore:         "SetSore",
	FindVar:         "FindVar",
	Add:             "Add",
	Sub:             "Sub",
	Mul:             "Mul",
	Div:             "Div",
	Mod:             "Mod",
	Gt:              "Gt",
	GtEq:            "GtEq",
	Lt:              "Lt",
	LtEq:            "LtEq",
	EqEq:            "EqEq",
	NtEq:            "NtEq",
	Exp:             "Exp",
	And:             "And",
	Or:              "Or",
	Jump:            "Jump",
	JumpIfTrue:      "JumpIfTrue",
	JumpLabel:       "JumpLabel",
	JumpLabelIfTrue: "JumpLabelIfTrue",
	DefLabel:        "DefLabel",
	IncLocal:        "IncLocal",
	IncReg:          "IncReg",
	NotReg:          "NotReg",
	NewArray:        "NewArray",
	SetArrayElem:    "SetArrayElem",
	AppendArray:     "AppendArray",
	NewHash:         "NewHash",
	SetHash:         "SetHash",
	GetArrayElem:    "GetArrayElem",
	GetArrayElemI:   "GetArrayElemI",
	CallFunc:        "CallFunc",
	CallUserFunc:    "CallUserFunc",
	Return:          "Return",
	Length:          "Length",
	Foreach:         "Foreach",
	ExString:        "ExString",
	FileInfo:        "FileInfo",
	Print:           "Print",
}

// ToString : コードを返す
func (p *TCompiler) ToString(code *TCode) string {
	typeName := codeNames[code.Type]
	if typeName == "" {
		typeName = fmt.Sprintf("(TypeUnknown:%d)", code.Type)
	}
	param := fmt.Sprintf("%d,%d,%d", code.A, code.B, code.C)
	desc := code.Memo + " "
	switch code.Type {
	}
	return fmt.Sprintf("%12s %s %s", typeName, param, desc)
}

// CodesToString : コード一覧を文字列にする
func (p *TCompiler) CodesToString(codes []*TCode) string {
	res := ""
	for i, v := range codes {
		res += fmt.Sprintf("%03d: %s\n", i, p.ToString(v))
	}
	return res
}
