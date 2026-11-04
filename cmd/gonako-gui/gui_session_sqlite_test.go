package main

import (
	"runtime"
	"testing"
	"time"
)

// Issue #363: GUIでプログラムを再実行するたびに、閉じ忘れたSQLite接続の
// goroutineが蓄積しないことを確かめる。再実行で以前の実行が退役し、
// その接続が解放されるはずである。
func TestGUIRerunReleasesForgottenSQLiteConnections(t *testing.T) {
	session := &guiSession{}
	count := func() int {
		var n int
		for i := 0; i < 10; i++ {
			runtime.GC()
			time.Sleep(50 * time.Millisecond)
			n = runtime.NumGoroutine()
		}
		return n
	}
	// 最初の1回で画面まわりの常駐goroutineを立ち上げてから数える。
	first := session.run("「:memory:」をSQLITE3開く", "gui.nako3", false, nil, nil)
	if !first.OK {
		t.Fatalf("初回実行 = %#v", first)
	}
	// waitBelow はgoroutine数が上限以下になるまでポーリングする。
	waitBelow := func(limit int) int {
		deadline := time.Now().Add(10 * time.Second)
		for {
			runtime.GC()
			n := runtime.NumGoroutine()
			if n <= limit || time.Now().After(deadline) {
				return n
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	base := count()
	for i := 0; i < 30; i++ {
		result := session.run("「:memory:」をSQLITE3開く", "gui.nako3", false, nil, nil)
		if !result.OK {
			t.Fatalf("再実行 %d = %#v", i, result)
		}
	}
	after := waitBelow(base + 5)
	if after > base+5 {
		t.Fatalf("goroutineが蓄積した: base=%d after=%d", base, after)
	}
	// セッション自体の終了でも、最後の実行が退役すること。
	session.close()
	afterClose := waitBelow(base + 5)
	if afterClose > base+5 {
		t.Fatalf("セッション終了後もgoroutineが残った: base=%d afterClose=%d", base, afterClose)
	}
}
