package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #207 の姉妹修正: nodelib の parseJSONBytes と同じ欠陥が
// 『JSONデコード』『JSON取得』にもあった。JSON.parse と同じく、先頭の値の
// 後に空白以外のゴミが残る入力は失敗する。
func TestDecodeJSONTrailingGarbage(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONデコード")
	for _, s := range []string{`[1,2] xxx`, `1 2`, `{"a":1} extra`} {
		if _, err := e.Fn(newContext(), []value.Value{value.String(s)}); err == nil {
			t.Fatalf("末尾にゴミがある %q はエラーになるべき", s)
		}
	}
	// 末尾の空白だけなら従来どおり読み取る
	v, err := e.Fn(newContext(), []value.Value{value.String("  [1,2]  ")})
	if err != nil {
		t.Fatalf("末尾が空白だけの入力は通るはず: %v", err)
	}
	if got := value.ToString(v); got != "1,2" {
		t.Fatalf("got=%q", got)
	}
}
