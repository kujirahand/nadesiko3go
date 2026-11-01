package nodelib_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupAliasIntoSource は N01 の再現入力を組み立てる。
// src/inner と src/keep.txt があり、alias は src/inner を指す。
// alias/new のように未作成の末尾を付けると、移動先の実体は移動元の内側になる。
func setupAliasIntoSource(t *testing.T) (dir, src string) {
	t.Helper()
	dir = t.TempDir()
	src = filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 空フォルダは移動で作られないため、内側の内容を確認できる入れ子のファイルを置く。
	if err := os.WriteFile(filepath.Join(src, "inner", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "inner"), filepath.Join(dir, "alias")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	return dir, src
}

// assertSourceKept は、拒否された移動が原本を残し、実体のコピーを
// 移動元の内側に作っていないことを確かめる。
func assertSourceKept(t *testing.T, src string) {
	t.Helper()
	if got, err := os.ReadFile(filepath.Join(src, "keep.txt")); err != nil || string(got) != "keep" {
		t.Errorf("移動元の keep.txt が失われました: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(src, "inner", "deep.txt")); err != nil || string(got) != "deep" {
		t.Errorf("移動元の inner/deep.txt が失われました: content=%q err=%v", got, err)
	}
	// コピーが移動元の内側に作られていると、コピー後の os.RemoveAll(src) が
	// 原本とコピーの両方を削除してしまう。作られていないことを確かめる。
	if _, err := os.Stat(filepath.Join(src, "inner", "new")); err == nil {
		t.Error("移動先の実体が移動元の内側に作られました: src/inner/new")
	}
}

// TestN01MoveIntoAncestorSymlinkWithMissingTailIsRejected は N01 の回帰テスト。
// 移動先の祖先だけがシンボリックリンクで末尾が未作成の場合、実体パスは
// 移動元の内側になる。移動を開始前に拒否し、原本を保持することを確認する。
func TestN01MoveIntoAncestorSymlinkWithMissingTailIsRejected(t *testing.T) {
	for _, cmd := range []string{"ファイル移動", "ファイル上書移動"} {
		t.Run(cmd, func(t *testing.T) {
			dir, src := setupAliasIntoSource(t)
			err := runExpectError(t, dir, `「src」を「alias/new」に`+cmd)
			if err == nil {
				t.Fatalf("%s: シンボリックリンク経由の内側移動がエラーになりませんでした", cmd)
			}
			if !strings.Contains(err.Error(), "移動先は移動元の内側です") {
				t.Errorf("%s: エラー文面 = %q", cmd, err.Error())
			}
			assertSourceKept(t, src)
		})
	}
}

// TestN01MoveIntoAbsoluteAncestorSymlinkIsRejected は、移動先を絶対パスで
// 指定した場合も同じ判定になり、原本が残ることを確認する。
func TestN01MoveIntoAbsoluteAncestorSymlinkIsRejected(t *testing.T) {
	dir, src := setupAliasIntoSource(t)
	dest := filepath.ToSlash(filepath.Join(dir, "alias", "new"))
	code := fmt.Sprintf(`「%s」を「%s」にファイル上書移動`, filepath.ToSlash(src), dest)
	err := runExpectError(t, dir, code)
	if err == nil {
		t.Fatal("絶対パス指定のシンボリックリンク経由の内側移動がエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "移動先は移動元の内側です") {
		t.Errorf("エラー文面 = %q", err.Error())
	}
	assertSourceKept(t, src)
}

// TestN01MoveIntoSymlinkedExistingAliasIsRejected は、移動先のシンボリック
// リンク先が実在する場合（alias → src/inner そのもの）も拒否されることを
// 確認する。既存の内部移動拒否を維持するケース。
func TestN01MoveIntoSymlinkedExistingAliasIsRejected(t *testing.T) {
	dir, src := setupAliasIntoSource(t)
	err := runExpectError(t, dir, `「src」を「alias」にファイル上書移動`)
	if err == nil {
		t.Fatal("シンボリックリンクのリンク先への移動がエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "移動先は移動元の内側です") {
		t.Errorf("エラー文面 = %q", err.Error())
	}
	assertSourceKept(t, src)
}

// TestN01MoveToDirectoryOutsideSourceKeepsContent は、移動元の外にある
// 独立したフォルダへの移動（日本語・空白を含むパス、相対パス、./ 付き）が
// 引き続き成功し、内容を保つことを確認する。
func TestN01MoveToDirectoryOutsideSourceKeepsContent(t *testing.T) {
	for _, dest := range []string{"moved", "./moved2", "日本語 フォルダ/中身", "空白 フォルダ/中身"} {
		t.Run(dest, func(t *testing.T) {
			dir, src := setupAliasIntoSource(t)
			runIn(t, dir, fmt.Sprintf(`「%s」を「%s」にファイル上書移動`, filepath.ToSlash(src), dest))

			if _, err := os.Stat(src); err == nil {
				t.Errorf("移動元が残っています: %s", src)
			}
			moved := filepath.Join(dir, filepath.FromSlash(dest))
			if got, err := os.ReadFile(filepath.Join(moved, "keep.txt")); err != nil || string(got) != "keep" {
				t.Errorf("移動先の keep.txt が壊れています: content=%q err=%v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(moved, "inner", "deep.txt")); err != nil || string(got) != "deep" {
				t.Errorf("移動先の inner/deep.txt が壊れています: content=%q err=%v", got, err)
			}
			if _, err := os.Lstat(filepath.Join(dir, "alias")); err != nil {
				t.Errorf("移動元の外にある alias が失われました: %v", err)
			}
		})
	}
}

// TestN01BrokenSymlinkDestReportsError は、壊れたシンボリックリンクを
// 移動先の祖先に指定した場合に、黙って成功せずエラーを報告し、
// 原本を保持することを確認する。
func TestN01BrokenSymlinkDestReportsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "inner", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}

	err := runExpectError(t, dir, `「src」を「broken/new」にファイル上書移動`)
	if err == nil {
		t.Fatal("壊れたシンボリックリンクへの移動がエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "作成できません") || !strings.Contains(err.Error(), "broken") {
		t.Errorf("エラー文面 = %q", err.Error())
	}
	assertSourceKept(t, src)
}
