package value

import "testing"

func TestTArray(t *testing.T) {
	v := NewTArray()
	v.Append(NewIntPtr(1))
	v.Append(NewIntPtr(2))
	v.Append(NewIntPtr(3))
	s := v.ToString()
	if s != "[1,2,3]" {
		t.Errorf("TArray.Append %s", s)
	}
}

func TestArray2(t *testing.T) {
	rows := NewArrayPtr()
	cols1 := NewArrayPtr()
	cols1.Append(NewIntPtr(1))
	cols1.Append(NewIntPtr(2))
	cols1.Append(NewIntPtr(3))
	cols2 := NewArrayPtr()
	cols2.Append(NewIntPtr(4))
	cols2.Append(NewIntPtr(5))
	cols2.Append(NewIntPtr(6))
	rows.Append(cols1)
	rows.Append(cols2)
	s := rows.ToString()
	if s != "[[1,2,3],[4,5,6]]" {
		t.Errorf("array2.Append %s", s)
	}
}

func TestArray3(t *testing.T) {
	rows := NewArrayPtr()
	var cols1 *Value = nil
	v1 := NewArrayPtr()
	cols1 = v1
	cols1.Append(NewIntPtr(1))
	cols1.Append(NewIntPtr(2))
	cols1.Append(NewIntPtr(3))
	rows.Append(cols1)

	v2 := NewArrayPtr()
	cols1 = v2
	cols1.Append(NewIntPtr(4))
	cols1.Append(NewIntPtr(5))
	cols1.Append(NewIntPtr(6))
	rows.Append(cols1)
	s := rows.ToString()
	if s != "[[1,2,3],[4,5,6]]" {
		t.Errorf("array2.Append %s", s)
	}
}
