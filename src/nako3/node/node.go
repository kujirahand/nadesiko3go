package node

import (
	"nako3/value"
)

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

type Node interface {
	GetType() NodeType
}

// NodeList
type NodeList []Node

func (n NodeList) GetType() NodeType { return TNodeList }

// NodeNop
type NodeNop struct {
	Node
}

func (n NodeNop) GetType() NodeType { return Nop }

func NewNodeNop() Node {
	n := NodeNop{}
	return n
}

// NodeConst
type NodeConst struct {
	Node
	Value value.Value
}

func (n NodeConst) GetType() NodeType { return Const }

func NewNodeConst(vtype value.ValueType, s string) NodeConst {
	node := NodeConst{
		Value: value.NewValue(vtype, s),
	}
	return node
}

// NodeOperator
type NodeOperator struct {
	Node
	Left     Node
	Right    Node
	Operator string
}

func (n NodeOperator) GetType() NodeType { return Operator }

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
	List NodeList
}

func (n NodeSentence) GetType() NodeType { return Sentence }

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
	Args  NodeList
	Name  string
	Cache *value.Value
}

func (n NodeCallFunc) GetType() NodeType { return CallFunc }

func NewNodeCallFunc(name string) NodeCallFunc {
	node := NodeCallFunc{Name: name}
	node.Args = []Node{}
	return node
}

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
		s += "\n" + ss
	}
	return indent + s
}
