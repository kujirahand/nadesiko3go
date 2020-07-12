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

	v3 := NewArrayPtr()
	v3.Append(NewIntPtr(1))
	v3.Append(NewIntPtr(2))
	v3.Append(NewIntPtr(3))
	v3.ToArray().Reverse()
	s3 := v3.ToString()
	if s3 != "[3,2,1]" {
		t.Errorf("array3.Reverse %s", s3)
	}

	v4 := NewArrayPtr()
	v4.Append(NewIntPtr(1))
	v4.Append(NewIntPtr(2))
	v4.Append(NewIntPtr(3))
	v4.Append(NewIntPtr(4))
	v4.ToArray().Reverse()
	s4 := v4.ToString()
	if s4 != "[4,3,2,1]" {
		t.Errorf("array4.Reverse %s", s3)
	}
}
