package value

// TNode : TNode struct
type TNode struct {
	v    *Value
	next *TNode
}

// TValueStack : TValueStack struct
type TValueStack struct {
	top    *TNode
	length int
}

// NewValueStack : get stack object
func NewValueStack() *TValueStack {
	stack := TValueStack{
		top:    nil,
		length: 0,
	}
	return &stack
}

// Push : push value
func (stack *TValueStack) Push(val *Value) int {
	// new node
	n := TNode{
		v:    val,
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

// Pop : pop value
func (stack *TValueStack) Pop() *Value {
	if stack.length == 0 {
		return nil
	}
	n := stack.top
	stack.top = n.next
	stack.length--
	return n.v
}

// Length : get stack length
func (stack *TValueStack) Length() int {
	return stack.length
}
