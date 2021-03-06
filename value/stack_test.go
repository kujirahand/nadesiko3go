package value

import (
	"testing"
)

func TestStackSimple(t *testing.T) {
	stack := NewValueStack()
	stack.Push(NewIntPtr(30))
	stack.Push(NewIntPtr(50))
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
	stack.Push(NewStrPtr("abc"))
	v1 := stack.Pop()
	if v1.ToString() != "abc" {
		t.Errorf("stack.push/pop")
	}
	stack.Push(NewStrPtr("ccc"))
	stack.Push(NewStrPtr("bbb"))
	stack.Push(NewStrPtr("aaa"))
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

func TestStackPushPop2(t *testing.T) {
	stack := NewValueStack()
	stack.Push(NewIntPtr(1))
	stack.Push(NewIntPtr(2))
	stack.Push(NewIntPtr(3))

	v1 := stack.Pop()
	if v1.ToInt() != 3 {
		t.Errorf("stack.push/pop 3 != %d", v1.ToInt())
	}
	v2 := stack.Pop()
	if v2.ToInt() != 2 {
		t.Errorf("stack.push.pop 2 != %d", v2.ToInt())
	}

	stack.Push(NewIntPtr(2))
	stack.Push(NewIntPtr(3))
	v3 := stack.Pop()
	if v3.ToInt() != 3 {
		t.Errorf("stack.push/pop 3 != %d", v1.ToInt())
	}

	if stack.Length() != 2 {
		t.Errorf("stack.length != %d", stack.Length())
	}
}
