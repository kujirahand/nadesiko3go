package value

import "testing"

func TestArray(t *testing.T) {
	v := NewValueArray()
	v.Append(NewValueIntPtr(1))
	v.Append(NewValueIntPtr(2))
	v.Append(NewValueIntPtr(3))
	s := v.ToString()
	if s != "[1,2,3]" {
		t.Errorf("array.Append %s", s)
	}
}
