package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #201: 行が配列でない表を渡すと、以前は nil ポインタでパニックに
// なっていた。明示的なエラーになることを確認する。
func TestTableCommandsNonArrayRow(t *testing.T) {
	r := stdlib.NewRegistry()
	table := value.NewArray(
		value.ArrayValue(value.NewArray(value.Number(1))),
		value.String("x"),
	)
	for _, name := range []string{"表行列交換", "表右回転"} {
		e, _ := r.Lookup(name)
		if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(table)}); err == nil {
			t.Fatalf("『%s』は非配列行でエラーになるべき", name)
		}
	}
	column := value.NewArray(value.Number(9), value.Number(9))
	e, _ := r.Lookup("表列挿入")
	if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(table), value.Number(0), value.ArrayValue(column)}); err == nil {
		t.Fatal("『表列挿入』は非配列行でエラーになるべき")
	}
	e, _ = r.Lookup("表列削除")
	if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(table), value.Number(0)}); err == nil {
		t.Fatal("『表列削除』は非配列行でエラーになるべき")
	}
}

// 通常の二次元配列の転置結果が変わっていないことも確認する。
func TestTransposeTableNormal(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("表行列交換")
	table := value.NewArray(
		value.ArrayValue(value.NewArray(value.Number(1), value.Number(2))),
		value.ArrayValue(value.NewArray(value.Number(3), value.Number(4))),
	)
	got, err := e.Fn(newContext(), []value.Value{value.ArrayValue(table)})
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := r.Lookup("JSONエンコード")
	out, err := enc.Fn(newContext(), []value.Value{got})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := out.String()
	if s != `[[1,3],[2,4]]` {
		t.Fatalf("転置結果 = %s", s)
	}
}
