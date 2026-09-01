package ir

// Op is an instruction opcode.
//
// The VM is a value-stack machine (AGENTS.md §6): expressions evaluate on an
// operand stack and local variables live in a slot array indexed by A. An
// instruction has only two operands, so anything of variable length is encoded
// as "the count goes in B, the values are already on the stack".
type Op uint16

const (
	OpNop Op = iota

	// --- 定数と変数 ---

	// OpConst pushes Consts[A].
	OpConst
	// OpLoadLocal pushes local slot A.
	OpLoadLocal
	// OpStoreLocal pops a value into local slot A.
	OpStoreLocal
	// OpLoadGlobal pushes global Consts[A] (the name).
	OpLoadGlobal
	// OpStoreGlobal pops a value into the global named by Consts[A].
	OpStoreGlobal

	// --- スタック操作 ---

	// OpPop discards the top of the stack.
	OpPop
	// OpDup duplicates the top of the stack.
	OpDup

	// --- 演算子 ---

	// OpBinary applies binary operator A to the two values on the stack.
	OpBinary
	// OpUnary applies unary operator A to the value on the stack.
	OpUnary

	// --- 集合 ---

	// OpMakeArray pops B values and pushes an array of them.
	OpMakeArray
	// OpMakeDict pops B key/value pairs and pushes a dictionary.
	OpMakeDict
	// OpIndexGet pops B indexes and a container, and pushes the element.
	OpIndexGet
	// OpIndexSet pops a value, B indexes and a container, and stores.
	OpIndexSet
	// OpIterKeys pops a container and pushes the array of keys to iterate:
	// the indexes of an array, or the keys of a dictionary in insertion order.
	OpIterKeys
	// OpLen pops an array and pushes its length.
	OpLen

	// --- 呼び出し ---

	// OpCallStd calls stdlib function A with B arguments from the stack.
	OpCallStd
	// OpCallUser calls Funcs[A] with B arguments from the stack.
	OpCallUser
	// OpCallValue calls the function value under B arguments on the stack.
	OpCallValue
	// OpMakeFunc pushes a function value referring to Funcs[A].
	OpMakeFunc
	// OpReturn returns the top of the stack from the current function.
	OpReturn

	// --- 制御 ---

	// OpJump continues at Code[A].
	OpJump
	// OpJumpIfFalse pops a value and continues at Code[A] when it is falsy.
	OpJumpIfFalse
	// OpJumpIfTrue pops a value and continues at Code[A] when it is truthy.
	OpJumpIfTrue

	// --- 例外 ---

	// OpTry starts an error-monitored region whose handler is at Code[A].
	OpTry
	// OpEndTry ends the innermost error-monitored region.
	OpEndTry
	// OpThrow raises the value on the stack as a runtime error.
	OpThrow
)

// BinaryOp identifies the operator an OpBinary instruction applies. The names
// match the parser's operator names so the compiler can map them directly.
type BinaryOp uint16

const (
	BinAdd         BinaryOp = iota // +  … 両辺を parseFloat してから足す
	BinSub                         // -
	BinMul                         // *
	BinDiv                         // ÷ /
	BinIntDiv                      // ÷÷
	BinMod                         // %
	BinPow                         // ^ **
	BinConcat                      // &  … 文字列連結
	BinEq                          // eq
	BinNotEq                       // noteq
	BinStrictEq                    // ===
	BinStrictNotEq                 // !==
	BinLt                          // lt
	BinLtEq                        // lteq
	BinGt                          // gt
	BinGtEq                        // gteq
	BinShiftL                      // shift_l
	BinShiftR                      // shift_r
	BinShiftR0                     // shift_r0
)

// UnaryOp identifies the operator an OpUnary instruction applies.
type UnaryOp uint16

const (
	UnaryNot UnaryOp = iota // not
	UnaryNeg                // 単項マイナス
)

// opNames gives each opcode a readable name for disassembly and errors.
var opNames = map[Op]string{
	OpNop: "Nop", OpConst: "Const",
	OpLoadLocal: "LoadLocal", OpStoreLocal: "StoreLocal",
	OpLoadGlobal: "LoadGlobal", OpStoreGlobal: "StoreGlobal",
	OpPop: "Pop", OpDup: "Dup",
	OpBinary: "Binary", OpUnary: "Unary",
	OpMakeArray: "MakeArray", OpMakeDict: "MakeDict",
	OpIndexGet: "IndexGet", OpIndexSet: "IndexSet",
	OpIterKeys: "IterKeys", OpLen: "Len",
	OpCallStd: "CallStd", OpCallUser: "CallUser", OpCallValue: "CallValue",
	OpMakeFunc: "MakeFunc",
	OpReturn:   "Return",
	OpJump:     "Jump", OpJumpIfFalse: "JumpIfFalse", OpJumpIfTrue: "JumpIfTrue",
	OpTry: "Try", OpEndTry: "EndTry", OpThrow: "Throw",
}

func (o Op) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return "Op?"
}
