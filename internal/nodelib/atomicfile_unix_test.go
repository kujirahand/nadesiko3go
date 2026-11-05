//go:build unix

package nodelib

import (
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// ignoreXFSZ は SIGXFSZ を無視し、元へ戻す関数を返す。無視しないと
// RLIMIT_FSIZE 超過時に書き込み側プロセスがシグナルで終了してしまい、
// EFBIG エラーとしての失敗を確認できない。
func ignoreXFSZ() func() {
	signal.Ignore(syscall.SIGXFSZ)
	return func() { signal.Reset(syscall.SIGXFSZ) }
}

// limitFileSize は RLIMIT_FSIZE を設定し、元へ戻す関数を返す。
// 容量不足による書き込み失敗を、root でも確実に再現するための注入方法。
// SIGXFSZ は無視し、プロセスが終了せず EFBIG エラーとして返るようにする。
func limitFileSize(t *testing.T, limit uint64) func() {
	t.Helper()
	signalNotify := ignoreXFSZ()
	var orig syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &orig); err != nil {
		signalNotify()
		t.Fatalf("RLIMIT_FSIZE を取得できません: %v", err)
	}
	lim := orig
	lim.Cur = limit
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
		signalNotify()
		t.Fatalf("RLIMIT_FSIZE を設定できません: %v", err)
	}
	return func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &orig)
		signalNotify()
	}
}

// Issue #425: 書き込みが途中で失敗しても、既存ファイルの内容とモードは
// 無傷で、一時ファイルも残らない。これが本 Issue の中核の回帰テストで、
// 修正前の os.WriteFile では元内容が書きかけに置き換わって失敗する。
func TestWriteFileAtomicFailureKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "a.txt")
	original := []byte("元の内容")
	if err := os.WriteFile(dst, original, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dst, 0o640); err != nil {
		t.Fatal(err)
	}

	restore := limitFileSize(t, 1024)
	err := writeFileAtomic(dst, make([]byte, 8192), 0o644)
	restore()
	if err == nil {
		t.Fatal("容量制限下の書き込みが成功を返しました")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("既存の内容が壊れています: %q", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("モード = %v, want 0o640", info.Mode().Perm())
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 新規作成が途中で失敗した場合は、ファイル自体が残らない。
func TestWriteFileAtomicFailureLeavesNoNewFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "new.txt")

	restore := limitFileSize(t, 1024)
	err := writeFileAtomic(dst, make([]byte, 8192), 0o644)
	restore()
	if err == nil {
		t.Fatal("容量制限下の書き込みが成功を返しました")
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Errorf("失敗した新規ファイルが残っています: %v", err)
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: FIFO は特殊ファイルなので rename で置き換えず、
// 従来どおり直接書き込む。読み手が受け取れ、FIFO のまま残る。
func TestWriteFileAtomicFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "a.fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatalf("FIFOを作成できません: %v", err)
	}
	data := []byte("fifo-data")

	readResult := make(chan []byte, 1)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			readResult <- nil
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		readResult <- b
	}()

	writeResult := make(chan error, 1)
	go func() {
		writeResult <- writeFileAtomic(fifo, data, 0o644)
	}()
	select {
	case err := <-writeResult:
		if err != nil {
			t.Fatalf("writeFileAtomic: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FIFO への書き込みが完了しません")
	}

	select {
	case got := <-readResult:
		if string(got) != string(data) {
			t.Fatalf("読み手の内容 = %q, want %q", got, data)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FIFO の読み手が完了しません")
	}

	info, err := os.Lstat(fifo)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("FIFO が FIFO ではなくなっています: %v", info.Mode())
	}
	assertNoAtomicTempLeftover(t, dir)
}
