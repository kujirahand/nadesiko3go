// Package event holds the timer queue that makes asynchronous execution
// deterministic (AGENTS.md §8).
//
// There are no goroutines here. Callbacks run one at a time, in order of their
// scheduled time and, within the same time, in the order they were posted. The
// clock is virtual: Run jumps straight to the next callback's time instead of
// waiting, so 『3秒後』 costs nothing to execute and the result does not depend
// on how fast the machine is.
package event

import (
	"errors"
	"sort"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/host"
)

// ErrTooManyCallbacks stops a repeating timer that would otherwise never end.
var ErrTooManyCallbacks = errors.New("タイマーのコールバックが多すぎます")

// timer is one scheduled callback.
type timer struct {
	id       host.TimerID
	at       time.Time
	interval time.Duration // 0 なら単発
	callback host.CallbackID
	seq      uint64 // 同じ時刻に積まれたものの順序
}

// Loop is the timer queue. It is not safe for concurrent use, which is the
// point: a single thread of control keeps the order observable.
type Loop struct {
	now    time.Time
	queue  []*timer
	nextID host.TimerID
	seq    uint64

	// MaxCallbacks bounds how many callbacks one run may dispatch, so that a
	// repeating timer nobody stops cannot hang the process.
	MaxCallbacks int
	dispatched   int
}

// DefaultMaxCallbacks is generous enough for a program that means it, and
// small enough that a runaway 『秒毎』 stops quickly.
const DefaultMaxCallbacks = 100000

// New creates a loop whose virtual clock starts at start.
func New(start time.Time) *Loop {
	return &Loop{now: start, MaxCallbacks: DefaultMaxCallbacks}
}

// Now reports the virtual time. Commands that read the clock must go through
// here, so that what they see matches the order callbacks run in.
func (l *Loop) Now() time.Time { return l.now }

// Post schedules a callback to run once at the given time.
func (l *Loop) Post(at time.Time, callback host.CallbackID) host.TimerID {
	return l.schedule(at, 0, callback)
}

// PostEvery schedules a callback to run at the given time and then repeatedly,
// one interval apart.
func (l *Loop) PostEvery(at time.Time, interval time.Duration, callback host.CallbackID) host.TimerID {
	if interval <= 0 {
		interval = time.Millisecond // 0秒毎は詰まるので最小の間隔にする
	}
	return l.schedule(at, interval, callback)
}

func (l *Loop) schedule(at time.Time, interval time.Duration, callback host.CallbackID) host.TimerID {
	l.nextID++
	l.seq++
	t := &timer{id: l.nextID, at: at, interval: interval, callback: callback, seq: l.seq}
	l.queue = append(l.queue, t)
	l.sortQueue()
	return t.id
}

// sortQueue orders by time, then by the order the timers were posted.
func (l *Loop) sortQueue() {
	sort.SliceStable(l.queue, func(i, j int) bool {
		if !l.queue[i].at.Equal(l.queue[j].at) {
			return l.queue[i].at.Before(l.queue[j].at)
		}
		return l.queue[i].seq < l.queue[j].seq
	})
}

// Cancel removes a timer. It reports whether there was one to remove.
func (l *Loop) Cancel(id host.TimerID) bool {
	for i, t := range l.queue {
		if t.id == id {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			return true
		}
	}
	return false
}

// CancelAll removes every timer.
func (l *Loop) CancelAll() { l.queue = nil }

// Pending reports how many timers are still scheduled.
func (l *Loop) Pending() int { return len(l.queue) }

// Dispatch runs one callback. The loop calls it and stops on the first error.
type Dispatch func(host.CallbackID) error

// RunUntil advances the clock to until, running every callback due on the way.
// The clock ends at until even when nothing was scheduled, which is what makes
// 『N秒待つ』 move time forward.
func (l *Loop) RunUntil(until time.Time, dispatch Dispatch) error {
	for {
		next := l.nextDue()
		if next == nil || next.at.After(until) {
			break
		}
		if err := l.runOne(next, dispatch); err != nil {
			return err
		}
	}
	if until.After(l.now) {
		l.now = until
	}
	return nil
}

// RunUntilIdle runs the one-shot callbacks that are still scheduled.
//
// Repeating timers are left alone: nothing would ever stop them, and the
// TypeScript version's process ends with them still ticking.
func (l *Loop) RunUntilIdle(dispatch Dispatch) error {
	for {
		next := l.nextDue()
		if next == nil || next.interval > 0 {
			return nil
		}
		if err := l.runOne(next, dispatch); err != nil {
			return err
		}
	}
}

// nextDue returns the earliest scheduled timer, or nil when the queue is empty.
func (l *Loop) nextDue() *timer {
	if len(l.queue) == 0 {
		return nil
	}
	return l.queue[0]
}

// runOne advances the clock to a timer's time and dispatches it, re-scheduling
// it first when it repeats.
//
// A repeating timer is re-scheduled before the callback runs, so that the
// callback can cancel it by id and have that stick.
func (l *Loop) runOne(t *timer, dispatch Dispatch) error {
	l.dispatched++
	if l.MaxCallbacks > 0 && l.dispatched > l.MaxCallbacks {
		return ErrTooManyCallbacks
	}
	if t.at.After(l.now) {
		l.now = t.at // 実時間を待たず、次の予定時刻へ飛ぶ
	}
	if t.interval > 0 {
		t.at = t.at.Add(t.interval)
		l.seq++
		t.seq = l.seq
		l.sortQueue()
	} else {
		l.Cancel(t.id)
	}
	return dispatch(t.callback)
}
