package stdlib_test

import (
	"fmt"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #344 の受け入れ条件用ベンチマーク: n件の配列の先頭へn件を一括挿入する。
// 修正前は要素ごとの移動で時間が超線形に増えた (n=40000 で約1.5〜2.3秒)。
// 実行例: go test ./internal/stdlib/ -bench BenchmarkArrayBulkInsertHead -benchmem
func BenchmarkArrayBulkInsertHead(b *testing.B) {
	for _, n := range []int{5000, 10000, 20000, 40000} {
		b.Run(fmt.Sprintf("n%d", n), func(b *testing.B) {
			r := stdlib.NewRegistry()
			e, _ := r.Lookup("配列一括挿入")
			srcItems := make([]value.Value, n)
			for i := range srcItems {
				srcItems[i] = value.Number(float64(i))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for iter := 0; iter < b.N; iter++ {
				dstItems := make([]value.Value, n)
				for i := range dstItems {
					dstItems[i] = value.Number(float64(i))
				}
				dst := value.NewArray(dstItems...)
				src := value.NewArray(srcItems...)
				if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(dst), value.Number(0), value.ArrayValue(src)}); err != nil {
					b.Fatal(err)
				}
				if dst.Len() != 2*n {
					b.Fatalf("len=%d", dst.Len())
				}
			}
		})
	}
}
