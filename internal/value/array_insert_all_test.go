package value

import (
	"testing"
)

// Issue #344: 『配列一括挿入』の高速化で追加した InsertAll の意味が
// 従来の要素ごと Insert (公式実装の要素ごと splice) と一致することを固定する。
// 負インデックスの期待値は公式 TypeScript 実装を Node で実行して得た実測値。
func TestArrayInsertAll(t *testing.T) {
	cases := []struct {
		name  string
		dst   []int
		index int
		src   []int
		want  []int
	}{
		{"先頭", []int{10, 20, 30, 40}, 0, []int{100, 200}, []int{100, 200, 10, 20, 30, 40}},
		{"中間", []int{10, 20, 30, 40}, 2, []int{100, 200}, []int{10, 20, 100, 200, 30, 40}},
		{"末尾", []int{10, 20, 30, 40}, 4, []int{100, 200}, []int{10, 20, 30, 40, 100, 200}},
		{"末尾超過は末尾", []int{10, 20, 30, 40}, 99, []int{100, 200}, []int{10, 20, 30, 40, 100, 200}},
		{"空の挿入元は何もしない", []int{1, 2}, 1, []int{}, []int{1, 2}},
		{"空配列へ挿入", []int{}, 0, []int{100, 200}, []int{100, 200}},
		{"負-1は逐次再評価", []int{10, 20, 30, 40}, -1, []int{100, 200}, []int{200, 10, 20, 30, 100, 40}},
		{"負-2は逐次再評価", []int{10, 20, 30, 40}, -2, []int{100, 200}, []int{10, 20, 100, 30, 200, 40}},
		{"負-4は逐次再評価", []int{10, 20, 30, 40}, -4, []int{100, 200}, []int{100, 10, 200, 20, 30, 40}},
		{"負-5は逐次再評価", []int{10, 20, 30, 40}, -5, []int{100, 200}, []int{100, 200, 10, 20, 30, 40}},
		{"負-100は逐次再評価", []int{10, 20, 30, 40}, -100, []int{100, 200}, []int{200, 100, 10, 20, 30, 40}},
		{"負-1で3要素は逐次再評価", []int{10, 20, 30, 40}, -1, []int{100, 200, 300}, []int{200, 300, 10, 20, 30, 100, 40}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			arr := NewArray(intValues(tt.dst)...)
			arr.InsertAll(tt.index, intValues(tt.src))
			assertIntArray(t, arr, tt.want)
		})
	}
}

// 自己挿入: 挿入元が自分自身でも、挿入前の要素列がそのまま挿入される。
// 期待値は公式実装の実測値 ([1,2,3] を自分自身へ挿入)。
func TestArrayInsertAllSelfInsert(t *testing.T) {
	cases := []struct {
		index int
		want  []int
	}{
		{0, []int{1, 2, 3, 1, 2, 3}},
		{1, []int{1, 1, 2, 3, 2, 3}},
		{-1, []int{2, 3, 1, 2, 1, 3}},
		{99, []int{1, 2, 3, 1, 2, 3}},
	}
	for _, tt := range cases {
		arr := NewArray(Number(1), Number(2), Number(3))
		// あえて自分自身の Values() を渡す。InsertAll 内の snapshot でも固定される。
		arr.InsertAll(tt.index, arr.Values())
		assertIntArray(t, arr, tt.want)
	}
}

func intValues(ns []int) []Value {
	out := make([]Value, len(ns))
	for i, n := range ns {
		out[i] = Number(float64(n))
	}
	return out
}

func assertIntArray(t *testing.T, arr *Array, want []int) {
	t.Helper()
	if arr.Len() != len(want) {
		t.Fatalf("len = %d, want %d (values=%v)", arr.Len(), len(want), arr.Values())
	}
	for i, w := range want {
		if got := ToNumber(arr.Get(i)); got != float64(w) {
			t.Fatalf("index %d = %v, want %d (values=%v)", i, got, w, arr.Values())
		}
	}
}
