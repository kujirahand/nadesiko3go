package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/event"
	"github.com/kujirahand/nadesiko3go/internal/host"
)

var start = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// record collects the callbacks a run dispatched, in order.
func record(out *[]host.CallbackID) event.Dispatch {
	return func(id host.CallbackID) error {
		*out = append(*out, id)
		return nil
	}
}

// TestOrderByTimeThenPost pins the two rules that make the order observable:
// earlier time first, and within the same time, the order they were posted.
func TestOrderByTimeThenPost(t *testing.T) {
	l := event.New(start)
	l.Post(start.Add(50*time.Millisecond), 1)
	l.Post(start.Add(10*time.Millisecond), 2)
	l.Post(start.Add(10*time.Millisecond), 3) // 2と同時刻。積んだ順で後。

	var got []host.CallbackID
	if err := l.RunUntil(start.Add(time.Second), record(&got)); err != nil {
		t.Fatal(err)
	}
	want := []host.CallbackID{2, 3, 1}
	if len(got) != len(want) {
		t.Fatalf("順序 = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("順序 = %v, want %v", got, want)
		}
	}
}

// TestClockJumps pins that the clock moves to the callback's time instead of
// waiting for it, which is what keeps 『3秒後』 cheap to run.
func TestClockJumps(t *testing.T) {
	l := event.New(start)
	l.Post(start.Add(3*time.Second), 1)

	var seen time.Time
	err := l.RunUntil(start.Add(10*time.Second), func(host.CallbackID) error {
		seen = l.Now()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seen.Equal(start.Add(3 * time.Second)) {
		t.Errorf("コールバック中の時刻 = %v, want +3s", seen.Sub(start))
	}
	// 予定がなくても、待った分だけ時計は進む
	if !l.Now().Equal(start.Add(10 * time.Second)) {
		t.Errorf("終了時の時刻 = %v, want +10s", l.Now().Sub(start))
	}
}

// TestRunUntilStopsAtLimit pins that a callback scheduled past the deadline is
// left in the queue.
func TestRunUntilStopsAtLimit(t *testing.T) {
	l := event.New(start)
	l.Post(start.Add(10*time.Millisecond), 1)
	l.Post(start.Add(500*time.Millisecond), 2)

	var got []host.CallbackID
	if err := l.RunUntil(start.Add(100*time.Millisecond), record(&got)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("実行したもの = %v, want [1]", got)
	}
	if l.Pending() != 1 {
		t.Errorf("残り = %d, want 1", l.Pending())
	}
}

func TestRepeatAndCancel(t *testing.T) {
	l := event.New(start)
	id := l.PostEvery(start.Add(10*time.Millisecond), 10*time.Millisecond, 1)

	count := 0
	err := l.RunUntil(start.Add(time.Second), func(host.CallbackID) error {
		count++
		if count == 3 {
			// コールバックの中から自分のタイマーを止められる
			if !l.Cancel(id) {
				t.Error("Cancel が false を返した")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("実行回数 = %d, want 3", count)
	}
}

// TestRunUntilIdleLeavesRepeating pins that the tail of a run finishes the
// one-shot callbacks and leaves the repeating ones alone, which would
// otherwise never end.
func TestRunUntilIdleLeavesRepeating(t *testing.T) {
	l := event.New(start)
	l.PostEvery(start.Add(10*time.Millisecond), 10*time.Millisecond, 1)
	l.Post(start.Add(5*time.Millisecond), 2)

	var got []host.CallbackID
	if err := l.RunUntilIdle(record(&got)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("実行したもの = %v, want [2]", got)
	}
	if l.Pending() != 1 {
		t.Errorf("残り = %d, want 1 (繰り返しタイマー)", l.Pending())
	}
}

// TestMaxCallbacks pins the guard against a repeating timer nobody stops.
func TestMaxCallbacks(t *testing.T) {
	l := event.New(start)
	l.MaxCallbacks = 5
	l.PostEvery(start.Add(time.Millisecond), time.Millisecond, 1)

	err := l.RunUntil(start.Add(time.Hour), func(host.CallbackID) error { return nil })
	if !errors.Is(err, event.ErrTooManyCallbacks) {
		t.Errorf("err = %v, want ErrTooManyCallbacks", err)
	}
}

// TestDispatchErrorStops pins that an error inside a callback ends the run
// rather than being swallowed.
func TestDispatchErrorStops(t *testing.T) {
	l := event.New(start)
	l.Post(start.Add(time.Millisecond), 1)
	l.Post(start.Add(2*time.Millisecond), 2)

	boom := errors.New("失敗")
	count := 0
	err := l.RunUntil(start.Add(time.Second), func(host.CallbackID) error {
		count++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want 失敗", err)
	}
	if count != 1 {
		t.Errorf("実行回数 = %d, want 1", count)
	}
}
