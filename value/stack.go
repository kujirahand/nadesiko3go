package value

// TNode : TNode struct
type TNode struct {
	v    *Value
	next *TNode
}

// ValueStack : ValueStack struct
type ValueStack struct {
	top    *TNode
	length int
}

// NewValueStack : get stack object
func NewValueStack() *ValueStack {
	stack := ValueStack{
		top:    nil,
		length: 0,
	}
	return &stack
}

func (stack *ValueStack) Push(val Value) int {
	// new node
	n := TNode{
		v:    &val,
		next: nil,
	}
	// add last
	if stack.length == 0 {
		stack.top = &n
	} else {
		top := stack.top
		stack.top = &n
		n.next = top
	}
	stack.length++
	return stack.length
}

func (stack *ValueStack) Pop() *Value {
	if stack.length == 0 {
		return nil
	}
	n := stack.top
	stack.top = n.next
	stack.length--
	return n.v
}

func (stack *ValueStack) Length() int {
	return stack.length
}
