package value

import "testing"

func TestTArray(t *testing.T) {
	v := NewTArray()
	v.Append(NewValueIntPtr(1))
	v.Append(NewValueIntPtr(2))
	v.Append(NewValueIntPtr(3))
	s := v.ToString()
	if s != "[1,2,3]" {
		t.Errorf("TArray.Append %s", s)
	}
}

func TestArray2(t *testing.T) {
	rows := NewValueArrayPtr()
	cols1 := NewValueArrayPtr()
	cols1.Append(NewValueIntPtr(1))
	cols1.Append(NewValueIntPtr(2))
	cols1.Append(NewValueIntPtr(3))
	cols2 := NewValueArrayPtr()
	cols2.Append(NewValueIntPtr(4))
	cols2.Append(NewValueIntPtr(5))
	cols2.Append(NewValueIntPtr(6))
	rows.Append(cols1)
	rows.Append(cols2)
	s := rows.ToString()
	if s != "[[1,2,3],[4,5,6]]" {
		t.Errorf("array2.Append %s", s)
	}
}

func TestArray3(t *testing.T) {
	rows := NewValueArrayPtr()
	var cols1 *Value = nil
	v1 := NewValueArrayPtr()
	cols1 = v1
	cols1.Append(NewValueIntPtr(1))
	cols1.Append(NewValueIntPtr(2))
	cols1.Append(NewValueIntPtr(3))
	rows.Append(cols1)

	v2 := NewValueArrayPtr()
	cols1 = v2
	cols1.Append(NewValueIntPtr(4))
	cols1.Append(NewValueIntPtr(5))
	cols1.Append(NewValueIntPtr(6))
	rows.Append(cols1)
	s := rows.ToString()
	if s != "[[1,2,3],[4,5,6]]" {
		t.Errorf("array2.Append %s", s)
	}
}
