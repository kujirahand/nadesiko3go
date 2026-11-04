package stdlib_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #344: 『配列一括挿入』の高速化後も、先頭・中間・末尾・自己挿入と
// 負インデックスの結果が公式実装と一致することを固定する。
// 負インデックスと自己挿入の期待値は公式 TypeScript 実装
// (plugin_system_array) を Node で実行して得た実測値。
func TestArrayBulkInsert(t *testing.T) {
	cases := []struct {
		name string
		at   value.Value
		want []int
	}{
		{"先頭", value.Number(0), []int{100, 200, 10, 20, 30, 40}},
		{"中間", value.Number(2), []int{10, 20, 100, 200, 30, 40}},
		{"末尾", value.Number(4), []int{10, 20, 30, 40, 100, 200}},
		{"末尾超過", value.Number(99), []int{10, 20, 30, 40, 100, 200}},
		{"数値文字列", value.String("2"), []int{10, 20, 100, 200, 30, 40}},
		{"非数値文字列は先頭", value.String("abc"), []int{100, 200, 10, 20, 30, 40}},
		{"負-1", value.Number(-1), []int{200, 10, 20, 30, 100, 40}},
		{"負-2", value.Number(-2), []int{10, 20, 100, 30, 200, 40}},
		{"負-4", value.Number(-4), []int{100, 10, 200, 20, 30, 40}},
		{"負-5", value.Number(-5), []int{100, 200, 10, 20, 30, 40}},
		{"負-100", value.Number(-100), []int{200, 100, 10, 20, 30, 40}},
		{"正の無限大は末尾", value.Number(math.Inf(1)), []int{10, 20, 30, 40, 100, 200}},
		{"負の無限大は先頭へ逆順", value.Number(math.Inf(-1)), []int{200, 100, 10, 20, 30, 40}},
		{"NaNは先頭", value.Number(math.NaN()), []int{100, 200, 10, 20, 30, 40}},
		{"巨大な正の有限値は末尾", value.Number(1e300), []int{10, 20, 30, 40, 100, 200}},
		{"巨大な負の有限値は先頭へ逆順", value.Number(-1e300), []int{200, 100, 10, 20, 30, 40}},
		{"正の小数は切り捨て", value.Number(1.9), []int{10, 100, 200, 20, 30, 40}},
		{"負の小数は要素ごとに切り捨て", value.Number(-1.9), []int{200, 10, 20, 30, 100, 40}},
		{"負の小数-0.5は要素ごとに切り捨て", value.Number(-0.5), []int{200, 100, 10, 20, 30, 40}},
	}
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("配列一括挿入")
	if !ok {
		t.Fatal("『配列一括挿入』が見つからない")
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dst := value.NewArray(value.Number(10), value.Number(20), value.Number(30), value.Number(40))
			src := value.NewArray(value.Number(100), value.Number(200))
			got, err := e.Fn(newContext(), []value.Value{value.ArrayValue(dst), tt.at, value.ArrayValue(src)})
			if err != nil {
				t.Fatal(err)
			}
			if gotArr, ok := got.Array(); !ok || gotArr != dst {
				t.Fatal("戻り値は挿入先の配列そのものであるべき")
			}
			assertNumbers(t, dst, tt.want)
		})
	}
}

func TestArrayBulkInsertSelfInsert(t *testing.T) {
	cases := []struct {
		at   float64
		want []int
	}{
		{0, []int{1, 2, 3, 1, 2, 3}},
		{1, []int{1, 1, 2, 3, 2, 3}},
		{-1, []int{2, 3, 1, 2, 1, 3}},
		{-2, []int{3, 1, 1, 2, 2, 3}},
		{99, []int{1, 2, 3, 1, 2, 3}},
	}
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("配列一括挿入")
	for _, tt := range cases {
		arr := value.NewArray(value.Number(1), value.Number(2), value.Number(3))
		v := value.ArrayValue(arr)
		if _, err := e.Fn(newContext(), []value.Value{v, value.Number(tt.at), v}); err != nil {
			t.Fatal(err)
		}
		assertNumbers(t, arr, tt.want)
	}
}

func TestArrayBulkInsertNonArray(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("配列一括挿入")
	arr := value.ArrayValue(value.NewArray(value.Number(1)))
	if _, err := e.Fn(newContext(), []value.Value{value.Number(1), value.Number(0), arr}); err == nil {
		t.Fatal("挿入先が配列でなければエラーになるべき")
	}
	if _, err := e.Fn(newContext(), []value.Value{arr, value.Number(0), value.Number(1)}); err == nil {
		t.Fatal("挿入元が配列でなければエラーになるべき")
	}
}

func assertNumbers(t *testing.T, arr *value.Array, want []int) {
	t.Helper()
	if arr.Len() != len(want) {
		t.Fatalf("len = %d, want %d (values=%v)", arr.Len(), len(want), arr.Values())
	}
	for i, w := range want {
		if got := value.ToNumber(arr.Get(i)); got != float64(w) {
			t.Fatalf("index %d = %v, want %d (values=%v)", i, got, w, arr.Values())
		}
	}
}
