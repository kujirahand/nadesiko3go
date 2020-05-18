package node

import (
	"nako3/value"
)

type NodeType int

const (
	Nop NodeType = iota
	TNodeList
	Const
	Operator
	Sentence
	Word
	CallFunc
)

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
