package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #346: 『等』と『一致』を分ける。
// 『等』は本家の a === b (参照の同一性)、『一致』は両辺がオブジェクトの
// ときだけ内容 (JSON 表現) を比べる。以前は両方が内容比較になっていて、
// 同内容の別配列まで『等』が真になっていた。

func callCompare(t *testing.T, name string, left, right value.Value) bool {
	t.Helper()
	r := stdlib.NewRegistry()
	e, ok := r.Lookup(name)
	if !ok || e.Fn == nil {
		t.Fatalf("命令『%s』が登録されていない", name)
	}
	got, err := e.Fn(newContext(), []value.Value{left, right})
	if err != nil {
		t.Fatalf("『%s』でエラー: %v", name, err)
	}
	b, _ := got.Bool()
	return b
}

func TestIdentity346EqualIsReferenceOnly(t *testing.T) {
	arrA := value.ArrayValue(value.NewArray(value.Number(1)))
	arrB := value.ArrayValue(value.NewArray(value.Number(1)))
	makeDict := func() value.Value {
		d := value.NewDict()
		d.Set("x", value.Number(1))
		return value.DictValue(d)
	}
	dictA, dictB := makeDict(), makeDict()

	tests := []struct {
		name  string
		cmd   string
		left  value.Value
		right value.Value
		want  bool
	}{
		{"別配列の等", "等", arrA, arrB, false},
		{"別配列の等無", "等無", arrA, arrB, true},
		{"別配列の一致", "一致", arrA, arrB, true},
		{"別配列の不一致", "不一致", arrA, arrB, false},
		{"同じ配列参照の等", "等", arrA, arrA, true},
		{"同じ配列参照の一致", "一致", arrA, arrA, true},
		{"別辞書の等", "等", dictA, dictB, false},
		{"別辞書の一致", "一致", dictA, dictB, true},
		{"配列と辞書の一致", "一致", arrA, dictA, false},
		// プリミティブは従来どおり厳密比較 (型変換しない)
		{"1と1の等", "等", value.Number(1), value.Number(1), true},
		{"1と「1」の等", "等", value.Number(1), value.String("1"), false},
		{"1と「1」の一致", "一致", value.Number(1), value.String("1"), false},
		{"1と「1」の等無", "等無", value.Number(1), value.String("1"), true},
		{"文字列どうしの一致", "一致", value.String("abc"), value.String("abc"), true},
		{"オンと1の等", "等", value.Bool(true), value.Number(1), false},
		// null (『NULL』定数) は本家では typeof が "object" なので
		// 『一致』の内容比較の対象になる。なお『空』定数は空文字列で別物。
		{"NULLとNULLの等", "等", value.Null(), value.Null(), true},
		{"NULLとNULLの一致", "一致", value.Null(), value.Null(), true},
		{"NULLと辞書の一致", "一致", value.Null(), dictA, false},
		{"NULLと未定義の一致", "一致", value.Null(), value.Undefined(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := callCompare(t, tt.cmd, tt.left, tt.right); got != tt.want {
				t.Errorf("『%s』の結果 = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// 関数は本家では typeof が "function" なので、『一致』でも内容比較せず
// 参照だけで決まる。
func TestIdentity346FuncIsReferenceOnly(t *testing.T) {
	f1 := value.FuncValue(&value.Func{ID: 1})
	f2 := value.FuncValue(&value.Func{ID: 1})
	if got := callCompare(t, "等", f1, f2); got {
		t.Errorf("別関数の『等』 = %v, want false", got)
	}
	if got := callCompare(t, "一致", f1, f2); got {
		t.Errorf("別関数の『一致』 = %v, want false", got)
	}
	if got := callCompare(t, "一致", f1, f1); !got {
		t.Errorf("同じ関数の『一致』 = %v, want true", got)
	}
}

// 循環参照の『一致』は、本家の JSON.stringify が例外にするのと同じく
// エラーになる。『不一致』だけが真になって例外を隠したりはしない。
func TestIdentity346MatchedCyclicIsError(t *testing.T) {
	cyclic := func() value.Value {
		d := value.NewDict()
		d.Set("self", value.DictValue(d))
		return value.DictValue(d)
	}
	r := stdlib.NewRegistry()
	for _, name := range []string{"一致", "不一致"} {
		e, _ := r.Lookup(name)
		if _, err := e.Fn(newContext(), []value.Value{cyclic(), cyclic()}); err == nil {
			t.Errorf("循環辞書の『%s』はエラーになるべき", name)
		}
	}
	// 『等』は参照だけを見るので、循環していてもエラーにならない
	if got := callCompare(t, "等", cyclic(), cyclic()); got {
		t.Errorf("別辞書の『等』 = %v, want false", got)
	}
	same := cyclic()
	if got := callCompare(t, "等", same, same); !got {
		t.Errorf("同じ辞書の『等』 = %v, want true", got)
	}
}
