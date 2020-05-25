package node

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/token"
	"github.com/kujirahand/nadesiko3go/value"
)

// NType : Nodeの種類
type NType int

const (
	// Nop : 何もしない
	Nop NType = iota
	// TypeNodeList : Nodeのリスト
	TypeNodeList
	// Const : 定数
	Const
	// Operator : 演算子
	Operator
	// Sentence : 文
	Sentence
	// Word : 変数など
	Word
	// CallFunc : 関数呼び出し
	CallFunc
	// Calc : カッコ
	Calc
	// Let : 代入文
	Let
	// If : もし
	If
	// Repeat : n回
	Repeat
	// For : 繰り返す
	For
	// While : 条件繰り返し
	While
	// Continue : 続ける
	Continue
	// Break : 抜ける
	Break
	// DefFunc : 関数定義
	DefFunc
	// Return : 戻る
	Return
	// JSONArray : array
	JSONArray
	// JSONHash : hash
	JSONHash
)

var nodeTypeNames = map[NType]string{
	Nop:          "Nop",
	TypeNodeList: "TypeNodeList",
	Const:        "値",
	Operator:     "演算",
	Sentence:     "文",
	Word:         "変数",
	CallFunc:     "関数呼出",
	Calc:         "計算",
	Let:          "代入",
	If:           "もし",
	Repeat:       "回",
	For:          "繰返",
	While:        "間",
	Continue:     "続",
	Break:        "抜",
	DefFunc:      "関数",
	Return:       "戻る",
	JSONArray:    "JSONArray",
	JSONHash:     "JSONHash",
}

// Node : Node Interface
type Node interface {
	GetType() NType
	GetJosi() string
	GetFileInfo() core.TFileInfo
}

// TNodeList : Node List
type TNodeList []Node

func (n TNodeList) GetType() NType              { return TypeNodeList }
func (n TNodeList) GetFileInfo() core.TFileInfo { return core.TFileInfo{} }
func (n TNodeList) GetJosi() string             { return "" }

func NewNodeList() TNodeList {
	return TNodeList{}
}

// UserFunc : ユーザー関数の一覧
var UserFunc = map[int]Node{}

// NodeNop : NOP
type NodeNop struct {
	Node
	Comment  string
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeNop) GetType() NType              { return Nop }
func (n NodeNop) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeNop) GetJosi() string             { return n.Josi }

func NewNodeNop(t *token.Token) Node {
	n := NodeNop{
		Comment:  t.Literal,
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
	}
	return n
}

// NodeCalc : Calc
type NodeCalc struct {
	Node
	Josi     string
	Child    Node
	FileInfo core.TFileInfo
}

func (n NodeCalc) GetType() NType              { return Calc }
func (n NodeCalc) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeCalc) GetJosi() string             { return n.Josi }

func NewNodeCalc(t *token.Token, child Node) Node {
	n := NodeCalc{
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
		Child:    child,
	}
	return n
}

// NodeLet :
type NodeLet struct {
	Node
	Var      string
	VarIndex TNodeList
	Value    Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeLet) GetType() NType              { return Let }
func (n NodeLet) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeLet) GetJosi() string             { return n.Josi }

func NewNodeLet(t *token.Token, index TNodeList, value Node) Node {
	n := NodeLet{
		Var:      t.Literal,
		VarIndex: index,
		Value:    value,
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
	}
	return n
}

// NodeConst : Const
type NodeConst struct {
	Node
	Value    value.Value
	IsExtend bool
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeConst) GetType() NType              { return Const }
func (n NodeConst) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeConst) GetJosi() string             { return n.Josi }

func NewNodeConst(vtype value.Type, t *token.Token) NodeConst {
	node := NodeConst{
		Value:    value.NewValueByType(vtype, t.Literal),
		IsExtend: false,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}
func NewNodeConstInt(t *token.Token, num int) NodeConst {
	node := NodeConst{
		Value:    value.NewValueInt(int64(num)),
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}
func NewNodeConstEx(vtype value.Type, t *token.Token) NodeConst {
	node := NewNodeConst(vtype, t)
	node.IsExtend = true
	return node
}

// NodeOperator
type NodeOperator struct {
	Node
	Left     Node
	Right    Node
	Operator string
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeOperator) GetType() NType              { return Operator }
func (n NodeOperator) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeOperator) GetJosi() string             { return n.Josi }

func NewNodeOperator(op *token.Token, left Node, right Node) NodeOperator {
	p := NodeOperator{
		Left:     left,
		Right:    right,
		Operator: op.Literal,
		Josi:     right.GetJosi(),
		FileInfo: left.GetFileInfo(),
	}
	return p
}

// NodeSentence
type NodeSentence struct {
	Node
	Memo     string
	List     TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeSentence) GetType() NType              { return Sentence }
func (n NodeSentence) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeSentence) GetJosi() string             { return n.Josi }

