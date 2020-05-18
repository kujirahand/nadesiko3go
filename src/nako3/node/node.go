package node

import (
	"nako3/core"
	"nako3/token"
	"nako3/value"
	"strings"
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
)

var nodeTypeNames = map[NType]string{
	Nop:          "Nop",
	TypeNodeList: "TypeNodeList",
	Const:        "Const",
	Operator:     "Operator",
	Sentence:     "Sentence",
	Word:         "Word",
	CallFunc:     "CallFunc",
	Calc:         "Calc",
}

// Node : Node Interface
type Node interface {
	GetType() NType
	GetJosi() string
	GetFileInfo() core.TFileInfo
}

// NodeList : Node List
type NodeList []Node

func (n NodeList) GetType() NType              { return TypeNodeList }
func (n NodeList) GetFileInfo() core.TFileInfo { return core.TFileInfo{} }
func (n NodeList) GetJosi() string             { return "" }

// NodeNop : NOP
type NodeNop struct {
	Node
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeNop) GetType() NType              { return Nop }
func (n NodeNop) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeNop) GetJosi() string             { return n.Josi }

func NewNodeNop(t *token.Token) Node {
	n := NodeNop{
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

// NodeConst : Const
type NodeConst struct {
	Node
	Value    value.Value
	Josi     string
	FileInfo core.TFileInfo
}

func (n NodeConst) GetType() NType              { return Const }
func (n NodeConst) GetFileInfo() core.TFileInfo { return n.FileInfo }
func (n NodeConst) GetJosi() string             { return n.Josi }

func NewNodeConst(vtype value.ValueType, t *token.Token) NodeConst {
	node := NodeConst{
		Value:    value.NewValue(vtype, t.Literal),
		FileInfo: t.FileInfo,
	}
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

func NewNodeOperator(op string, left Node, right Node) NodeOperator {
	p := NodeOperator{
		Left:     left,
		Right:    right,
		Operator: op,
	}
	return p
}

// NodeSentence
type NodeSentence struct {
	Node
	List     NodeList
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
	node.List = NodeList{}
	return node
}

func (l *NodeSentence) Append(node Node) {
	l.List = append(l.List, node)
}

// NodeCallFunc
type NodeCallFunc struct {
	Node
	Args     NodeList
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

// ---

// NodeToString : Nodeの値をデバッグ用に出力する
func NodeToString(n Node, level int) string {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "|-"
	}
	s := nodeTypeNames[n.GetType()] + "(" + n.GetJosi() + ")"
	ss := ""
	switch n.GetType() {
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
		for _, v := range n.(NodeSentence).List {
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
	}
	if ss != "" {
		s += "\n" + strings.TrimRight(ss, " \t\n")
	}
	return indent + s
}
