package value

import "testing"

// #343: Truncate で切り捨てた要素への参照が基底配列に残らないことを確かめる。
// 参照が残ると、配列が生きている間は削除した大きな文字列などがGCされない。
func TestArrayTruncateClearsDiscardedSlots(t *testing.T) {
	a := NewArray(String("a"), String("b"), String("c"), String("d"))

	a.Truncate(2)

	if a.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", a.Len())
	}
	if got, _ := a.Get(0).String(); got != "a" {
		t.Fatalf("要素[0] = %q, want %q", got, "a")
	}
	if got, _ := a.Get(1).String(); got != "b" {
		t.Fatalf("要素[1] = %q, want %q", got, "b")
	}
	backing := a.items[:cap(a.items)]
	for i := 2; i < len(backing); i++ {
		if backing[i] != (Value{}) {
			t.Fatalf("基底配列[%d] = %#v, want ゼロ値（参照が残っている）", i, backing[i])
		}
	}
}

// #343: Truncate(0) ですべての要素への参照が消えることを確かめる。
func TestArrayTruncateToZeroClearsAllSlots(t *testing.T) {
	a := NewArray(String("a"), String("b"), String("c"))

	a.Truncate(0)

	if a.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", a.Len())
	}
	backing := a.items[:cap(a.items)]
	for i := range backing {
		if backing[i] != (Value{}) {
			t.Fatalf("基底配列[%d] = %#v, want ゼロ値（参照が残っている）", i, backing[i])
		}
	}
}

// #343: Remove で前に詰めた後に残る複製への参照が基底配列に残らないこと、
// 残存要素の順序と内容が維持されること、戻り値が削除要素を保持することを確かめる。
func TestArrayRemoveClearsDiscardedSlots(t *testing.T) {
	a := NewArray(Number(1), Number(2), Number(3), Number(4), Number(5))

	removed := a.Remove(1, 2)

	if len(removed) != 2 {
		t.Fatalf("len(removed) = %d, want 2", len(removed))
	}
	if n, _ := removed[0].Number(); n != 2 {
		t.Fatalf("removed[0] = %v, want 2", n)
	}
	if n, _ := removed[1].Number(); n != 3 {
		t.Fatalf("removed[1] = %v, want 3", n)
	}
	if a.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", a.Len())
	}
	for i, want := range []float64{1, 4, 5} {
		if n, _ := a.Get(i).Number(); n != want {
			t.Fatalf("要素[%d] = %v, want %v", i, n, want)
		}
	}
	backing := a.items[:cap(a.items)]
	for i := 3; i < len(backing); i++ {
		if backing[i] != (Value{}) {
			t.Fatalf("基底配列[%d] = %#v, want ゼロ値（参照が残っている）", i, backing[i])
		}
	}
}

// #343: Remove(0, Len()) で全要素を削除した場合も参照が残らないことを確かめる。
func TestArrayRemoveAllClearsAllSlots(t *testing.T) {
	a := NewArray(String("a"), String("b"), String("c"))

	removed := a.Remove(0, a.Len())

	if len(removed) != 3 {
		t.Fatalf("len(removed) = %d, want 3", len(removed))
	}
	if a.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", a.Len())
	}
	backing := a.items[:cap(a.items)]
	for i := range backing {
		if backing[i] != (Value{}) {
			t.Fatalf("基底配列[%d] = %#v, want ゼロ値（参照が残っている）", i, backing[i])
		}
	}
}
