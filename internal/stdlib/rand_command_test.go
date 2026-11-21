package stdlib_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #525: 『乱数』命令のテスト。本家の契約は、数値Aなら 0..A-1 の整数、
// 配列 [最小,最大] ならその範囲の整数を返すこと。値そのものはランダムなので
// 範囲と整数であることを多数回で確かめる。
func TestRandomCommand(t *testing.T) {
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("乱数")
	if !ok || e.Fn == nil {
		t.Fatal("乱数 が登録されていない")
	}
	fn := e.Fn

	call := func(arg value.Value) float64 {
		t.Helper()
		got, err := fn(nil, []value.Value{arg})
		if err != nil {
			t.Fatalf("乱数(%v): %v", arg, err)
		}
		n, ok := got.Number()
		if !ok {
			t.Fatalf("乱数(%v) = %v, 数値ではありません", arg, got)
		}
		return n
	}

	// 数値: 0..A-1 の整数。端の値も出ること (確率的に十分な回数を回す)。
	seen := map[float64]bool{}
	for i := 0; i < 2000; i++ {
		n := call(value.Number(10))
		if n < 0 || n > 9 || n != float64(int64(n)) {
			t.Fatalf("10の乱数 = %v, 0..9 の整数ではありません", n)
		}
		seen[n] = true
	}
	if !seen[0] || !seen[9] {
		t.Fatalf("10の乱数で端の値が出ませんでした: %v", seen)
	}

	// 1の乱数は常に0。
	for i := 0; i < 50; i++ {
		if n := call(value.Number(1)); n != 0 {
			t.Fatalf("1の乱数 = %v, want 0", n)
		}
	}

	// 配列 [最小,最大]: その範囲の整数。
	rangeArg := value.ArrayValue(value.NewArray(value.Number(3), value.Number(5)))
	seen = map[float64]bool{}
	for i := 0; i < 2000; i++ {
		n := call(rangeArg)
		if n < 3 || n > 5 || n != float64(int64(n)) {
			t.Fatalf("[3,5]の乱数 = %v, 3..5 の整数ではありません", n)
		}
		seen[n] = true
	}
	if len(seen) != 3 {
		t.Fatalf("[3,5]の乱数で全ての値が出ませんでした: %v", seen)
	}

	// 最大 < 最小は最小を返す（乱数を振らない）。
	if n := call(value.ArrayValue(value.NewArray(value.Number(5), value.Number(3)))); n != 5 {
		t.Fatalf("[5,3]の乱数 = %v, want 5", n)
	}
}

// 幅が int64 に収まらない範囲でも panic せず、最小値を返す。
func TestRandomCommandHugeRangeDoesNotPanic(t *testing.T) {
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("乱数")
	if !ok || e.Fn == nil {
		t.Fatal("乱数 が登録されていない")
	}
	wide := value.ArrayValue(value.NewArray(
		value.Number(float64(math.MinInt64)),
		value.Number(math.Pow(2, 62)),
	))
	got, err := e.Fn(nil, []value.Value{wide})
	if err != nil {
		t.Fatal(err)
	}
	n, ok := got.Number()
	if !ok {
		t.Fatalf("乱数 = %v, 数値ではありません", got)
	}
	if n != float64(math.MinInt64) {
		t.Fatalf("巨大範囲の乱数 = %v, want %v", n, float64(math.MinInt64))
	}
}
