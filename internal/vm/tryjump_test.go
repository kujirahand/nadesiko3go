package vm_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Issue #193: 監視領域の内側から抜ける/続けるで脱出してもハンドラが残り、
// 領域外のエラーで死んだはずのエラーならば節へ飛んでしまっていた。

// 監視領域を抜けるで出たあとのエラーは捕捉されない。
func TestTryExceptBreakClearsHandler(t *testing.T) {
	code := `3回
  エラー監視
    抜ける。
  エラーならば
    「except」と表示。
  ここまで。
ここまで。
「E」でエラー発生。
「ループ後」と表示。
`
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err == nil {
		t.Fatalf("expected an uncaught error, got log %q", r.Log)
	}
	if strings.Contains(r.Log, "except") {
		t.Fatalf("except block fired outside the monitored region: %q", r.Log)
	}
}

// 続けるで出た場合も同じくハンドラが外れる。
func TestTryExceptContinueClearsHandler(t *testing.T) {
	code := `3回
  エラー監視
    続ける。
  エラーならば
    「except」と表示。
  ここまで。
ここまで。
「E」でエラー発生。
`
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err == nil {
		t.Fatalf("expected an uncaught error, got log %q", r.Log)
	}
	if strings.Contains(r.Log, "except") {
		t.Fatalf("except block fired outside the monitored region: %q", r.Log)
	}
}

// 監視領域の中のエラーは従来どおり捕捉され、抜けるも正しくループを終わらせる。
func TestTryExceptBreakKeepsInnerCatch(t *testing.T) {
	code := `3回
  エラー監視
    もし回数=2ならば抜ける
    「{回数}」でエラー発生
  エラーならば
    「except{エラーメッセージ}」と表示
  ここまで
ここまで
`
	if got := run(t, code); got != "except1" {
		t.Fatalf("got %q, want %q", got, "except1")
	}
}

// 内側ループの抜けるは外側の監視領域を超えないので、ハンドラは残る。
func TestTryExceptInnerBreakKeepsOuterCatch(t *testing.T) {
	code := `3回
  エラー監視
    5回
      抜ける
    ここまで
    「E{回数}」でエラー発生
  エラーならば
    エラーメッセージを表示
  ここまで
ここまで
`
	if got := run(t, code); got != "E1\nE2\nE3" {
		t.Fatalf("got %q, want %q", got, "E1\nE2\nE3")
	}
}

// エラーならば節の中の抜けるは、捕捉時に外れた分を余計に外さない。
func TestTryExceptBreakInsideExceptBlock(t *testing.T) {
	code := `3回
  エラー監視
    「E」でエラー発生
  エラーならば
    抜ける
  ここまで
ここまで
「終了」と表示
`
	if got := run(t, code); got != "終了" {
		t.Fatalf("got %q, want %q", got, "終了")
	}
}

// 監視領域の中の戻るはそのまま値を返す。
func TestTryExceptReturnInsideRegion(t *testing.T) {
	code := `●Fとは
  エラー監視
    1を戻す
  エラーならば
    0を戻す
  ここまで
ここまで
(F())を表示
`
	if got := run(t, code); got != "1" {
		t.Fatalf("got %q, want %q", got, "1")
	}
}

// while(〜の間)ループでも同じく、脱出でハンドラが外れる。
func TestTryExceptBreakInWhile(t *testing.T) {
	code := `I=0
(I<3)の間
  エラー監視
    I=I+1
    抜ける
  エラーならば
    「except」と表示
  ここまで
ここまで
「E」でエラー発生
`
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err == nil {
		t.Fatalf("expected an uncaught error, got log %q", r.Log)
	}
	if strings.Contains(r.Log, "except") {
		t.Fatalf("except block fired outside the monitored region: %q", r.Log)
	}
}
