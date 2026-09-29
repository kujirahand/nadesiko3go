package value_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #192: 自身を要素に持つ配列を文字列化すると、以前は回復不能な
// スタックオーバーフローでプロセスごと終了していた。循環要素は "(循環)"
// と表記して処理が終わることを確認する。
func TestToStringCyclicArray(t *testing.T) {
	a := value.NewArray(value.Number(1), value.Undefined())
	a.Set(1, value.ArrayValue(a))
	if got := value.ToString(value.ArrayValue(a)); got != "1,(循環)" {
		t.Fatalf("ToString(循環配列) = %q, want %q", got, "1,(循環)")
	}
}

// 循環が辞書経由でも検出されること。辞書は "[object Object]" にしかなら
// ないので、配列→配列→辞書の経路で再帰が止まることを確かめる。
func TestToStringCyclicNested(t *testing.T) {
	inner := value.NewArray(value.Number(9))
	a := value.NewArray(value.Number(1), value.ArrayValue(inner))
	inner.Set(1, value.ArrayValue(a))
	if got := value.ToString(value.ArrayValue(a)); got != "1,9,(循環)" {
		t.Fatalf("ToString(入れ子の循環) = %q, want %q", got, "1,9,(循環)")
	}
}

// 同じ配列を循環ではなく共有しているだけなら、従来どおり中身を出す。
// seen は経路を降りた時点で抹消されるので、二回目も展開されなければならない。
func TestToStringSharedArrayNotCyclic(t *testing.T) {
	shared := value.NewArray(value.Number(7))
	a := value.NewArray(value.ArrayValue(shared), value.ArrayValue(shared))
	if got := value.ToString(value.ArrayValue(a)); got != "7,7" {
		t.Fatalf("ToString(共有配列) = %q, want %q", got, "7,7")
	}
}
