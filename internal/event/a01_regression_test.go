package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/event"
	"github.com/kujirahand/nadesiko3go/internal/host"
)

// A01: タイマーの追加と個別キャンセルが件数に対して二次的に重くならないこと。
// ここでは件数を上げても順序・ID払い出し・停止の意味が変わらないことを固定する。

// TestA01SameTimeFIFO は、同一時刻に積んだ単発タイマーが積んだ順に一度だけ
// 実行されることを大きな件数で確かめる。
func TestA01SameTimeFIFO(t *testing.T) {
	const n = 1024
	l := event.New(start)
	for i := 1; i <= n; i++ {
		l.Post(start, host.CallbackID(i))
	}
	if got := l.Pending(); got != n {
		t.Fatalf("登録直後の残り = %d, want %d", got, n)
	}

	var got []host.CallbackID
	if err := l.RunUntilIdle(record(&got)); err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("実行件数 = %d, want %d", len(got), n)
	}
	for i, id := range got {
		if want := host.CallbackID(i + 1); id != want {
			t.Fatalf("実行順[%d] = %d, want %d", i, id, want)
		}
	}
	if got := l.Pending(); got != 0 {
		t.Errorf("実行後の残り = %d, want 0", got)
	}
}

// TestA01MixedTimeOrder は、予定時刻がばらばらでも時刻順に実行されることを
// 確かめる。積んだ順は実行順と一致しない。
func TestA01MixedTimeOrder(t *testing.T) {
	const n = 1000
	l := event.New(start)
	// 後から積むほど予定時刻が早いので、実行順は積んだ順の逆になる
	for i := 1; i <= n; i++ {
		l.Post(start.Add(time.Duration(n-i)*time.Millisecond), host.CallbackID(i))
	}

	var got []host.CallbackID
	if err := l.RunUntil(start.Add(time.Hour), record(&got)); err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("実行件数 = %d, want %d", len(got), n)
	}
	for i, id := range got {
		if want := host.CallbackID(n - i); id != want {
			t.Fatalf("実行順[%d] = %d, want %d", i, id, want)
		}
	}
}

// TestA01RegisterAndCancelAtScale は 8k/16k/32k の登録と個別キャンセルで
// 残り件数が正しく、残った単発が積んだ順に実行されることを確かめる。
func TestA01RegisterAndCancelAtScale(t *testing.T) {
	for _, n := range []int{8192, 16384, 32768} {
		l := event.New(start)
		ids := make([]host.TimerID, n)
		for i := range ids {
			ids[i] = l.Post(start, host.CallbackID(i))
		}
		if got := l.Pending(); got != n {
			t.Fatalf("n=%d 登録直後の残り = %d, want %d", n, got, n)
		}

		for i := 0; i < n; i += 2 {
			if !l.Cancel(ids[i]) {
				t.Fatalf("n=%d 偶数番のキャンセルが失敗: i=%d", n, i)
			}
		}
		if got := l.Pending(); got != n/2 {
			t.Fatalf("n=%d キャンセル後の残り = %d, want %d", n, got, n/2)
		}
		if l.Cancel(ids[0]) {
			t.Fatalf("n=%d キャンセル済みIDの再キャンセルが true", n)
		}

		var got []host.CallbackID
		if err := l.RunUntilIdle(record(&got)); err != nil {
			t.Fatal(err)
		}
		if len(got) != n/2 {
			t.Fatalf("n=%d 実行件数 = %d, want %d", n, len(got), n/2)
		}
		for k, id := range got {
			if want := host.CallbackID(2*k + 1); id != want {
				t.Fatalf("n=%d 実行順[%d] = %d, want %d", n, k, id, want)
			}
		}
		if got := l.Pending(); got != 0 {
			t.Fatalf("n=%d 実行後の残り = %d, want 0", n, got)
		}
	}
}

// TestA01CancelAll は全キャンセルが単発も繰り返しも外し、IDの払い出しは
// 巻き戻さないことを確かめる。
func TestA01CancelAll(t *testing.T) {
	l := event.New(start)
	l.Post(start.Add(time.Millisecond), 1)
	repeatID := l.PostEvery(start.Add(time.Millisecond), time.Millisecond, 2)
	if got := l.Pending(); got != 2 {
		t.Fatalf("登録直後の残り = %d, want 2", got)
	}

	l.CancelAll()
	if got := l.Pending(); got != 0 {
		t.Errorf("全停止後の残り = %d, want 0", got)
	}
	if l.Cancel(repeatID) {
		t.Error("全停止後の Cancel が true")
	}
	if id := l.Post(start, 3); id != 3 {
		t.Errorf("全停止後の新しいID = %d, want 3", id)
	}
}

