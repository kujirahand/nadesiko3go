package value

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func TestValueSize(t *testing.T) {
	if got := unsafe.Sizeof(Value{}); got != 32 {
		t.Fatalf("Valueのサイズ = %d bytes, want 32", got)
	}
}

func TestValueReferenceKinds(t *testing.T) {
	tests := []struct {
		name string
		got  Value
		kind Kind
		text string
	}{
		{"空文字列", String(""), KindString, ""},
		{"日本語と補助平面文字", String("なでしこ𩸽"), KindString, "なでしこ𩸽"},
		{"配列", ArrayValue(NewArray(Number(1))), KindArray, ""},
		{"辞書", DictValue(NewDict()), KindDict, ""},
		{"関数", FuncValue(&Func{ID: 3}), KindFunc, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got.Kind() != tt.kind {
				t.Fatalf("Kind() = %v, want %v", tt.got.Kind(), tt.kind)
			}
			switch tt.kind {
			case KindString:
				got, ok := tt.got.String()
				if !ok || got != tt.text {
					t.Fatalf("String() = %q, %v, want %q, true", got, ok, tt.text)
				}
			case KindArray:
				if got, ok := tt.got.Array(); !ok || got == nil || got.Len() != 1 {
					t.Fatalf("Array() = %v, %v", got, ok)
				}
			case KindDict:
				if got, ok := tt.got.Dict(); !ok || got == nil {
					t.Fatalf("Dict() = %v, %v", got, ok)
				}
			case KindFunc:
				if got, ok := tt.got.Func(); !ok || got == nil || got.ID != 3 {
					t.Fatalf("Func() = %v, %v", got, ok)
				}
			}
		})
	}
}

func TestStringValueKeepsBackingBytesAlive(t *testing.T) {
	want := strings.Repeat("なでしこ𩸽", 100)
	v := String(strings.Clone(want))
	runtime.GC()
	got, ok := v.String()
	if !ok || got != want {
		t.Fatalf("String() after GC = %q, %v", got, ok)
	}
}

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
