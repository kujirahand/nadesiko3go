// Package ast defines the syntax tree the parser builds (nako_ast.mts
// equivalent).
//
// Node is one flat struct rather than an interface hierarchy. That is a
// deliberate trade: the tree is compared field by field against the TypeScript
// parser's output during development, and a shape that mirrors nako_ast.mts
// keeps that comparison honest. Only the compiler reads these nodes, and it
// switches on Type, so the looseness does not leak past internal/compiler.
package ast

import "github.com/kujirahand/nadesiko3go/internal/lexer"

// NodeType names a kind of syntax tree node.
type NodeType string

const (
	Nop                NodeType = "nop"
	EOL                NodeType = "eol"
	Comment            NodeType = "comment"
	Number             NodeType = "number"
	BigInt             NodeType = "bigint"
	Bool               NodeType = "bool"
	Null               NodeType = "null"
	Word               NodeType = "word"
	String             NodeType = "string"
	Block              NodeType = "block"
	End                NodeType = "end"
	If                 NodeType = "if"
	While              NodeType = "while"
	Atohantei          NodeType = "atohantei"
	For                NodeType = "for"
	Foreach            NodeType = "foreach"      // 反復
	RepeatTimes        NodeType = "repeat_times" // N回
	Switch             NodeType = "switch"
	TryExcept          NodeType = "try_except"
	DefFunc            NodeType = "def_func"
	Return             NodeType = "return"
	Continue           NodeType = "continue"
	Break              NodeType = "break"
	DefTest            NodeType = "def_test"
	Let                NodeType = "let"
	LetProp            NodeType = "let_prop" // #1793
	LetArray           NodeType = "let_array"
	JSONArray          NodeType = "json_array"
	JSONObj            NodeType = "json_obj"
	Op                 NodeType = "op"
	Calc               NodeType = "calc"
	Variable           NodeType = "variable"
	Not                NodeType = "not"
	And                NodeType = "and"
	Or                 NodeType = "or"
	Eq                 NodeType = "eq"
	Inc                NodeType = "inc"
	Func               NodeType = "func"
	CalcFunc           NodeType = "calc_func"
	CallValue          NodeType = "call_value"
	FuncPointer        NodeType = "func_pointer"
	FuncObj            NodeType = "func_obj"
	Renbun             NodeType = "renbun"
	DefLocalVar        NodeType = "def_local_var"
	DefLocalVarList    NodeType = "def_local_varlist"
	RefArray           NodeType = "ref_array"       // 配列参照
	RefProp            NodeType = "ref_prop"        // #1793 プロパティ参照
	RefArrayValue      NodeType = "ref_array_value" // 配列参照演算子
	Require            NodeType = "require"
	PerformanceMonitor NodeType = "performance_monitor"
	SpeedMode          NodeType = "speed_mode"
	RunMode            NodeType = "run_mode"
)

// SourceMap locates a node in the source. Offsets are rune offsets.
type SourceMap struct {
	Line   int
	Column int
	File   string
	Offset int
	Length int
}

// Node is a syntax tree node. Which fields carry meaning depends on Type; the
// comments name the node types that use each one.
type Node struct {
	Type NodeType
	SourceMap

	// Josi is the particle that followed the expression this node came from.
	// The parser matches arguments to parameters by it.
	Josi    string
	RawJosi string

	// Blocks holds the child nodes. Their meaning is positional and differs
	// per Type; see the comments on the constructor helpers in the parser.
	Blocks []*Node

	// Name identifies what the node refers to. It is a plain string for most
	// nodes, but stays a token for the ones whose error messages quote the
	// original word.
	Name      string
	NameToken *lexer.Token

	// Value carries a literal: float64, string, or bool.
	Value any

	// Index holds the subscripts of an array or property access.
	Index []*Node

	Operator string  // op
	Args     []*Node // def_func, def_test
	Names    []*Node // def_local_varlist
	VarType  string  // def_local_var / def_local_varlist: 「変数」か「定数」
	Word     string  // for, foreach: ループ変数名。使わないときは空
	Comment  string  // eol, comment

	// Meta describes the function a def_func defines or a func calls.
	Meta *lexer.FuncItem

	IsExport  bool // def_func
	AsyncFn   bool // def_func, func
	Setter    bool // func: 関数の定義側かどうか
	CheckInit bool // let_array: DNCLモードで配列を自動初期化するか
	CaseCount int  // switch: caseの数

	// FlagDown, FlagUp and LoopDirection describe which way a for loop counts.
	FlagDown      bool
	FlagUp        bool
	LoopDirection string // "", "up", "down"

	// End marks where a block-closing token was, for error messages.
	End *SourceMap

	Options map[string]bool
}

// NewNode builds a node of the given type at the position of tok.
func NewNode(t NodeType, tok *lexer.Token) *Node {
	n := &Node{Type: t}
	if tok != nil {
		n.SourceMap = SourceMap{
			Line: tok.Line, Column: tok.Column, File: tok.File,
			Offset: tok.Offset, Length: tok.Length,
		}
	}
	return n
}

// Block returns Blocks[i], or nil when there is no such child.
func (n *Node) Block(i int) *Node {
	if n == nil || i < 0 || i >= len(n.Blocks) {
		return nil
	}
	return n.Blocks[i]
}
