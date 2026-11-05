package nodelib

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// assertNoAtomicTempLeftover は、dir の中に一時ファイル (.gonako-save-*)
// が残っていないことを確認する (#425)。
func assertNoAtomicTempLeftover(t *testing.T, dir string) {
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

// Issue #425: 新規ファイルは現行の os.WriteFile と同じ内容・権限で作られる。
func TestWriteFileAtomicNewFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "new.txt")
	ctrl := filepath.Join(dir, "ctrl.txt")
	data := []byte("新しい内容\n")

	if err := writeFileAtomic(dst, data, 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if err := os.WriteFile(ctrl, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("内容 = %q, want %q", got, data)
	}
	if runtime.GOOS != "windows" {
		dstInfo, err := os.Stat(dst)
		if err != nil {
			t.Fatal(err)
		}
		ctrlInfo, err := os.Stat(ctrl)
		if err != nil {
			t.Fatal(err)
		}
		if dstInfo.Mode().Perm() != ctrlInfo.Mode().Perm() {
			t.Errorf("権限 = %v, want os.WriteFile と同じ %v", dstInfo.Mode().Perm(), ctrlInfo.Mode().Perm())
		}
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 既存ファイルの上書きでは内容が置き換わり、モードは保持される。
func TestWriteFileAtomicOverwritePreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("モードの保持を確認する Unix 向けのテスト")
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(dst, []byte("旧内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dst, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(dst, []byte("新内容"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "新内容" {
		t.Fatalf("内容 = %q, want 新内容", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("モード = %v, want 0o640 (既存モードの保持)", info.Mode().Perm())
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 通常ファイルへの symlink は辿り、リンクを維持したまま
// 実体だけを置き換える。実体のモードも保持される。
func TestWriteFileAtomicSymlinkFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("実体の旧内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(target, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("target.txt", link); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}

	if err := writeFileAtomic(link, []byte("実体の新内容"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link.txt が symlink ではなくなっています")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "実体の新内容" {
		t.Fatalf("実体の内容 = %q, want 実体の新内容", got)
	}
	if runtime.GOOS != "windows" {
		tinfo, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if tinfo.Mode().Perm() != 0o640 {
			t.Errorf("実体のモード = %v, want 0o640", tinfo.Mode().Perm())
		}
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 壊れた symlink は辿る先が無いため、#424 の方針どおり
// リンク位置に通常ファイルができる（現行はリンク先を新規作成していた）。
func TestWriteFileAtomicBrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "broken.txt")
	if err := os.Symlink("no-such-target.txt", link); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}

	if err := writeFileAtomic(link, []byte("内容"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("broken.txt が symlink のままです")
	}
	got, err := os.ReadFile(link)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "内容" {
		t.Fatalf("内容 = %q, want 内容", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "no-such-target.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("リンク先が作られています: %v", err)
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 保存先がディレクトリなら従来どおりエラーになり、
// 一時ファイルも作られない。
func TestWriteFileAtomicDirectory(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "subdir")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(dst, []byte("x"), 0o644); err == nil {
		t.Fatal("ディレクトリへの書き込みが成功を返しました")
	}
	info, err := os.Stat(dst)
	if err != nil || !info.IsDir() {
		t.Fatalf("ディレクトリが壊れています: info=%v err=%v", info, err)
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: 親フォルダが存在しない場合は ErrNotExist を含むエラーを返し、
// 何も作らない（現行の os.WriteFile と同じ失敗の仕方）。
func TestWriteFileAtomicMissingParent(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "no-such-dir", "a.txt")

	err := writeFileAtomic(dst, []byte("x"), 0o644)
	if err == nil {
		t.Fatal("存在しないフォルダへの書き込みが成功を返しました")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("エラーに ErrNotExist が含まれません: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "no-such-dir")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("フォルダが作られています: %v", err)
	}
}

// Issue #425: 読み取り専用の既存ファイルは、rename のフォルダ権限だけで
// 置き換えられてしまう後退を避けるため、従来どおりエラーになり無傷で残る。
func TestWriteFileAtomicReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("読み取り専用の確認をする Unix 向けのテスト")
	}
	if os.Geteuid() == 0 {
		t.Skip("root は読み取り専用ファイルにも書けるためスキップ")
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(dst, []byte("元の内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dst, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dst, 0o644) })

	err := writeFileAtomic(dst, []byte("新しい内容"), 0o644)
	if err == nil {
		t.Fatal("読み取り専用ファイルへの書き込みが成功を返しました")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("エラーに ErrPermission が含まれません: %v", err)
	}
	got, rerr := os.ReadFile(dst)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != "元の内容" {
		t.Fatalf("内容 = %q, want 元の内容", got)
	}
	assertNoAtomicTempLeftover(t, dir)
}

// Issue #425: /dev/full は特殊ファイルなので一時ファイルを作らず、
// 従来どおり直接書き込みの失敗 (ENOSPC) を返す。デバイスは残る。
func TestWriteFileAtomicDevFull(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full を利用する Linux 固有のテスト")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skipf("/dev/full がないためスキップ: %v", err)
	}

	err := writeFileAtomic("/dev/full", []byte("data"), 0o644)
	if err == nil {
		t.Fatal("/dev/full への書き込みが成功を返しました")
	}
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("エラーに ENOSPC が含まれません: %v", err)
	}
	info, err := os.Stat("/dev/full")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		t.Fatalf("/dev/full がキャラクタデバイスではなくなっています: %v", info.Mode())
	}
}

// Issue #425: 特殊ファイルへの symlink も辿った先へ直接書く。
// 一時ファイルは作られず、リンクもデバイスも残る。
func TestWriteFileAtomicSymlinkToDevFull(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full を利用する Linux 固有のテスト")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skipf("/dev/full がないためスキップ: %v", err)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "full")
	if err := os.Symlink("/dev/full", link); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}

	err := writeFileAtomic(link, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("/dev/full への書き込みが成功を返しました")
	}
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("エラーに ENOSPC が含まれません: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink が置き換えられています")
	}
	assertNoAtomicTempLeftover(t, dir)
}
