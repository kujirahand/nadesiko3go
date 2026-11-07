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

// terminalHost は標準入力が端末に繋がっている体裁の CUIHost を作る。
// 文字列リーダーは端末ではないため、そのままでは継続確認の対象にならない。
func terminalHost(out *bytes.Buffer, input string) *CUIHost {
	h := NewCUIHost(out, strings.NewReader(input), nil)
	h.stdinTerminal = true
	return h
}

func TestInstructionLimitContinueWithCUIAnswer(t *testing.T) {
	var out bytes.Buffer
	host := terminalHost(&out, strings.Repeat("y\n", 100))
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
	host := terminalHost(&out, "n\n")
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("『n』と答えたのに上限エラーになっていない: %v", err)
	}
}

// T1 (#449 レビュー H1): 呼び出し先の関数で上限に達して継続し、呼び出し元へ
// 戻ったあと、延長済みの枠を使い切る前に再び質問されないこと。
func TestInstructionLimitContinueAcrossFrames(t *testing.T) {
	src := "●Fとは\n  C=0\n  20回\n    C=C+1\n  ここまで\nここまで\nFを実行\nX=0\n20回\n  X=X+1\nここまで\nXを表示\n"
	prog, err := CompileProgram(src, "limit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := &confirmHost{accepted: true}
	opts := DefaultOptions()
	opts.MaxInstructions = 50
	m := New(prog, runtimeRegistry(), host, opts)
	if err := m.Run(); err != nil {
		t.Fatalf("継続を許可したのにエラーになった: %v", err)
	}
	// 実測: 同期ありで7回、呼び出し元の古い上限で即再質問される不具合
	// （同期なし）では10回になる。余裕を見て上限は8回とする。
	if host.calls == 0 || host.calls > 8 {
		t.Errorf("ShowDialog calls = %d, want 1..8", host.calls)
	}
	if got := host.Log(); !strings.Contains(got, "20") {
		t.Errorf("最後まで実行されていない: 表示ログ = %q", got)
	}
}

// T2 (#449 レビュー M1): 継続して再開しても、実行した命令の課金が
// 上限なしで実行した場合と一致すること（二重課金・課金漏れがないこと）。
func TestInstructionLimitContinueKeepsExecutedCount(t *testing.T) {
	run := func(opts Options, h Host) (uint64, error) {
		prog, err := CompileProgram(instructionLimitContinueSource, "limit.nako3")
		if err != nil {
			t.Fatal(err)
		}
		m := New(prog, runtimeRegistry(), h, opts)
		err = m.Run()
		return m.executed, err
	}
	unlimited := DefaultOptions()
	unlimited.MaxInstructions = 0
	want, err := run(unlimited, &confirmHost{accepted: true})
	if err != nil {
		t.Fatalf("上限なしの実行がエラーになった: %v", err)
	}
	limited := DefaultOptions()
	limited.MaxInstructions = 10
	got, err := run(limited, &confirmHost{accepted: true})
	if err != nil {
		t.Fatalf("継続した実行がエラーになった: %v", err)
	}
	if got != want {
		t.Errorf("executed = %d, want %d (上限なしの実行と一致)", got, want)
	}
}

// T3 (#449 レビュー): エラー監視の中で上限に達しても、継続すれば
// 監視に捕捉されず最後まで実行できること。
func TestInstructionLimitContinueInsideErrorMonitor(t *testing.T) {
	src := "エラー監視\n  カウンタX=0\n  10回\n    カウンタX=カウンタX+1\n  ここまで\nエラーならば\n  「捕捉した」を表示\nここまで\n「終わり」を表示\n"
	prog, err := CompileProgram(src, "limit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := &confirmHost{accepted: true}
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	m := New(prog, runtimeRegistry(), host, opts)
	if err := m.Run(); err != nil {
		t.Fatalf("継続を許可したのにエラーになった: %v", err)
	}
	if got := host.Log(); !strings.Contains(got, "終わり") || strings.Contains(got, "捕捉した") {
		t.Errorf("エラー監視の結果が想定と違う: 表示ログ = %q", got)
	}
}

// T4 (#449 レビュー): 上限が無効 (0) のときは質問が出ないこと。
func TestInstructionLimitZeroLimitNeverAsks(t *testing.T) {
	host := &confirmHost{accepted: false}
	opts := DefaultOptions()
	opts.MaxInstructions = 0
	if err := runInstructionLimitSource(t, host, opts); err != nil {
		t.Fatalf("上限なしなのにエラーになった: %v", err)
	}
	if host.calls != 0 {
		t.Errorf("上限なしなのに質問された: calls = %d", host.calls)
	}
}

// T7 (#449 レビュー): 端末では各種の肯定回答で継続し、EOF では停止すること。
// 端末でない標準入力（パイプ等）では質問せず、入力も消費しないこと。
func TestInstructionLimitCUIAnswerVariants(t *testing.T) {
	for _, answer := range []string{"はい", "YES", " y "} {
		var out bytes.Buffer
		host := terminalHost(&out, strings.Repeat(answer+"\n", 100))
		opts := DefaultOptions()
		opts.MaxInstructions = 10
		if err := runInstructionLimitSource(t, host, opts); err != nil {
			t.Errorf("回答 %q で継続しなかった: %v", answer, err)
		}
	}
}

func TestInstructionLimitCUIEOFStops(t *testing.T) {
	var out bytes.Buffer
	host := terminalHost(&out, "")
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("EOF なのに上限エラーになっていない: %v", err)
	}
}

func TestInstructionLimitPipedStdinNeverAsks(t *testing.T) {
	var out bytes.Buffer
	// 端末ではない標準入力（パイプを想定）。中身はプログラムへのデータ。
	host := NewCUIHost(&out, strings.NewReader("y\nデータ行\n"), nil)
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	err := runInstructionLimitSource(t, host, opts)
	if err == nil || !strings.Contains(err.Error(), instructionLimitMessage) {
		t.Fatalf("パイプ入力なのに上限エラーになっていない: %v", err)
	}
	if strings.Contains(out.String(), "継続しますか") {
		t.Errorf("パイプ入力なのにプロンプトが出た: %q", out.String())
	}
	// 入力が消費されていないこと（データがプログラム側に残ること）。
	line, rerr := host.ReadLine()
	if rerr != nil || line != "y" {
		t.Errorf("入力が消費されている: line = %q, err = %v", line, rerr)
	}
}

// T5 (#453): 上限延長の計算（飽和付き加算）の単体テスト。
func TestExtendInstructionLimit(t *testing.T) {
	max := ^uint64(0)
	tests := []struct {
		name          string
		current, step uint64
		want          uint64
	}{
		{name: "通常加算", current: 100, step: 20, want: 120},
		{name: "ちょうど最大値", current: max - 10, step: 10, want: max},
		{name: "溢れは飽和", current: max - 10, step: 11, want: max},
		{name: "延長幅ゼロ", current: 100, step: 0, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extendInstructionLimit(tt.current, tt.step); got != tt.want {
				t.Fatalf("extendInstructionLimit(%d, %d) = %d, want %d", tt.current, tt.step, got, tt.want)
			}
		})
	}
}

// M2残件 (#453): 継続確認のプロンプトは標準出力ではなく Err へ出ること。
// Err 未設定のときは従来どおり Out へ出ること。
func TestInstructionLimitPromptGoesToErr(t *testing.T) {
	var out, errOut bytes.Buffer
	host := terminalHost(&out, "n\n")
	host.Err = &errOut
	opts := DefaultOptions()
	opts.MaxInstructions = 10
	if err := runInstructionLimitSource(t, host, opts); err == nil {
		t.Fatal("上限エラーになるはずがならなかった")
	}
	if !strings.Contains(errOut.String(), "継続しますか？ (y/N): ") {
		t.Errorf("プロンプトが Err に出ていない: Err = %q", errOut.String())
	}
	if strings.Contains(out.String(), "継続しますか") {
		t.Errorf("プロンプトが標準出力に混ざっている: Out = %q", out.String())
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
