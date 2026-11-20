package vm

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// #528: 本体終了後に (イベントハンドラ相当の CallFunc で) 登録したタイマーが、
// RunDueTimers で実時間どおりに発火することを確かめる。
func TestRunDueTimersFiresTimersRegisteredAfterMain(t *testing.T) {
	code := `回数=0
数える=関数()
　回数=回数+1
ここまで
●登録とは
　数えるを0.03秒後
　数えるを0.01秒毎
ここまで
●回数表示とは
　回数を表示
ここまで`
	prog, err := CompileProgram(code, "timer_pump.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := &Collector{}
	m := New(prog, runtimeRegistry(), host, DefaultOptions())
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		t.Helper()
		host.buf.Reset()
		fn := m.FindFunc("回数表示")
		if fn == nil {
			t.Fatal("回数表示 が見つかりません")
		}
		if _, err := m.CallFunc(fn, nil); err != nil {
			t.Fatal(err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(host.buf.String()))
		if err != nil {
			t.Fatalf("回数の表示が数値ではありません: %q", host.buf.String())
		}
		return n
	}
	register := m.FindFunc("登録")
	if register == nil {
		t.Fatal("登録 が見つかりません")
	}
	if _, err := m.CallFunc(register, nil); err != nil {
		t.Fatal(err)
	}
	// 登録直後はまだ期限が来ていないので発火しない。
	if err := m.RunDueTimers(); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 0 {
		t.Fatalf("登録直後の回数 = %d, want 0", got)
	}
	// 実時間で 0.03 秒 + 0.01 秒毎が発火する分だけ待つ。
	time.Sleep(50 * time.Millisecond)
	if err := m.RunDueTimers(); err != nil {
		t.Fatal(err)
	}
	// 単発1回 + 繰り返し (50ms / 10ms で最大5回程度)。境界の誤差を見て 2 回以上。
	if got := count(); got < 2 {
		t.Fatalf("ポンプ後の回数 = %d, want >= 2", got)
	}
	// 全タイマー停止後は増えない。
	m.CancelAllTimers()
	before := count()
	time.Sleep(30 * time.Millisecond)
	if err := m.RunDueTimers(); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != before {
		t.Fatalf("停止後の回数 = %d, want %d", got, before)
	}
}
