//go:build unix

package nodelib_test

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// limitFileSizeForSave は RLIMIT_FSIZE を設定し、元へ戻す関数を返す。
// 容量不足による書き込み失敗を命令レベルで再現するための注入方法 (#425)。
// SIGXFSZ は無視し、プロセスが終了せずエラーとして返るようにする。
func limitFileSizeForSave(t *testing.T, limit uint64) func() {
	t.Helper()
	signal.Ignore(syscall.SIGXFSZ)
	var orig syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &orig); err != nil {
		signal.Reset(syscall.SIGXFSZ)
		t.Fatalf("RLIMIT_FSIZE を取得できません: %v", err)
	}
	lim := orig
	lim.Cur = limit
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
		signal.Reset(syscall.SIGXFSZ)
		t.Fatalf("RLIMIT_FSIZE を設定できません: %v", err)
	}
	restore := func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &orig)
		signal.Reset(syscall.SIGXFSZ)
	}
	// 途中で t.Fatal しても制限が残らないよう、Cleanup でも戻す
	// (明示的に呼んだ後の二重の復元は無害)。
	t.Cleanup(restore)
	return restore
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
	restore := limitFileSizeForSave(t, 1024)
	err := runExpectError(t, dir, code)
	restore()
	if err == nil {
		t.Fatal("容量制限下の保存が成功を返しました")
	}

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
	restore := limitFileSizeForSave(t, 1024)
	err := runExpectError(t, dir, code)
	restore()
	if err == nil {
		t.Fatal("容量制限下の EUC 保存が成功を返しました")
	}

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

	restore := limitFileSizeForSave(t, 1024)
	err := runExpectError(t, dir, `「src.txt」を「dest.txt」にファイル上書コピー`)
	restore()
	if err == nil {
		t.Fatal("容量制限下のコピーが成功を返しました")
	}

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

	restore := limitFileSizeForSave(t, 1024)
	err := runExpectError(t, dir, `「srcdir」を「destdir」にファイル上書コピー`)
	restore()
	if err == nil {
		t.Fatal("容量制限下のフォルダコピーが成功を返しました")
	}

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
	restore := limitFileSizeForSave(t, 1024)
	err := runExpectError(t, dir, code)
	restore()
	if err == nil {
		t.Fatal("容量制限下の SJIS 保存が成功を返しました")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "元の内容" {
		t.Fatalf("既存の内容が壊れています: %q", got)
	}
	assertNoSaveTempLeftover(t, dir)
}