func NewNodeSentence(finfo core.TFileInfo) NodeSentence {
	node := NodeSentence{
		FileInfo: finfo,
	}
	node.List = TNodeList{}
	return node
}

func (l *NodeSentence) Append(node Node) {
	l.List = append(l.List, node)
}

// NodeCallFunc
type NodeCallFunc struct {
	Node
	Args     TNodeList
	Name     string
	Cache    *value.Value
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeCallFunc) GetType() NType              { return CallFunc }
func (n NodeCallFunc) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeCallFunc) GetJosi() string             { return n.Josi }

func NewNodeCallFunc(t *token.Token) NodeCallFunc {
	node := NodeCallFunc{
		Name:     t.Literal,
		FileInfo: t.FileInfo,
	}
	node.Args = []Node{}
	return node
}

// NodeWord : 変数
type NodeWord struct {
	Node
	Name     string
	Cache    *value.Value
	Index    TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeWord) GetType() NType              { return Word }
func (n NodeWord) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeWord) GetJosi() string             { return n.Josi }

func NewNodeWord(t *token.Token, index TNodeList) NodeWord {
	node := NodeWord{
		Name:     t.Literal,
		Index:    index,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeIf : もし
type NodeIf struct {
	Node
	Expr      Node
	TrueNode  Node
	FalseNode Node
	Josi      string
	FileInfo  core.TFileInfo
}

func (n NodeIf) GetType() NType              { return If }
func (n NodeIf) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeIf) GetJosi() string             { return n.Josi }

func NewNodeIf(t *token.Token, nExpr, nTrue, nFalse Node) NodeIf {
	node := NodeIf{
		Expr:      nExpr,
		TrueNode:  nTrue,
		FalseNode: nFalse,
		Josi:      t.Josi,
		FileInfo:  t.FileInfo,
	}
	return node
}

// NodeFor : 繰り返す
type NodeFor struct {
	Node
	LoopId   int
	Word     string
	FromNode Node
	ToNode   Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeFor) GetType() NType              { return For }
func (n NodeFor) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeFor) GetJosi() string             { return n.Josi }

