package vm

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunClosesExternalEvents(t *testing.T) {
	for _, code := range []string{"「正常」を表示", "終了", "「失敗」でエラー発生"} {
		t.Run(code, func(t *testing.T) {
			prog, err := CompileProgram(code, "cleanup.nako3")
			if err != nil {
				t.Fatal(err)
			}
			m := New(prog, runtimeRegistry(), &Collector{}, DefaultOptions())
			closed := 0
			m.PostExternalEvent(func() bool { return false }, nil, func() { closed++ })
			_ = m.Run()
			if closed != 1 || m.hasExternal {
				t.Fatalf("登録解除の回数: %d, hasExternal=%v", closed, m.hasExternal)
			}
		})
	}
}

// 実際のSIGINTを別プロセスへ送り、実行中のVMとハンドラの競合を検証する。
// -raceで実行すると子プロセスも同じ検出器を使う。
func TestInterruptDuringExecution(t *testing.T) {
	if mode := os.Getenv("GONAKO_SIGNAL_TEST_CHILD"); mode != "" {
		code := `完了=0
回数=0
●中断処理
  完了=1
  「handler」を表示
  偽で戻る
ここまで
「中断処理」を強制終了時
「ready」を表示
(完了<1)の間
  回数=回数+1
ここまで
「done」を表示`
		if mode == "second" {
			// ハンドラが戻り登録解除が完了した後に、親へ2回目の送信を依頼する。
			code = strings.Replace(code, "「done」を表示", `「continued」を表示
(完了=1)の間
  回数=回数+1
ここまで`, 1)
		}
		prog, err := CompileProgram(code, "signal.nako3")
		if err != nil {
			t.Fatal(err)
		}
		opts := DefaultOptions()
		opts.MaxInstructions = 0
		host := NewCUIHost(os.Stdout, strings.NewReader(""), nil)
		if err := New(prog, runtimeRegistry(), host, opts).Run(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("WindowsではProcess.SignalによるSIGINT送信に対応していない")
	}
	for _, mode := range []string{"continue", "second"} {
		t.Run(mode, func(t *testing.T) { testInterruptProcess(t, mode) })
	}
}

func testInterruptProcess(t *testing.T, mode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInterruptDuringExecution$")
	cmd.Env = append(os.Environ(), "GONAKO_SIGNAL_TEST_CHILD="+mode)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var lines []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "ready" || (mode == "second" && line == "continued") {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
		}
	}
	err = cmd.Wait()
	if mode == "second" {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("2回目のSIGINTで終了しない: %v\n%v\n%s", err, lines, &stderr)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("SIGINT以外で終了: %v\n%v\n%s", err, lines, &stderr)
		}
		if got := strings.Join(lines, "\n"); got != "ready\nhandler\ncontinued" {
			t.Fatalf("実行順が不正: %q", got)
		}
		return
	}
	if err != nil {
		t.Fatalf("SIGINT実行: %v\n%v\n%s", err, lines, &stderr)
	}
	if got := strings.Join(lines, "\n"); !strings.HasPrefix(got, "ready\nhandler\ndone\nPASS") {
		t.Fatalf("実行順が不正: %q", got)
	}
}
