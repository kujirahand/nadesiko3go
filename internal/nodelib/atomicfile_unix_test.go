//go:build unix

package nodelib

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	restore := func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &orig)
		signalNotify()
	}
	// 途中で t.Fatal しても制限が残らないよう、Cleanup でも戻す
	// (明示的に呼んだ後の二重の復元は無害)。
	t.Cleanup(restore)
	return restore
}

// 子プロセスの終了コード (親が意味を判別できるよう、0 は使わない。
// 0 で終わる場合は「子が何も実行しなかった」として親が失敗にする)。
const (
	exitLimitWriteFailed  = 17 // 期待どおり EFBIG で書き込みが失敗した
	exitLimitWriteSuccess = 18 // 制限下なのに書き込みが成功した
	exitLimitSetupFailed  = 19 // 子プロセス側の準備に失敗した
	exitLimitWrongError   = 20 // 書き込みは失敗したが EFBIG ではなかった
)

// TestWriteFileAtomicLimitChild は、RLIMIT_FSIZE をかけた子プロセスで
// writeFileAtomic を実行するためだけの入口で、通常のテスト実行ではスキップする。
// 制限をテストプロセス自体にかけると、testing が記録する testlog の
// 書き込みまで EFBIG で失敗し、全テストが合格してもパッケージが
// FAIL になる (CI で発生した)。そのため制限は子プロセスに閉じ込める。
func TestWriteFileAtomicLimitChild(t *testing.T) {
	if os.Getenv("GONAKO_ATOMIC_LIMIT_CHILD") != "1" {
		t.Skip("子プロセス専用")
	}
	dst := os.Getenv("GONAKO_ATOMIC_LIMIT_DST")
	size, err := strconv.Atoi(os.Getenv("GONAKO_ATOMIC_LIMIT_SIZE"))
	if err != nil || dst == "" {
		os.Exit(exitLimitSetupFailed)
	}
	// 子プロセスはこのまま終了するため、制限の復元は不要。
	_ = limitFileSize(t, 1024)
	werr := writeFileAtomic(dst, make([]byte, size), 0o644)
	switch {
	case werr == nil:
		os.Exit(exitLimitWriteSuccess)
	case !errors.Is(werr, syscall.EFBIG):
		os.Exit(exitLimitWrongError)
	default:
		os.Exit(exitLimitWriteFailed)
	}
}

// runAtomicWriteWithLimit は子プロセスで制限下の writeFileAtomic を実行し、
// EFBIG で失敗したこと (子プロセスの終了コード exitLimitWriteFailed) を確認する。
func runAtomicWriteWithLimit(t *testing.T, dst string, size int) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("テストバイナリのパスを取得できません: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestWriteFileAtomicLimitChild$", "-test.count=1")
	cmd.Env = append(os.Environ(),
		"GONAKO_ATOMIC_LIMIT_CHILD=1",
		"GONAKO_ATOMIC_LIMIT_DST="+dst,
		"GONAKO_ATOMIC_LIMIT_SIZE="+strconv.Itoa(size),
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("子プロセスが終了コード 0 で終わりました (制限下の書き込みが実行されていない可能性があります)\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("子プロセスを実行できませんでした: %v\n%s", err, out)
	}
	switch code := exitErr.ExitCode(); code {
	case exitLimitWriteFailed:
		// 期待どおり
	case exitLimitWriteSuccess:
		t.Fatalf("制限下なのに書き込みが成功しました\n%s", out)
	case exitLimitSetupFailed:
		t.Fatalf("子プロセスの準備に失敗しました\n%s", out)
	case exitLimitWrongError:
		t.Fatalf("書き込みは失敗しましたが EFBIG ではありません\n%s", out)
	default:
		t.Fatalf("子プロセスが想定外の終了コード %d で終わりました\n%s", code, out)
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

	runAtomicWriteWithLimit(t, dst, 8192)

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

	runAtomicWriteWithLimit(t, dst, 8192)
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Errorf("失敗した新規ファイルが残っています: %v", err)
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: umask の下でも、新規ファイルは os.WriteFile と同じ権限に
// なり、既存ファイルのモード (umask で変わる 0o666 を含む) は保持される。
// 既存ファイル側は、書き込み後の Chmod が無いと一時ファイル作成時の
// umask で 0o640 に落ちてしまうため、Chmod の有無を検出できる。
func TestWriteFileAtomicUmask(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("旧"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o666); err != nil {
		t.Fatal(err)
	}

	oldMask := syscall.Umask(0o027)
	defer syscall.Umask(oldMask)

	fresh := filepath.Join(dir, "fresh.txt")
	if err := writeFileAtomic(fresh, []byte("新規"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic(新規): %v", err)
	}
	info, err := os.Stat(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("新規ファイルのモード = %v, want 0o640 (0o644 &^ umask 0o027)", info.Mode().Perm())
	}

	if err := writeFileAtomic(existing, []byte("新"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic(既存): %v", err)
	}
	info, err = os.Stat(existing)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o666 {
		t.Errorf("既存ファイルのモード = %v, want 0o666 (umask に関わらず保持)", info.Mode().Perm())
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: rename 自体の失敗 (バインドマウントされたファイルへの
// rename は EBUSY になる) では、エラー文面に一時ファイル名が出ず、
// 既存の内容が無傷で、一時ファイルも残らない。
// バインドマウントには権限が要るため、一般ユーザーの CI ではスキップ
// される。その場合の文面の回帰防止は TestRetargetLinkError が担う。
func TestWriteFileAtomicRenameFailure(t *testing.T) {
	dir := t.TempDir()
	mounted := filepath.Join(dir, "mounted.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(mounted, []byte("マウント元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(mounted, dst, "", syscall.MS_BIND, ""); err != nil {
		t.Skipf("バインドマウントできません: %v", err)
	}
	defer func() { _ = syscall.Unmount(dst, 0) }()

	err := writeFileAtomic(dst, []byte("新しい内容"), 0o644)
	if err == nil {
		t.Fatal("マウントされたファイルへの rename が成功を返しました")
	}
	if !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("エラーに EBUSY が含まれません: %v", err)
	}
	if strings.Contains(err.Error(), ".gonako-save-") {
		t.Fatalf("エラー文面に一時ファイル名が含まれます: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "マウント元の内容" {
		t.Fatalf("内容 = %q, want マウント元の内容", got)
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
