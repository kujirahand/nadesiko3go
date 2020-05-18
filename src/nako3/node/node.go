package node

import (
	"nako3/core"
	"nako3/token"
	"nako3/value"
	"strings"
)

// NodeType : Nodeの種類
type NodeType int

const (
	Nop       = 0
	TNodeList = 1
	Const     = 2
	Operator  = 3
	Sentence  = 4
	Word      = 5
	CallFunc  = 6
)

var nodeTypeNames = [...]string{
	"Nop",
	"TNodeList",
	"Const",
	"Operator",
	"Sentence",
	"Word",
	"CallFunc",
}

// Node : Node Interface
type Node interface {
	GetType() NodeType
	GetFileInfo() core.TFileInfo
}

// NodeList : Node List
type NodeList []Node

func (n NodeList) GetType() NodeType           { return TNodeList }
func (n NodeList) GetFileInfo() core.TFileInfo { return core.TFileInfo{} }

// NodeNop : NOP
type NodeNop struct {
	Node
	FileInfo core.TFileInfo
}

func (n NodeNop) GetType() NodeType           { return Nop }
func (n NodeNop) GetFileInfo() core.TFileInfo { return n.FileInfo }

func NewNodeNop() Node {
	n := NodeNop{}
	return n
}

// NodeConst : Const
type NodeConst struct {
	Node
	Value    value.Value
	FileInfo core.TFileInfo
}

func (n NodeConst) GetType() NodeType           { return Const }
func (n NodeConst) GetFileInfo() core.TFileInfo { return n.FileInfo }

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
	FileInfo core.TFileInfo
}

func (n NodeOperator) GetType() NodeType           { return Operator }
func (n NodeOperator) GetFileInfo() core.TFileInfo { return n.FileInfo }

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
	FileInfo core.TFileInfo
}

func (n NodeSentence) GetType() NodeType           { return Sentence }
func (n NodeSentence) GetFileInfo() core.TFileInfo { return n.FileInfo }

func NewNodeSentence() NodeSentence {
	node := NodeSentence{}
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
	FileInfo core.TFileInfo
}

func (n NodeCallFunc) GetType() NodeType           { return CallFunc }
func (n NodeCallFunc) GetFileInfo() core.TFileInfo { return n.FileInfo }

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
	s := nodeTypeNames[int(n.GetType())]
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
	}
	if ss != "" {
		s += "\n" + strings.TrimRight(ss, " \t\n")
	}
	return indent + s
}
