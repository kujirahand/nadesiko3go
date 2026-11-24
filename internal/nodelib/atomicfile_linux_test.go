//go:build linux

package nodelib

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

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