// TestA01DrainSkipsRepeat は、繰り返しタイマーが先頭にあっても単発の
// 取りこぼしがなく、繰り返しは残ることを確かめる。
func TestA01DrainSkipsRepeat(t *testing.T) {
	l := event.New(start)
	l.PostEvery(start.Add(5*time.Millisecond), 10*time.Millisecond, 1)
	l.Post(start.Add(50*time.Millisecond), 2)
	l.Post(start.Add(30*time.Millisecond), 3)

	var got []host.CallbackID
	if err := l.RunUntilIdle(record(&got)); err != nil {
		t.Fatal(err)
	}
	want := []host.CallbackID{3, 2}
	if len(got) != len(want) {
		t.Fatalf("実行したもの = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("実行したもの = %v, want %v", got, want)
		}
	}
	if p := l.Pending(); p != 1 {
		t.Errorf("残り = %d, want 1 (繰り返しタイマー)", p)
	}
}

// TestA01RepeatSelfStop は、繰り返しタイマーがコールバックから自分を止められ、
// 別の繰り返しは止めた後も動き続けることを確かめる。
func TestA01RepeatSelfStop(t *testing.T) {
	l := event.New(start)
	selfID := l.PostEvery(start, 10*time.Millisecond, 1)
	otherID := l.PostEvery(start, 15*time.Millisecond, 2)

	counts := map[host.CallbackID]int{}
	err := l.RunUntil(start.Add(65*time.Millisecond), func(id host.CallbackID) error {
		counts[id]++
		if id == 1 && counts[id] == 3 {
			if !l.Cancel(selfID) {
				t.Error("自分のタイマーの Stop が false")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if counts[1] != 3 {
		t.Errorf("自分を止めた側の実行回数 = %d, want 3", counts[1])
	}
	if counts[2] != 5 {
		t.Errorf("残った側の実行回数 = %d, want 5", counts[2])
	}
	if ids := l.Pending(); ids != 1 {
		t.Errorf("残り = %d, want 1", ids)
	}
	if l.Cancel(otherID) != true {
		t.Error("残った繰り返しタイマーを停止できなかった")
	}
}

// TestA01DispatchErrorStops は、コールバックのエラーで実行が止まり、
// 停止時点の残り件数が保たれることを確かめる。
func TestA01DispatchErrorStops(t *testing.T) {
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
	if p := l.Pending(); p != 1 {
		t.Errorf("停止時の残り = %d, want 1", p)
	}
}

// TestA01MaxCallbacks は、止まらない繰り返しタイマーが上限で打ち切られ、
// タイマー自体は残ることを確かめる。
func TestA01MaxCallbacks(t *testing.T) {
	l := event.New(start)
	l.MaxCallbacks = 5
	l.PostEvery(start.Add(time.Millisecond), time.Millisecond, 1)

	err := l.RunUntil(start.Add(time.Hour), func(host.CallbackID) error { return nil })
	if !errors.Is(err, event.ErrTooManyCallbacks) {
		t.Errorf("err = %v, want ErrTooManyCallbacks", err)
	}
	if p := l.Pending(); p != 1 {
		t.Errorf("打ち切り時の残り = %d, want 1", p)
	}
}

// BenchmarkA01Post は、同一時刻にまとめて積む最悪ケースの登録コストを測る。
func BenchmarkA01Post(b *testing.B) {
	for _, n := range []int{8192, 16384, 32768} {
		b.Run(sizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				l := event.New(start)
				for j := 0; j < n; j++ {
					l.Post(start, host.CallbackID(j))
				}
				if l.Pending() != n {
					b.Fatalf("残り = %d, want %d", l.Pending(), n)
				}
			}
		})
	}
}

// BenchmarkA01Cancel は、積んだ後の個別キャンセルだけを測る。
func BenchmarkA01Cancel(b *testing.B) {
	for _, n := range []int{8192, 16384, 32768} {
		b.Run(sizeName(n), func(b *testing.B) {
			ids := make([]host.TimerID, n)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := event.New(start)
				for j := 0; j < n; j++ {
					ids[j] = l.Post(start, host.CallbackID(j))
				}
				b.StartTimer()
				for j := 0; j < n; j++ {
					if !l.Cancel(ids[j]) {
						b.Fatalf("キャンセルが失敗: j=%d", j)
					}
				}
				if l.Pending() != 0 {
					b.Fatalf("残り = %d, want 0", l.Pending())
				}
			}
		})
	}
}

// BenchmarkA01Drain は、積んだ後の単発drainだけを測る。予定時刻を混ぜて
// ヒープの入れ替えも通す。
func BenchmarkA01Drain(b *testing.B) {
	for _, n := range []int{8192, 16384, 32768} {
		b.Run(sizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := event.New(start)
				for j := 0; j < n; j++ {
					l.Post(start.Add(time.Duration(j%97)*time.Millisecond), host.CallbackID(j))
				}
				b.StartTimer()
				ran := 0
				if err := l.RunUntilIdle(func(host.CallbackID) error {
					ran++
					return nil
				}); err != nil {
					b.Fatal(err)
				}
				if ran != n || l.Pending() != 0 {
					b.Fatalf("実行 = %d, 残り = %d, want %d, 0", ran, l.Pending(), n)
				}
			}
		})
	}
}

// sizeName は件数を 8k/16k/32k のサブベンチマーク名に変換する。
func sizeName(n int) string {
	return map[int]string{8192: "8k", 16384: "16k", 32768: "32k"}[n]
}
