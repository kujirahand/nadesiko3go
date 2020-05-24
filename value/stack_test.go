package value

import (
	"testing"
)

func TestStackSimple(t *testing.T) {
	stack := NewValueStack()
	stack.Push(NewValueInt(30))
	stack.Push(NewValueInt(50))
	v1 := stack.Pop()
	if v1.ToInt() != 50 {
		t.Errorf("stack.push/pop 50 != %d", v1.ToInt())
	}
	v2 := stack.Pop()
	if v2.ToInt() != 30 {
		t.Errorf("stack.push.pop 30 != %d", v2.ToInt())
	}
	if stack.Length() != 0 {
		t.Errorf("stack.length != %d", stack.Length())
	}
}

func TestStack2(t *testing.T) {
	stack := NewValueStack()
	// push & pop
	stack.Push(NewValueStr("abc"))
	v1 := stack.Pop()
	if v1.ToString() != "abc" {
		t.Errorf("stack.push/pop")
	}
	stack.Push(NewValueStr("ccc"))
	stack.Push(NewValueStr("bbb"))
	stack.Push(NewValueStr("aaa"))
	if stack.Pop().ToString() != "aaa" {
		t.Errorf("stack error aaa")
	}
	if stack.Pop().ToString() != "bbb" {
		t.Errorf("stack error bbb")
	}
	if stack.Pop().ToString() != "ccc" {
		t.Errorf("stack error ccc")
	}
}
