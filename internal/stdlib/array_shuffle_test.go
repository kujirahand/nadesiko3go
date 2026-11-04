package stdlib

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// #345: shuffleArray（Fisher-Yates）に決定的な intn 系列を与え、
// 系列ごとに期待どおりの異なる順序になることを確かめる。
// 期待順序は本家の定義（i を末尾から1まで、j = intn(i+1) と交換）から手計算した。
func TestShuffleArrayWithScriptedRand(t *testing.T) {
	tests := []struct {
		name string
		intn func(n int) int
		want []float64
	}{
		{
			name: "常に0を選ぶ",
			intn: func(n int) int { return 0 },
			// [1,2,3,4] →(i=3,j=0) [4,2,3,1] →(i=2,j=0) [3,2,4,1] →(i=1,j=0) [2,3,4,1]
			want: []float64{2, 3, 4, 1},
		},
		{
			name: "常に最大値を選ぶ（自分自身と交換）",
			intn: func(n int) int { return n - 1 },
			want: []float64{1, 2, 3, 4},
		},
		{
			name: "混合した系列",
			intn: func(n int) int {
				// n=4→1, n=3→0, n=2→1 の順に呼ばれる
				return map[int]int{4: 1, 3: 0, 2: 1}[n]
			},
			// [1,2,3,4] →(i=3,j=1) [1,4,3,2] →(i=2,j=0) [3,4,1,2] →(i=1,j=1) [3,4,1,2]
			want: []float64{3, 4, 1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := value.NewArray(value.Number(1), value.Number(2), value.Number(3), value.Number(4))
			var ns []int
			shuffleArray(arr, func(n int) int {
				ns = append(ns, n)
				return tt.intn(n)
			})
			// intn には i+1（4, 3, 2 の順）が渡ること
			if len(ns) != 3 || ns[0] != 4 || ns[1] != 3 || ns[2] != 2 {
				t.Fatalf("intn に渡った n = %v, want [4 3 2]", ns)
			}
			if arr.Len() != len(tt.want) {
				t.Fatalf("Len() = %d, want %d", arr.Len(), len(tt.want))
			}
			for i, want := range tt.want {
				if n, _ := arr.Get(i).Number(); n != want {
					t.Fatalf("要素[%d] = %v, want %v（全体: %v）", i, n, want, arr.Values())
				}
			}
		})
	}
}

// #345: 空配列と1要素では intn が呼ばれず、内容も変わらないことを確かめる。
func TestShuffleArrayDoesNotCallIntNForShortArrays(t *testing.T) {
	intn := func(n int) int {
		t.Fatalf("要素数1以下で intn(%d) が呼ばれた", n)
		return 0
	}

	empty := value.NewArray()
	shuffleArray(empty, intn)
	if empty.Len() != 0 {
		t.Fatalf("空配列の Len() = %d, want 0", empty.Len())
	}

	single := value.NewArray(value.Number(42))
	shuffleArray(single, intn)
	if n, _ := single.Get(0).Number(); n != 42 {
		t.Fatalf("1要素の要素 = %v, want 42", n)
	}
}
