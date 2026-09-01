package value

import "testing"

func TestArrayExtensionUsesUndefined(t *testing.T) {
	a := NewArray(Number(1))
	if !a.Set(3, Number(9)) {
		t.Fatal("配列を拡張できませんでした")
	}
	if a.Len() != 4 || a.Get(1).Kind() != KindUndefined || a.Get(2).Kind() != KindUndefined {
		t.Fatalf("疎な配列がundefinedで埋まりませんでした: %+v", a.Values())
	}
}

func TestDictPreservesInsertionOrder(t *testing.T) {
	d := NewDict()
	d.Set("x", Number(1))
	d.Set("y", Number(2))
	d.Set("x", Number(3))
	keys := d.Keys()
	if len(keys) != 2 || keys[0] != "x" || keys[1] != "y" {
		t.Fatalf("unexpected key order: %v", keys)
	}
}

func TestNumberUsesFloat64(t *testing.T) {
	v := Number(9007199254740993)
	n, ok := v.Number()
	if !ok || n != 9007199254740992 {
		t.Fatalf("JavaScript number境界と一致しません: %.0f", n)
	}
}
