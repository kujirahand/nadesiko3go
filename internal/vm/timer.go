package vm

import (
	"errors"
	"math"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/host"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// SetTimer schedules a function value and reports the timer id as a number,
// which is the form 『対象』 and 『タイマー停止』 use.
func (m *VM) SetTimer(fn *value.Func, seconds float64, repeat bool) (float64, error) {
	if fn == nil {
		return 0, errors.New("タイマーに指定できるのは関数だけです。")
	}
	if math.IsNaN(seconds) || seconds < 0 {
		seconds = 0
	}
	delay := time.Duration(seconds * float64(time.Second))

	m.nextCallback++
	id := m.nextCallback

	at := m.loop.Now().Add(delay)
	var timerID host.TimerID
	if repeat {
		timerID = m.loop.PostEvery(at, delay, id)
	} else {
		timerID = m.loop.Post(at, id)
	}
	// タイマーIDが確定してから登録する。解放は完了・停止の各経路で行う。
	m.callbacks[id] = queuedCallback{fn: fn, isTimer: true, timerID: timerID, repeat: repeat}
	m.timerCallbacks[timerID] = id
	return float64(timerID), nil
}

// PostFunc schedules a one-shot callback after the current statement stream.
// Posting at the current virtual time preserves FIFO order without sleeping.
func (m *VM) PostFunc(fn *value.Func, args []value.Value) error {
	if fn == nil {
		return errors.New("コールバックに指定できるのは関数だけです。")
	}
	m.nextCallback++
	id := m.nextCallback
	timerID := m.loop.Post(m.loop.Now(), id)
	m.callbacks[id] = queuedCallback{fn: fn, args: append([]value.Value(nil), args...), timerID: timerID}
	m.timerCallbacks[timerID] = id
	return nil
}

// CancelTimer stops one timer.
func (m *VM) CancelTimer(id float64) bool {
	if math.IsNaN(id) || id < 0 {
		return false
	}
	timerID := host.TimerID(id)
	if !m.loop.Cancel(timerID) {
		return false // 完了済み・停止済み（保持していたものは解放済み）
	}
	m.releaseTimer(timerID)
	return true
}

// CancelAllTimers stops every timer and releases the callbacks they held.
// 繰り返しコールバックの実行中に呼ばれても、実行中の CallFunc は
// 自分の持つ値で最後まで動くので、map を空にして問題ない。
func (m *VM) CancelAllTimers() {
	m.loop.CancelAll()
	clear(m.callbacks)
	clear(m.timerCallbacks)
}

// releaseTimer はタイマーIDに対応するコールバックを解放する。
func (m *VM) releaseTimer(timerID host.TimerID) {
	if id, ok := m.timerCallbacks[timerID]; ok {
		delete(m.callbacks, id)
		delete(m.timerCallbacks, timerID)
	}
}

// Wait moves the clock forward, running the callbacks that come due on the
// way. When RealSleep is enabled, it pauses in real time.
func (m *VM) Wait(seconds float64) error {
	if math.IsNaN(seconds) || seconds < 0 {
		seconds = 0
	}
	if m.options.RealSleep && seconds > 0 {
		time.Sleep(time.Duration(seconds * float64(time.Second)))
	}
	until := m.loop.Now().Add(time.Duration(seconds * float64(time.Second)))
	return m.loop.RunUntil(until, m.dispatch)
}

// runPendingCallbacks runs the one-shot callbacks left over when main ends.
func (m *VM) runPendingCallbacks() error {
	return m.loop.RunUntilIdle(m.dispatch)
}

// dispatch runs one scheduled callback. An error inside a callback stops the
// loop and surfaces as the program's error.
// コールバック内の『終了』は error ではなく exitPanic で抜けるので、エラー監視や
// Run まで素通りする (#337)。
func (m *VM) dispatch(id host.CallbackID) error {
	callback, ok := m.callbacks[id]
	if !ok {
		return nil // 停止済みのタイマー
	}
	// 単発は実行前に解放する（エラー時も保持し続けない）。繰り返しは
	// 停止されるまで保持する。
	if !callback.repeat {
		delete(m.callbacks, id)
		delete(m.timerCallbacks, callback.timerID)
	}
	if callback.isTimer {
		m.pendingTimerTarget = float64(callback.timerID)
		defer func() { m.pendingTimerTarget = 0 }()
	}
	_, err := m.CallFunc(callback.fn, callback.args)
	return err
}

// Now reports the virtual time, so that a command reading the clock sees the
// same time the callbacks are ordered by.
func (m *VM) Now() time.Time { return m.loop.Now() }

// PostExternalEventはVMと同じスレッドで処理する外部イベントを登録する。
func (m *VM) PostExternalEvent(ready func() bool, run func() error, close func()) {
	m.loop.PostExternal(ready, run, close)
	m.hasExternal = true
}

// PollExternalEventsはVMとGoコード生成版の安全な実行境界で受信を確認する。
func (m *VM) PollExternalEvents() {
	if err := m.loop.PollExternal(); err != nil {
		m.failAt(err.Error(), m.currentPos)
	}
}
