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
	"container/heap"
	"errors"
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

	// ヒープ内の位置。-1 は未収容。要素を任意の位置から O(log n) で外すために持つ。
	inAll  int
	inOnce int
}

// timerHeap orders timers by time, then by the order they were posted. Write
// back each element's position so that Cancel can remove one from the middle.
type timerHeap struct {
	items []*timer
	pos   func(*timer) *int
}

func (h *timerHeap) Len() int { return len(h.items) }

func (h *timerHeap) Less(i, j int) bool {
	a, b := h.items[i], h.items[j]
	if !a.at.Equal(b.at) {
		return a.at.Before(b.at)
	}
	return a.seq < b.seq
}

func (h *timerHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	*h.pos(h.items[i]) = i
	*h.pos(h.items[j]) = j
}

func (h *timerHeap) Push(x any) {
	t := x.(*timer)
	*h.pos(t) = len(h.items)
	h.items = append(h.items, t)
}

func (h *timerHeap) Pop() any {
	n := len(h.items) - 1
	t := h.items[n]
	h.items[n] = nil
	h.items = h.items[:n]
	*h.pos(t) = -1
	return t
}

// Loop is the timer queue. It is not safe for concurrent use, which is the
// point: a single thread of control keeps the order observable.
type Loop struct {
	now time.Time

	// all holds every timer and oneShots only the one-shot ones, both in the
	// same time-then-seq order. RunUntil drains all, RunUntilIdle pops oneShots
	// so that a repeating timer at the head never needs a linear scan.
	// byID makes Cancel independent of the queue size.
	all      timerHeap
	oneShots timerHeap
	byID     map[host.TimerID]*timer

	nextID host.TimerID
	seq    uint64

	// MaxCallbacks bounds how many callbacks one run may dispatch, so that a
	// repeating timer nobody stops cannot hang the process.
	MaxCallbacks int
	dispatched   int
	external     []externalEvent
	polling      bool
}

// externalEventは受信の確認も実行もVMと同じスレッドで行う。
type externalEvent struct {
	ready func() bool
	run   func() error
	close func()
}

// PostExternalは外部イベントを登録する。readyは受信チャネルを非ブロッキングで確認する。
func (l *Loop) PostExternal(ready func() bool, run func() error, close func()) {
	l.external = append(l.external, externalEvent{ready: ready, run: run, close: close})
}

// PollExternalは受信済みのイベントを単発で実行する。ハンドラ内では再入しない。
func (l *Loop) PollExternal() error {
	if l.polling || len(l.external) == 0 {
		return nil
	}
	l.polling = true
	defer func() { l.polling = false }()
	for i := 0; i < len(l.external); i++ {
		e := &l.external[i]
		if e.ready != nil && e.ready() {
			run := e.run
			// 受信済みの単発イベントは確認を打ち切り、以後のポーリングで
			// readyやハンドラを再実行しない。ハンドラがエラーを返す場合も同様。
			e.ready = nil
			if err := run(); err != nil {
				return err
			}
		}
	}
	return nil
}

// CloseExternalは実行終了時に外部イベントの登録を解除する。
// 解除後は登録一覧を空にするため、繰り返し呼んでも解除処理を重複実行しない。
func (l *Loop) CloseExternal() {
	for _, e := range l.external {
		if e.close != nil {
			e.close()
		}
	}
	l.external = nil
}

// DefaultMaxCallbacks is generous enough for a program that means it, and
// small enough that a runaway 『秒毎』 stops quickly.
const DefaultMaxCallbacks = 100000

// New creates a loop whose virtual clock starts at start.
func New(start time.Time) *Loop {
	l := &Loop{
		now:          start,
		byID:         make(map[host.TimerID]*timer),
		MaxCallbacks: DefaultMaxCallbacks,
	}
	l.all.pos = func(t *timer) *int { return &t.inAll }
	l.oneShots.pos = func(t *timer) *int { return &t.inOnce }
	return l
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
	t := &timer{
		id:       l.nextID,
		at:       at,
		interval: interval,
		callback: callback,
		seq:      l.seq,
		inAll:    -1,
		inOnce:   -1,
	}
	l.byID[t.id] = t
	heap.Push(&l.all, t)
	if interval == 0 {
		heap.Push(&l.oneShots, t)
	}
	return t.id
}

// remove takes one timer out of both heaps and the index.
func (l *Loop) remove(t *timer) {
	heap.Remove(&l.all, t.inAll)
	if t.inOnce >= 0 {
		heap.Remove(&l.oneShots, t.inOnce)
	}
	delete(l.byID, t.id)
}

// Cancel removes a timer. It reports whether there was one to remove.
func (l *Loop) Cancel(id host.TimerID) bool {
	t, ok := l.byID[id]
	if !ok {
		return false
	}
	l.remove(t)
	return true
}

// CancelAll removes every timer.
func (l *Loop) CancelAll() {
	l.all.items = nil
	l.oneShots.items = nil
	clear(l.byID)
}

// Pending reports how many timers are still scheduled.
func (l *Loop) Pending() int { return len(l.byID) }

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
// TypeScript version's process ends with them still ticking. A repeat that
// sits at the head of the queue is skipped rather than ending the drain,
// so the one-shots behind it still run.
func (l *Loop) RunUntilIdle(dispatch Dispatch) error {
	for {
		next := l.nextDueOneShot()
		if next == nil {
			return nil
		}
		if err := l.runOne(next, dispatch); err != nil {
			return err
		}
	}
}

// nextDue returns the earliest scheduled timer, or nil when the queue is empty.
func (l *Loop) nextDue() *timer {
	if l.all.Len() == 0 {
		return nil
	}
	return l.all.items[0]
}

// nextDueOneShot returns the earliest one-shot still scheduled, skipping the
// repeating timers that would otherwise hide everything queued behind them.
func (l *Loop) nextDueOneShot() *timer {
	if l.oneShots.Len() == 0 {
		return nil
	}
	return l.oneShots.items[0]
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
		heap.Fix(&l.all, t.inAll)
	} else {
		l.remove(t)
	}
	return dispatch(t.callback)
}
