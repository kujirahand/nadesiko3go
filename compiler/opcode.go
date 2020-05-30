package compiler

// OPCODE 64bit (Type:8bit, A:16bit, B:16bit, C:16bit)
const (
	// NOP : 何もしない
	NOP = iota
	// MoveR A,B : R[A] = R[B]
	MoveR
	// ConstO A,B : R[A] = CONSTS[B]
	ConstO
	// SetGlobal A,B : Vars[ CONSTS[A] ] = R[B]
	SetGlobal
	// GetGlobal A,B : R[A] = Vars[ CONSTS[B] ]
	GetGlobal
	// SetLocal A, B: Scope[A] = R[B]
	SetLocal
	// GetLocal A,B : R[A] = Scope[B]
	GetLocal
	// FindVar A, B : R[A] = FindVar(CONSTS[B])
	FindVar
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
	// NotReg A : R[A] = !R[A]
	NotReg
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
	// CallFunc A B C : R[A] = call(fn=CONSTS[B], args=R[C])
	CallFunc
	// CallUserFunc A B C : R[A] = call(fn=LABELS[B], args=R[C])
	CallUserFunc
	// Retruen A : return R[A]
	Return
)

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
