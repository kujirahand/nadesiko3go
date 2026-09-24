package value

import (
	"testing"
)

func TestArrayProps(t *testing.T) {
	arr := NewArray(Number(1), Number(2))
	if arr.Len() != 2 {
		t.Fatalf("expected len 2, got %d", arr.Len())
	}
	if arr.HasProp("key") {
		t.Fatal("expected HasProp false for missing prop")
	}
	if v := arr.GetProp("key"); v.Kind() != KindUndefined {
		t.Fatalf("expected Undefined, got %v", v)
	}

	arr.SetProp("key", String("value"))
	if !arr.HasProp("key") {
		t.Fatal("expected HasProp true")
	}
	if arr.Len() != 2 {
		t.Fatalf("expected len 2 after SetProp, got %d", arr.Len())
	}
	if v := arr.GetProp("key"); ToString(v) != "value" {
		t.Fatalf("expected 'value', got %v", ToString(v))
	}

	if !arr.DeleteProp("key") {
		t.Fatal("expected DeleteProp true")
	}
	if arr.HasProp("key") {
		t.Fatal("expected HasProp false after DeleteProp")
	}
}
