package stdlib_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #192: 循環した値を JSONエンコード すると、以前は回復不能な
// スタックオーバーフローでプロセスごと終了していた。JavaScript の
// JSON.stringify と同じく、循環は捕捉できる明示的なエラーになる。
func TestEncodeJSONCyclicArray(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONエンコード")
	a := value.NewArray(value.Undefined())
	a.Set(0, value.ArrayValue(a))
	if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(a)}); err == nil {
		t.Fatal("循環する配列はエラーになるべき")
	}
}

func TestEncodeJSONCyclicDict(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONエンコード")
	d := value.NewDict()
	d.Set("self", value.DictValue(d))
	if _, err := e.Fn(newContext(), []value.Value{value.DictValue(d)}); err == nil {
		t.Fatal("循環する辞書はエラーになるべき")
	}
}

// 配列と辞書が互いを指す場合も同様。
func TestEncodeJSONCyclicAcrossKinds(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONエンコード")
	a := value.NewArray()
	d := value.NewDict()
	d.Set("a", value.ArrayValue(a))
	a.Set(0, value.DictValue(d))
	if _, err := e.Fn(newContext(), []value.Value{value.ArrayValue(a)}); err == nil {
		t.Fatal("相互参照する値はエラーになるべき")
	}
}

// 同じ辞書を循環でなく複数箇所から共有しているだけなら許容する。
// JSON.stringify と同じく、検出は再帰経路上のものだけに限る。
func TestEncodeJSONSharedNonCyclic(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONエンコード")
	shared := value.NewDict()
	shared.Set("v", value.Number(1))
	d := value.NewDict()
	d.Set("a", value.DictValue(shared))
	d.Set("b", value.DictValue(shared))
	got, err := e.Fn(newContext(), []value.Value{value.DictValue(d)})
	if err != nil {
		t.Fatalf("共有だけの非循環な値はエラーにならないはず: %v", err)
	}
	s, _ := got.String()
	if s != `{"a":{"v":1},"b":{"v":1}}` {
		t.Fatalf("got=%s", s)
	}
}

// 循環配列を『表示』してもプロセスが終了せず、循環要素が "(循環)" と
// 表記されることを確認する（value.ToString 経由）。
func TestPrintCyclicArray(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()
	e, _ := r.Lookup("表示")
	a := value.NewArray(value.Number(1), value.Undefined())
	a.Set(1, value.ArrayValue(a))
	if _, err := e.Fn(ctx, []value.Value{value.ArrayValue(a)}); err != nil {
		t.Fatal(err)
	}
	if len(ctx.out) != 1 || !strings.Contains(ctx.out[0], "(循環)") {
		t.Fatalf("循環配列の表示 = %v", ctx.out)
	}
}
