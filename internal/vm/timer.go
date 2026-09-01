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
	m.callbacks[id] = fn

	at := m.loop.Now().Add(delay)
	var timerID host.TimerID
	if repeat {
		timerID = m.loop.PostEvery(at, delay, id)
	} else {
		timerID = m.loop.Post(at, id)
	}
	return float64(timerID), nil
}

// CancelTimer stops one timer.
func (m *VM) CancelTimer(id float64) bool {
	if math.IsNaN(id) || id < 0 {
		return false
	}
	return m.loop.Cancel(host.TimerID(id))
}

// CancelAllTimers stops every timer.
func (m *VM) CancelAllTimers() { m.loop.CancelAll() }

// Wait moves the clock forward, running the callbacks that come due on the
// way. It never sleeps: the clock is virtual (AGENTS.md §8).
func (m *VM) Wait(seconds float64) error {
	if math.IsNaN(seconds) || seconds < 0 {
		seconds = 0
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
func (m *VM) dispatch(id host.CallbackID) error {
	fn, ok := m.callbacks[id]
	if !ok {
		return nil // 停止済みのタイマー
	}
	_, err := m.CallFunc(fn, nil)
	return err
}

// Now reports the virtual time, so that a command reading the clock sees the
// same time the callbacks are ordered by.
func (m *VM) Now() time.Time { return m.loop.Now() }
