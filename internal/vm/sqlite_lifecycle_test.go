package vm_test

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func goroutineCount(t *testing.T) int {
	t.Helper()
	// GCを数回回して、回収待ちのgoroutineが消えるまで少し待つ。
	var n int
	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	return n
}

// waitGoroutinesAtMost はgoroutine数が上限以下になるまでポーリングする。
// プロセス全体の数を数えるため、一時的な増減に揺れないよう待ちを入れる。
func waitGoroutinesAtMost(t *testing.T, limit int) int {
	t.Helper()
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

// Issue #363: 『SQLITE3開く』で開いたまま閉じ忘れたプログラムを繰り返し
// 実行しても、接続に紐づくgoroutineが蓄積しないことを確かめる。
// 一回限りの実行入口（RunProgram）は実行の終了時に環境を退役させる。
func TestSQLiteForgottenCloseDoesNotAccumulateGoroutines(t *testing.T) {
	base := goroutineCount(t)
	for i := 0; i < 30; i++ {
		got, err := runCUICommands(t, "「:memory:」をSQLITE3開く")
		if err != nil {
			t.Fatalf("実行 %d: %v (出力 %q)", i, err, got)
		}
	}
	after := waitGoroutinesAtMost(t, base+5)
	if after > base+5 {
		t.Fatalf("goroutineが蓄積した: base=%d after=%d", base, after)
	}
}

// 異常終了した実行でも接続が解放されることを確かめる。
func TestSQLiteErrorRunDoesNotAccumulateGoroutines(t *testing.T) {
	base := goroutineCount(t)
	for i := 0; i < 30; i++ {
		_, err := runCUICommands(t, "「:memory:」をSQLITE3開く\n「存在しない表」を[]でSQLITE3実行")
		if err == nil {
			t.Fatal("存在しない表への実行はエラーになるはず")
		}
	}
	after := waitGoroutinesAtMost(t, base+5)
	if after > base+5 {
		t.Fatalf("goroutineが蓄積した: base=%d after=%d", base, after)
	}
}

// メイン処理の後に残るコールバックが動く間は接続を閉じてはいけない。
// コールバックの中でSQLを実行し、その結果が見えることで確かめる。
func TestSQLiteCallbackRunsBeforeEnvironmentRetires(t *testing.T) {
	code := strings.Join([]string{
		`「:memory:」をSQLITE3開く`,
		`「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行`,
		`F=関数()`,
		`「INSERT INTO t VALUES(?)」を["値"]でSQLITE3実行`,
		`R=「SELECT v FROM t」を[]でSQLITE3取得`,
		`R["v"]を表示`,
		`ここまで`,
		`Fに「SELECT 1」を[]でSQLITE3実行時`,
		`「メイン終わり」と表示`,
		// 明示的に閉じない。環境の退役はコールバックの後でなければならない。
	}, "\n")
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}
	if want := "メイン終わり\n値"; got != want {
		t.Fatalf("log = %q, want %q", got, want)
	}
}
