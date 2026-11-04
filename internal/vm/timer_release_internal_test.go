package vm

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// #338: 完了・停止済みタイマーのコールバックを VM が保持し続けないことを確かめる。

func newTimerReleaseVM(t *testing.T) *VM {
	t.Helper()
	prog, err := CompileProgram("「開始」と表示", "timer_release.nako3")
	if err != nil {
		t.Fatal(err)
	}
	return New(prog, runtimeRegistry(), &Collector{}, DefaultOptions())
}

// 個別にキャンセルしたタイマーのコールバックが解放される。
func TestTimerReleaseOnCancel(t *testing.T) {
	m := newTimerReleaseVM(t)
	fn := &value.Func{ID: 0}
	base := len(m.callbacks)
	ids := make([]float64, 0, 1000)
	for i := 0; i < 1000; i++ {
		id, err := m.SetTimer(fn, 100, false)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if !m.CancelTimer(id) {
			t.Fatalf("タイマー %v の停止に失敗", id)
		}
	}
	if len(m.callbacks) != base {
		t.Errorf("個別キャンセル後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("個別キャンセル後に対応表が残留: %d", len(m.timerCallbacks))
	}
	// 停止済みの再停止は false で、何も壊さない。
	if m.CancelTimer(ids[0]) {
		t.Error("停止済みタイマーの再停止が true を返した")
	}
}

// 単発タイマーは完了時に解放される。
func TestTimerReleaseOnComplete(t *testing.T) {
	m := newTimerReleaseVM(t)
	fn := &value.Func{ID: 0}
	base := len(m.callbacks)
	for i := 0; i < 1000; i++ {
		if _, err := m.SetTimer(fn, 0, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Wait(1); err != nil {
		t.Fatal(err)
	}
	if len(m.callbacks) != base {
		t.Errorf("単発完了後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("単発完了後に対応表が残留: %d", len(m.timerCallbacks))
	}
}

// 全タイマー停止で保持中のコールバックがすべて解放される。
func TestTimerReleaseOnCancelAll(t *testing.T) {
	m := newTimerReleaseVM(t)
	fn := &value.Func{ID: 0}
	base := len(m.callbacks)
	for i := 0; i < 500; i++ {
		if _, err := m.SetTimer(fn, 100, false); err != nil {
			t.Fatal(err)
		}
	}
	m.CancelAllTimers()
	if len(m.callbacks) != base {
		t.Errorf("全キャンセル後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("全キャンセル後に対応表が残留: %d", len(m.timerCallbacks))
	}
}

// PostFunc の単発コールバックも、完了時と全停止時に解放される。
func TestTimerReleasePostFunc(t *testing.T) {
	m := newTimerReleaseVM(t)
	fn := &value.Func{ID: 0}
	base := len(m.callbacks)
	if err := m.PostFunc(fn, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Wait(1); err != nil {
		t.Fatal(err)
	}
	if len(m.callbacks) != base {
		t.Errorf("PostFunc 完了後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}

	if err := m.PostFunc(fn, nil); err != nil {
		t.Fatal(err)
	}
	m.CancelAllTimers()
	if len(m.callbacks) != base {
		t.Errorf("PostFunc の全停止後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("PostFunc の全停止後に対応表が残留: %d", len(m.timerCallbacks))
	}
}

// 繰り返しコールバックが『対象のタイマー停止』で自分を止めたときも解放される。
func TestTimerReleaseOnRepeatSelfStop(t *testing.T) {
	prog, err := CompileProgram("N=0\nF=関数()\nN=N+1\nもしN>=3ならば\n対象のタイマー停止\nここまで\nここまで\nFを0.01秒毎\n0.2秒待つ\nNを表示", "timer_release_self.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := &Collector{}
	m := New(prog, runtimeRegistry(), out, DefaultOptions())
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if got := out.Log(); got != "3" {
		t.Errorf("繰り返し実行回数 = %q, want \"3\"", got)
	}
	if len(m.callbacks) != 0 {
		t.Errorf("自分を止めた繰り返しのコールバックが残留: %d", len(m.callbacks))
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("自分を止めた繰り返しの対応表が残留: %d", len(m.timerCallbacks))
	}
}

// 繰り返しコールバックの実行中に『全タイマー停止』を呼んでも、その実行は
// 最後まで動き、保持中のコールバックはすべて解放される。
func TestTimerReleaseOnCancelAllFromCallback(t *testing.T) {
	prog, err := CompileProgram("N=0\nF=関数()\nN=N+1\nもしN>=3ならば\n全タイマー停止\nここまで\nここまで\nFを0.01秒毎\n0.2秒待つ\nNを表示", "timer_release_all.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := &Collector{}
	m := New(prog, runtimeRegistry(), out, DefaultOptions())
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if got := out.Log(); got != "3" {
		t.Errorf("繰り返し実行回数 = %q, want \"3\"", got)
	}
	if len(m.callbacks) != 0 {
		t.Errorf("全タイマー停止後にコールバックが残留: %d", len(m.callbacks))
	}
	if m.loop.Pending() != 0 {
		t.Errorf("全タイマー停止後にタイマーが残留: %d", m.loop.Pending())
	}
}

// 繰り返しタイマーは停止されるまで保持され、停止で解放される。
func TestTimerReleaseOnRepeatStop(t *testing.T) {
	m := newTimerReleaseVM(t)
	fn := &value.Func{ID: 0}
	base := len(m.callbacks)
	id, err := m.SetTimer(fn, 0.01, true)
	if err != nil {
		t.Fatal(err)
	}
	// 複数回実行されても保持し続け、ループにも残る。
	if err := m.Wait(0.05); err != nil {
		t.Fatal(err)
	}
	if m.loop.Pending() != 1 {
		t.Errorf("繰り返し実行中はループに1件残るはず: pending=%d", m.loop.Pending())
	}
	if len(m.callbacks) != base+1 {
		t.Errorf("繰り返し実行中は1件保持するはず: callbacks=%d want %d", len(m.callbacks), base+1)
	}
	if !m.CancelTimer(id) {
		t.Fatal("繰り返しタイマーの停止に失敗")
	}
	if len(m.callbacks) != base {
		t.Errorf("繰り返し停止後にコールバックが残留: callbacks=%d want %d", len(m.callbacks), base)
	}
	if len(m.timerCallbacks) != 0 {
		t.Errorf("繰り返し停止後に対応表が残留: %d", len(m.timerCallbacks))
	}
}
