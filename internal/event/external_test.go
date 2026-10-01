package event

import (
	"testing"
	"time"
)

func TestExternalEventLifecycle(t *testing.T) {
	l := New(time.Time{})
	ready := false
	calls, closed := 0, 0
	l.PostExternal(func() bool { return ready }, func() error {
		calls++
		// ハンドラのVM実行が再度ポーリングしても再入しない。
		return l.PollExternal()
	}, func() { closed++ })
	if err := l.PollExternal(); err != nil || calls != 0 {
		t.Fatalf("未受信イベントを実行: calls=%d, err=%v", calls, err)
	}
	ready = true
	for i := 0; i < 2; i++ {
		if err := l.PollExternal(); err != nil {
			t.Fatal(err)
		}
	}
	l.CloseExternal()
	l.CloseExternal()
	if calls != 1 || closed != 1 {
		t.Fatalf("calls=%d, closed=%d", calls, closed)
	}
}
