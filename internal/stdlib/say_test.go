package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// sayContext は Print と Write を別々に記録するテスト用コンテキスト。
type sayContext struct {
	*fakeContext
	printed []string
	written []string
}

func (c *sayContext) Print(s string) { c.printed = append(c.printed, s) }
func (c *sayContext) Write(s string) { c.written = append(c.written, s) }

// TestSayWritesStdoutOnly は issue #224 の回帰テスト。ダイアログ非対応の
// 『言』フォールバックは stdout への書き出し(Write)だけで、表示ログを
// 更新する Print 経路を通らないこと（本家の sys.logger.send('stdout')）。
func TestSayWritesStdoutOnly(t *testing.T) {
	r := stdlib.NewRegistry()
	say, ok := r.Lookup("言")
	if !ok || say.Fn == nil {
		t.Fatal("言 が実装されていない")
	}

	ctx := &sayContext{fakeContext: newContext()}
	if _, err := say.Fn(ctx, []value.Value{value.String("X")}); err != nil {
		t.Fatalf("言: %v", err)
	}
	if len(ctx.printed) != 0 {
		t.Errorf("言 が Print を呼んだ（表示ログに混入する）: %v", ctx.printed)
	}
	if len(ctx.written) == 0 {
		t.Error("言 が stdout への書き出しをしなかった")
	}
}