func NewNodeFor(t *token.Token, hensu string, nFrom, nTo, block Node) NodeFor {
	node := NodeFor{
		Word:     hensu,
		FromNode: nFrom,
		ToNode:   nTo,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeWhile: 繰り返す
type NodeWhile struct {
	Node
	LoopId   int
	Expr     Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeWhile) GetType() NType              { return While }
func (n NodeWhile) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeWhile) GetJosi() string             { return n.Josi }

func NewNodeWhile(t *token.Token, expr, block Node) NodeWhile {
	node := NodeWhile{
		Expr:     expr,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeRepeat : 回
type NodeRepeat struct {
	Node
	LoopId   int
	Expr     Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeRepeat) GetType() NType              { return Repeat }
func (n NodeRepeat) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeRepeat) GetJosi() string             { return n.Josi }

func NewNodeRepeat(t *token.Token, nExpr, block Node) NodeRepeat {
	node := NodeRepeat{
		Expr:     nExpr,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeContinue : Continue
type NodeContinue struct {
	Node
	LoopId   int
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeContinue) GetType() NType              { return Continue }
func (n NodeContinue) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeContinue) GetJosi() string             { return n.Josi }

func NewNodeContinue(t *token.Token, loopId int) NodeContinue {
	node := NodeContinue{
		LoopId:   loopId,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeBreak : Break
type NodeBreak struct {
	Node
	LoopId   int
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeBreak) GetType() NType              { return Break }
func (n NodeBreak) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeBreak) GetJosi() string             { return n.Josi }

func NewNodeBreak(t *token.Token, loopId int) NodeBreak {
	node := NodeBreak{
		LoopId:   loopId,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeDefFunc : NodeDefFunc
type NodeDefFunc struct {
	Node
	Word     string
	Args     TNodeList
	ArgNames []string
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeDefFunc) GetType() NType              { return DefFunc }
func (n NodeDefFunc) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeDefFunc) GetJosi() string             { return n.Josi }

func NewNodeDefFunc(t *token.Token, args Node, block Node) NodeDefFunc {
	word := t.Literal
	node := NodeDefFunc{
		Word:     word,
		Args:     args.(TNodeList),
		ArgNames: []string{},
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	// Analize Args
	a := [][]string{}
	m := map[string]int{}
	cnt := 0
	for _, v := range node.Args {
		nw := v.(NodeWord)
		i, ok := m[nw.Name]
		if !ok {
			m[nw.Name] = cnt
			cnt++
			a = append(a, []string{nw.Josi})
			node.ArgNames = append(node.ArgNames, nw.Name)
		} else {
			a[i] = append(a[i], nw.Josi)
		}
	}
	// Add System
	funcID := core.GetSystem().AddUserFunc(word, a)
	UserFunc[funcID] = node
	return node
}

// NodeReturn : Return
type NodeReturn struct {
	Node
	Arg      Node
	LoopId   int
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeReturn) GetType() NType              { return Return }
func (n NodeReturn) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeReturn) GetJosi() string             { return n.Josi }

func NewNodeReturn(t *token.Token, arg Node, loopId int) NodeReturn {
	node := NodeReturn{
		LoopId:   loopId,
		Arg:      arg,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NodeJSONArray : NodeJSONArray
type NodeJSONArray struct {
	Node
	Items    TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeJSONArray) GetType() NType              { return JSONArray }
func (n NodeJSONArray) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeJSONArray) GetJosi() string             { return n.Josi }

func NewNodeJSONArray(t *token.Token, items TNodeList) NodeJSONArray {
	node := NodeJSONArray{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

type JSONHashKeyValue map[string]Node

// NodeJSONHash : NodeJSONHash
type NodeJSONHash struct {
	Node
	Items    JSONHashKeyValue
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeJSONHash) GetType() NType              { return JSONHash }
func (n NodeJSONHash) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeJSONHash) GetJosi() string             { return n.Josi }

func NewNodeJSONHash(t *token.Token, items JSONHashKeyValue) NodeJSONHash {
	node := NodeJSONHash{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// ---

// NodeToString : Nodeの値をデバッグ用に出力する
func NodeToString(n Node, level int) string {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "|-"
	}
	s := fmt.Sprintf("%03d: %s {%s}",
		n.GetFileInfo().Line,
		nodeTypeNames[n.GetType()],
		n.GetJosi())
	ss := ""
	switch n.GetType() {
	case Nop:
		np := n.(NodeNop)
		s += " // " + np.Comment
	case Word:
		nw := n.(NodeWord)
		s += nw.Name
	case Const:
		cv := n.(NodeConst).Value
		s += "(" + cv.ToString() + ")"
	case Operator:
		s += ":" + n.(NodeOperator).Operator
		l := n.(NodeOperator).Left
		r := n.(NodeOperator).Right
		ss += NodeToString(l, level+1) + "\n"
		ss += NodeToString(r, level+1) + "\n"
	case Sentence:
		s += n.(NodeSentence).Memo
		for _, v := range n.(NodeSentence).List {
			ss += NodeToString(v, level+1) + "\n"
		}
	case TypeNodeList:
		nlist := n.(TNodeList)
		s += fmt.Sprintf("(%d)", len(nlist))
		for _, v := range nlist {
			ss += NodeToString(v, level+1) + "\n"
		}
	case CallFunc:
		s += ":" + n.(NodeCallFunc).Name
		for _, v := range n.(NodeCallFunc).Args {
			ss += NodeToString(v, level+1) + "\n"
		}
	case Calc:
		nc := n.(NodeCalc)
		ss += NodeToString(nc.Child, level+1) + "\n"
	case Let:
		nl := n.(NodeLet)
		s += " " + nl.Var
		if len(nl.VarIndex) > 0 {
			s += fmt.Sprintf("[]*%d", len(nl.VarIndex))
			for _, v := range nl.VarIndex {
				ss += NodeToString(v, level+1)
			}
		}
		ss += NodeToString(nl.Value, level+1) + "\n"
	case If:
		ni := n.(NodeIf)
		ss += NodeToString(ni.Expr, level+1) + "\n"
		ss += NodeToString(ni.TrueNode, level+1) + "\n"
		ss += NodeToString(ni.FalseNode, level+1) + "\n"
	case For:
		ni := n.(NodeFor)
		s += fmt.Sprintf("%s id=%d", ni.Word, ni.LoopId)
		ss += NodeToString(ni.FromNode, level+1) + "\n"
		ss += NodeToString(ni.ToNode, level+1) + "\n"
		ss += NodeToString(ni.Block, level+1) + "\n"
	case Repeat:
		nn := n.(NodeRepeat)
		s += fmt.Sprintf(" id=%d", nn.LoopId)
		ss += NodeToString(nn.Expr, level+1) + "\n"
		ss += NodeToString(nn.Block, level+1) + "\n"
	case While:
		nn := n.(NodeWhile)
		s += fmt.Sprintf(" id=%d", nn.LoopId)
		ss += NodeToString(nn.Expr, level+1) + "\n"
		ss += NodeToString(nn.Block, level+1) + "\n"
	case Continue:
		s += fmt.Sprintf("id=%d", n.(NodeContinue).LoopId)
	case Break:
		s += fmt.Sprintf("id=%d", n.(NodeBreak).LoopId)
	case DefFunc:
		nn := n.(NodeDefFunc)
		s += fmt.Sprintf(" %s", nn.Word)
		ss += NodeToString(nn.Args, level+1) + "\n"
	case JSONArray:
		nn := n.(NodeJSONArray)
		ss += NodeToString(nn.Items, level+1) + "\n"
	default:
		s += " *"
	}
	if ss != "" {
		s += "\n" + strings.TrimRight(ss, " \t\n")
	}
	return indent + s
}
