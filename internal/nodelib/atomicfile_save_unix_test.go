//go:build unix

package nodelib_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 子プロセスの終了コード (親が意味を判別できるよう、0 は使わない。
// 0 で終わる場合は「子が何も実行しなかった」として親が失敗にする)。
const (
	exitLimitSaveFailed  = 17 // 期待どおり容量制限 (file too large) で保存が失敗した
	exitLimitSaveSuccess = 18 // 制限下なのに保存が成功した
	exitLimitSetupFailed = 19 // 子プロセス側の準備に失敗した
	exitLimitWrongError  = 20 // 保存は失敗したが容量制限によるものではなかった
)

// TestSaveWithFileSizeLimitChild は、RLIMIT_FSIZE をかけた子プロセスで
// なでしこプログラムを実行するためだけの入口で、通常のテスト実行では
// スキップする。制限をテストプロセス自体にかけると、testing が記録する
// testlog の書き込みまで EFBIG で失敗し、全テストが合格しても
// パッケージが FAIL になる (CI で発生した)。そのため制限は
// 子プロセスに閉じ込める (#425 の失敗注入方法)。
func TestSaveWithFileSizeLimitChild(t *testing.T) {
	if os.Getenv("GONAKO_SAVE_LIMIT_CHILD") != "1" {
		t.Skip("子プロセス専用")
	}
	dir := os.Getenv("GONAKO_SAVE_LIMIT_DIR")
	code := os.Getenv("GONAKO_SAVE_LIMIT_CODE")
	// SIGXFSZ は無視し、プロセスが終了せずエラーとして返るようにする。
	signal.Ignore(syscall.SIGXFSZ)
	var orig syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &orig); err != nil {
		os.Exit(exitLimitSetupFailed)
	}
	lim := orig
	lim.Cur = 1024
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
		os.Exit(exitLimitSetupFailed)
	}
	err := runExpectError(t, dir, code)
	switch {
	case err == nil:
		os.Exit(exitLimitSaveSuccess)
	case !strings.Contains(err.Error(), "file too large"):
		os.Exit(exitLimitWrongError)
	default:
		os.Exit(exitLimitSaveFailed)
	}
}

// runSaveWithFileSizeLimit は子プロセスで制限下のプログラムを実行し、
// 容量制限で保存が失敗したこと (子プロセスの終了コード
// exitLimitSaveFailed) を確認する。
func runSaveWithFileSizeLimit(t *testing.T, dir, code string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("テストバイナリのパスを取得できません: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestSaveWithFileSizeLimitChild$", "-test.count=1")
	cmd.Env = append(os.Environ(),
		"GONAKO_SAVE_LIMIT_CHILD=1",
		"GONAKO_SAVE_LIMIT_DIR="+dir,
		"GONAKO_SAVE_LIMIT_CODE="+code,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("子プロセスが終了コード 0 で終わりました (制限下の保存が実行されていない可能性があります)\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("子プロセスを実行できませんでした: %v\n%s", err, out)
	}
	switch c := exitErr.ExitCode(); c {
	case exitLimitSaveFailed:
		// 期待どおり
	case exitLimitSaveSuccess:
		t.Fatalf("制限下なのに保存が成功しました\n%s", out)
	case exitLimitSetupFailed:
		t.Fatalf("子プロセスの準備に失敗しました\n%s", out)
	case exitLimitWrongError:
		t.Fatalf("保存は失敗しましたが容量制限によるものではありません\n%s", out)
	default:
		t.Fatalf("子プロセスが想定外の終了コード %d で終わりました\n%s", c, out)
	}
}

func assertNoSaveTempLeftover(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gonako-save-") {
			t.Errorf("一時ファイルが残っています: %s", e.Name())
		}
	}
}

// Issue #425 の回帰テスト: 『保存』が途中で失敗しても、既存ファイルの
// 内容は無傷で、一時ファイルも残らない。修正前は元内容が書きかけの
// 内容に置き換わって失われていた。
func TestSaveFailureKeepsExistingContent(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(dst, []byte("元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := "「" + strings.Repeat("y", 4096) + "」を\"a.txt\"に保存"
	runSaveWithFileSizeLimit(t, dir, code)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "元の内容" {
		t.Fatalf("既存の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dir)
}

// Issue #425 の回帰テスト: 『EUCファイル保存』が途中で失敗しても、
// 既存ファイルの内容は無傷で、一時ファイルも残らない。
func TestEUCSaveFailureKeepsExistingContent(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "enc.txt")
	if err := os.WriteFile(dst, []byte("元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := "「" + strings.Repeat("あ", 1500) + "」を\"enc.txt\"にEUCファイル保存"
	runSaveWithFileSizeLimit(t, dir, code)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "元の内容" {
		t.Fatalf("既存の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dir)
}

// Issue #425 の回帰テスト: 『ファイル上書コピー』が途中で失敗しても、
// コピー先の既存内容は無傷で、一時ファイルも残らない。
func TestOverwriteCopyFailureKeepsDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(src, []byte(strings.Repeat("s", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("コピー先の元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	runSaveWithFileSizeLimit(t, dir, `「src.txt」を「dest.txt」にファイル上書コピー`)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "コピー先の元の内容" {
		t.Fatalf("コピー先の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dir)
}

// Issue #425 の回帰テスト: フォルダの上書コピーで中のファイルの
// 書き込みが途中で失敗しても、コピー先の既存ファイルは無傷で、
// 一時ファイルも残らない (file.go のフォルダマージコピー経路)。
func TestFolderOverwriteCopyFailureKeepsDestination(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "srcdir")
	dstDir := filepath.Join(dir, "destdir")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte(strings.Repeat("s", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	dstFile := filepath.Join(dstDir, "f.txt")
	if err := os.WriteFile(dstFile, []byte("フォルダコピー先の元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	runSaveWithFileSizeLimit(t, dir, `「srcdir」を「destdir」にファイル上書コピー`)

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "フォルダコピー先の元の内容" {
		t.Fatalf("コピー先の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dstDir)
}

// Issue #425 の回帰テスト: 『SJISファイル保存』が途中で失敗しても、
// 既存ファイルの内容は無傷で、一時ファイルも残らない。
func TestSJISSaveFailureKeepsExistingContent(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "enc.txt")
	if err := os.WriteFile(dst, []byte("元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := "「" + strings.Repeat("あ", 1500) + "」を\"enc.txt\"にSJISファイル保存"
	runSaveWithFileSizeLimit(t, dir, code)

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "元の内容" {
		t.Fatalf("既存の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dir)
}
