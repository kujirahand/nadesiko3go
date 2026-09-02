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

	// OpLoadConst pushes Consts[A].
	OpLoadConst
	// OpLoadLocal pushes local cell A.
	OpLoadLocal
	// OpStoreLocal pops a value into local cell A. It fails on a constant.
	OpStoreLocal
	// OpInitLocal pops a value into local cell A once. Only a constant cell
	// is initialized this way, and only the first time.
	OpInitLocal
	// OpLoadCapture pushes captured cell A, which the enclosing frame shares.
	OpLoadCapture
	// OpStoreCapture pops a value into captured cell A.
	OpStoreCapture
	// OpLoadGlobal pushes global cell A.
	OpLoadGlobal
	// OpStoreGlobal pops a value into global cell A.
	OpStoreGlobal
	// OpInitGlobal pops a value into global cell A once.
	OpInitGlobal
	// OpLoadSpecial pushes the system value A (『それ』『対象』など).
	OpLoadSpecial
	// OpStoreSpecial pops a value into the system value A.
	OpStoreSpecial

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

// Special identifies one of the values the language keeps outside the ordinary
// variable scopes.
//
// 『それ』 belongs to the running function, the way a local does. The others are
// shared by the whole program, because that is where the TypeScript version
// keeps them (its system variable table).
type Special uint16

const (
	SpecialSore         Special = iota // それ
	SpecialTarget                      // 対象
	SpecialTargetKey                   // 対象キー
	SpecialKaisu                       // 回数
	SpecialErrorMessage                // エラーメッセージ
	SpecialMatched                     // 抽出文字列
	// SpecialCount is how many system values there are, for range checks.
	SpecialCount
)

// SpecialNames gives each system value its nadesiko name.
var SpecialNames = [SpecialCount]string{
	SpecialSore: "それ", SpecialTarget: "対象", SpecialTargetKey: "対象キー",
	SpecialKaisu: "回数", SpecialErrorMessage: "エラーメッセージ", SpecialMatched: "抽出文字列",
}

// SpecialByName finds a system value by name.
func SpecialByName(name string) (Special, bool) {
	for i, n := range SpecialNames {
		if n == name {
			return Special(i), true
		}
	}
	return 0, false
}

// IsFrameSpecial reports whether the value belongs to the running function
// rather than to the program as a whole.
func (s Special) IsFrameSpecial() bool {
	switch s {
	case SpecialSore, SpecialTarget, SpecialTargetKey, SpecialKaisu, SpecialErrorMessage:
		return true
	default:
		return false
	}
}

// Valid reports whether the id names a system value.
func (s Special) Valid() bool { return s < SpecialCount }

func (s Special) String() string {
	if s.Valid() {
		return SpecialNames[s]
	}
	return "Special?"
}

// UnaryOp identifies the operator an OpUnary instruction applies.
type UnaryOp uint16

const (
	UnaryNot UnaryOp = iota // not
	UnaryNeg                // 単項マイナス
)

// opNames gives each opcode a readable name for disassembly and errors.
var opNames = map[Op]string{
	OpNop: "Nop", OpLoadConst: "LoadConst",
	OpLoadLocal: "LoadLocal", OpStoreLocal: "StoreLocal", OpInitLocal: "InitLocal",
	OpLoadCapture: "LoadCapture", OpStoreCapture: "StoreCapture",
	OpLoadGlobal: "LoadGlobal", OpStoreGlobal: "StoreGlobal", OpInitGlobal: "InitGlobal",
	OpLoadSpecial: "LoadSpecial", OpStoreSpecial: "StoreSpecial",
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
