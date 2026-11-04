package stdlib_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// #345: 『配列シャッフル』命令レベルのテスト。本家の契約どおり、
// 元配列を書き換えて同じ配列を返し、要素の集合と個数を保存する。

func shuffleCommand(t *testing.T) stdlib.Impl {
	t.Helper()
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("配列シャッフル")
	if !ok || e.Fn == nil {
		t.Fatal("配列シャッフル が登録されていない")
	}
	return e.Fn
}

func numberArray(nums ...float64) *value.Array {
	items := make([]value.Value, len(nums))
	for i, n := range nums {
		items[i] = value.Number(n)
	}
	return value.NewArray(items...)
}

func arrayString(arr *value.Array) string {
	parts := make([]string, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		parts = append(parts, value.ToString(arr.Get(i)))
	}
	return strings.Join(parts, ",")
}

func countNumbers(arr *value.Array) map[float64]int {
	counts := map[float64]int{}
	for i := 0; i < arr.Len(); i++ {
		if n, ok := arr.Get(i).Number(); ok {
			counts[n]++
		}
	}
	return counts
}

func TestArrayShuffleCommandMutatesAndPreservesElements(t *testing.T) {
	fn := shuffleCommand(t)
	// 重複要素を含む配列でも、要素の集合と個数が保存されること
	input := numberArray(1, 1, 2, 3, 3, 3)
	wantCounts := countNumbers(input)

	got, err := fn(newContext(), []value.Value{value.ArrayValue(input)})
	if err != nil {
		t.Fatalf("配列シャッフル() error = %v", err)
	}
	gotArr, ok := got.Array()
	if !ok {
		t.Fatalf("戻り値が配列でない: %v", got.Kind())
	}
	// 戻り値は入力と同じ配列オブジェクトで、入力側も書き換わっていること
	if gotArr != input {
		t.Fatalf("戻り値の配列 = %p, want 入力配列 %p", gotArr, input)
	}
	if input.Len() != 6 {
		t.Fatalf("Len() = %d, want 6", input.Len())
	}
	gotCounts := countNumbers(input)
	if len(gotCounts) != len(wantCounts) {
		t.Fatalf("要素の種類数 = %d, want %d（%v）", len(gotCounts), len(wantCounts), arrayString(input))
	}
	for n, want := range wantCounts {
		if gotCounts[n] != want {
			t.Fatalf("要素 %v の個数 = %d, want %d（%v）", n, gotCounts[n], want, arrayString(input))
		}
	}
}

func TestArrayShuffleCommandAcceptsEmptyAndSingleElementArrays(t *testing.T) {
	fn := shuffleCommand(t)
	for _, input := range []*value.Array{numberArray(), numberArray(42)} {
		got, err := fn(newContext(), []value.Value{value.ArrayValue(input)})
		if err != nil {
			t.Fatalf("配列シャッフル(要素数%d) error = %v", input.Len(), err)
		}
		gotArr, ok := got.Array()
		if !ok || gotArr != input {
			t.Fatalf("戻り値の配列 = %p, want 入力配列 %p", gotArr, input)
		}
	}
}

func TestArrayShuffleCommandRejectsNonArray(t *testing.T) {
	fn := shuffleCommand(t)
	if _, err := fn(newContext(), []value.Value{value.String("abc")}); err == nil {
		t.Fatal("配列以外でエラーにならなかった")
	}
}

// #345 の再現ケース: 本番乱数で繰り返し実行しても、常に逆順にはならないこと。
func TestArrayShuffleCommandIsNotAlwaysReverse(t *testing.T) {
	fn := shuffleCommand(t)
	seen := map[string]bool{}
	for i := 0; i < 16; i++ {
		input := numberArray(1, 2, 3, 4)
		if _, err := fn(newContext(), []value.Value{value.ArrayValue(input)}); err != nil {
			t.Fatalf("配列シャッフル() error = %v", err)
		}
		seen[arrayString(input)] = true
	}
	if len(seen) == 1 && seen["4,3,2,1"] {
		t.Fatal("16回すべて逆順になった")
	}
	if len(seen) < 2 {
		t.Fatalf("16回すべて同じ順序になった: %v", seen)
	}
}
