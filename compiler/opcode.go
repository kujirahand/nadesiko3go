package compiler

// OPCODE
const (
	// NOP : 何もしない
	NOP = iota
	// FileInfo A, B : fileno:A, lineno:B
	FileInfo
	// MoveR A,B : R[A] = R[B]
	MoveR
	// ConstO A,B : R[A] = CONSTS[B]
	ConstO
	// ExString A,B : R[A] = ExString(CONSTS[B])
	ExString
	// SetGlobal A,B : Vars[ CONSTS[A] ] = R[B]
	SetGlobal
	// GetGlobal A,B : R[A] = scope.values[ CONSTS[B] ]
	GetGlobal
	// SetLocal A, B: scope.values[A] = R[B]
	SetLocal
	// GetLocal A,B : R[A] = Scope[B]
	GetLocal
	// FindVar A, B : R[A] = FindVar(CONSTS[B])
	FindVar
	// SetSore A : Sore = R[A]
	SetSore
	// IncReg A : R[A]++
	IncReg
	// IncLocal A : Scope[A]++
	IncLocal
	// Jump A : PC += A
	Jump
	// JumpTo A :  PC = A
	JumpTo
	// JumpIfTrue A, B: if A then PC += B
	JumpIfTrue
	// JumpLabel A : PC = LABELS[A]
	JumpLabel
	// JumpLabelIfTrue A B : if A then PC = LABELS[B]
	JumpLabelIfTrue
	// DefLabel A : LABELS[A] = addr
	DefLabel
	// Add A,B,C : R[A] = R[B] + R[C]
	Add
	// Sub A,B,C : R[A] = R[B] - R[C]
	Sub
	// Mul A,B,C : R[A] = R[B] * R[C]
	Mul
	// Div A,B,C : R[A] = R[B] * R[C]
	Div
	// Mod A,B,C : R[A] = R[B] % R[C]
	Mod
	// Gt A,B,C : R[A] = R[B] > R[C]
	Gt
	// GtEq A,B,C : R[A] = R[B] >= R[C]
	GtEq
	// Lt A,B,C : R[A] = R[B] < R[C]
	Lt
	// LtEq A,B,C : R[A] = R[B] <= R[C]
	LtEq
	// EqEq A,B,C : R[A] = R[B] == R[C]
	EqEq
	// NtEq A,B,C : R[A] = R[B] != R[C]
	NtEq
	// Exp A,B,C : R[A] = R[B] ^ R[C]
	Exp
	// And A,B,C : R[A] = R[B] && R[C]
	And
	// Or A,B,C : R[A] = R[B] || R[C]
	Or
	// NotReg A : R[A] = !R[A]
	NotReg
	// Length A B : R[A] = Len(R[B])
	Length
	// NewArray A : R[A] = NewArray
	NewArray
	// SetArrayElem A B : R[A].Value = R[B]
	SetArrayElem
	// AppendArray A B : (R[A]).append(R[B])
	AppendArray
	// GetArrayElem A B C : R[A] = (R[B])[ R[C] ]
	GetArrayElem
	// GetArrayElemI A B C : R[A] = (R[B])[ C ]
	GetArrayElemI
	// NewHash A : R[A] = NewHash
	NewHash
	// SetHash A B C : (R[A]).HashSet(CONSTS[B], C)
	SetHash
	// Foreach A B C : FOREACH isContinue:A expr:B counter:C -> それ|対象|対象キーの値を更新
	Foreach
	// CallFunc A B C : R[A] = call(fn=CONSTS[B], argStart=R[C])
	CallFunc
	// CallUserFunc A B C : R[A] = call(fn=LABELS[B], argStart=R[C])
	CallUserFunc
	// Retruen A : return R[A]
	Return
	// Print A : PRINT R[A] for DEBUG
	Print
)

// OPCODE:END

// TCode : コードを表す構造体
type TCode struct {
	Type int
	A    int
	B    int
	C    int
	Link *TCode
	Memo string
}

// NewCode : コードを生成
func NewCode(t, A, B, C int) *TCode {
	p := TCode{
		Type: t,
		A:    A,
		B:    B,
		C:    C,
	}
	return &p
}

// NewCodeMemo : NewCode with Memo
func NewCodeMemo(t, A, B, C int, memo string) *TCode {
	p := TCode{
		Type: t,
		A:    A,
		B:    B,
		C:    C,
		Memo: memo,
	}
	return &p
}
