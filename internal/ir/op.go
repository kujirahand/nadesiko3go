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
	// OpBinaryAt applies a binary operator to two operands read straight from
	// where they live, and pushes the result. It is what the peephole pass
	// makes of 『Load;Load;Binary』: three dispatches and four stack
	// operations become one dispatch and one push.
	//
	// A packs the operator and the two operand kinds (→ EncodeBinaryAt),
	// B is the index of the left operand and C of the right.
	OpBinaryAt

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

// Src says where a fused instruction reads an operand from. It covers the
// places a value can be fetched without touching the operand stack, which is
// the whole point of fusing: 『それ』 などのシステム値は入れていない (フレーム
// ごとの読み分けが要り、素直な添字にならないため)。
type Src uint8

const (
	SrcConst   Src = iota // Consts[i]
	SrcLocal              // ローカルセル i
	SrcCapture            // 捕捉セル i
	SrcGlobal             // グローバルセル i
	// srcCount is how many operand sources there are, for range checks.
	srcCount
)

// Valid reports whether the id names an operand source.
func (s Src) Valid() bool { return s < srcCount }

func (s Src) String() string {
	switch s {
	case SrcConst:
		return "Const"
	case SrcLocal:
		return "Local"
	case SrcCapture:
		return "Capture"
	case SrcGlobal:
		return "Global"
	}
	return "Src?"
}

// The A operand of OpBinaryAt holds three small numbers side by side. Keeping
// the layout in these two functions means nothing else has to know it.
const (
	binaryAtOpBits   = 8
	binaryAtKindBits = 4
	binaryAtOpMask   int32 = 1<<binaryAtOpBits - 1
	binaryAtKindMask int32 = 1<<binaryAtKindBits - 1
)

// EncodeBinaryAt packs an operator and its two operand kinds into the A
// operand of an OpBinaryAt instruction.
func EncodeBinaryAt(op BinaryOp, left, right Src) int32 {
	return int32(op)&binaryAtOpMask |
		int32(left)&binaryAtKindMask<<binaryAtOpBits |
		int32(right)&binaryAtKindMask<<(binaryAtOpBits+binaryAtKindBits)
}

// DecodeBinaryAt unpacks what EncodeBinaryAt made.
func DecodeBinaryAt(a int32) (op BinaryOp, left, right Src) {
	return BinaryOp(a & binaryAtOpMask),
		Src(a >> binaryAtOpBits & binaryAtKindMask),
		Src(a >> (binaryAtOpBits + binaryAtKindBits) & binaryAtKindMask)
}

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
	OpBinary: "Binary", OpUnary: "Unary", OpBinaryAt: "BinaryAt",
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
