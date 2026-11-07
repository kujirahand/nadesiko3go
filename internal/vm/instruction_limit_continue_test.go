package vm

import (
	"bytes"
	"strings"
	"testing"
)

// instructionLimitContinueSource は有限のループで、命令数の上限を小さく
// 設定すれば上限に当たるが、継続を許可すれば最後まで終わる (#449)。
const instructionLimitContinueSource = "カウンタX=0\n10回\n  カウンタX=カウンタX+1\nここまで\nカウンタXを表示\n"

// confirmHost はダイアログ対応ホスト。ShowDialog の呼び出し回数と、
// 「継続する」への回答を記録する。
type confirmHost struct {
	Collector
	calls    int
	accepted bool
}

func (h *confirmHost) ShowDialog(kind, message string) (string, bool, bool, error) {
	h.calls++
	if kind != "confirm" {
		return "", false, true, nil
	}
	if !strings.Contains(message, instructionLimitMessage) {
		return "", false, true, nil
	}
	return "", h.accepted, true, nil
}

func runInstructionLimitSource(t *testing.T, h Host, opts Options) error {
	t.Helper()
	prog, err := CompileProgram(instructionLimitContinueSource, "limit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	m := New(prog, runtimeRegistry(), h, opts)
	return m.Run()
}

func TestDefaultInstructionLimitIs600Million(t *testing.T) {
	if got, want := DefaultOptions().MaxInstructions, uint64(600_000_000); got != want {
		t.Errorf("DefaultOptions().MaxInstructions = %d, want %d (従来の3倍)", got, want)
	}
}

func TestInstructionLimitContinueWithDialog(t *testing.T) {
	host := &confirmHost{accepted: true}
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	if err := runInstructionLimitSource(t, host, opts); err != nil {
		t.Fatalf("継続を許可したのにエラーになった: %v", err)
	}
	if host.calls == 0 {
		t.Fatal("上限到達時に確認ダイアログが呼ばれていない")
	}
	if got := host.Log(); !strings.Contains(got, "10") {
		t.Errorf("最後まで実行されていない: 表示ログ = %q", got)
	}
}

func TestInstructionLimitContinueWithCUIAnswer(t *testing.T) {
	var out bytes.Buffer
	host := NewCUIHost(&out, strings.NewReader(strings.Repeat("y\n", 100)), nil)
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	if err := runInstructionLimitSource(t, host, opts); err != nil {
		t.Fatalf("『y』と答えたのにエラーになった: %v", err)
	}
	if !strings.Contains(out.String(), "継続しますか？ (y/N): ") {
		t.Errorf("継続確認のプロンプトが出ていない: %q", out.String())
	}
	if !strings.Contains(out.String(), "10") {
		t.Errorf("最後まで実行されていない: %q", out.String())
	}
}

func TestInstructionLimitDialogRejectStops(t *testing.T) {
	host := &confirmHost{accepted: false}
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("継続を断ったのに上限エラーになっていない: %v", err)
	}
	if host.calls != 1 {
		t.Errorf("ShowDialog calls = %d, want 1", host.calls)
	}
}

func TestInstructionLimitCUIAnswerNoStops(t *testing.T) {
	var out bytes.Buffer
	host := NewCUIHost(&out, strings.NewReader("n\n"), nil)
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("『n』と答えたのに上限エラーになっていない: %v", err)
	}
}

func TestInstructionLimitLongtimeErrorSkipsQuestion(t *testing.T) {
	host := &confirmHost{accepted: true}
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	opts.LongtimeError = true
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("--longtime-error 相当なのに上限エラーになっていない: %v", err)
	}
	if host.calls != 0 {
		t.Errorf("LongtimeError なのに確認ダイアログが呼ばれた: calls = %d", host.calls)
	}
}
