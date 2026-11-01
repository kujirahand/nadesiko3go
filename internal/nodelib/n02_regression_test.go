package nodelib_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// N02 の再現入力。空ディレクトリの階層・通常ファイル・symlink(壊れたものを含む)を
// 持つディレクトリを作る。
func setupMoveEntries(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "src")
	for _, d := range []string{"empty/nested", "files"} {
		if err := os.MkdirAll(filepath.Join(src, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "files", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 通常の symlink と、リンク先が存在しない壊れた symlink
	if err := os.Symlink("file.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	if err := os.Symlink("nowhere", filepath.Join(src, "broken")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	return src
}

func moveSrcToDst(t *testing.T, dir, src, dst string) error {
	t.Helper()
	code := fmt.Sprintf(`「%s」を「%s」にファイル移動`, filepath.ToSlash(src), filepath.ToSlash(dst))
	return runExpectError(t, dir, code)
}

// TestN02MoveKeepsEmptyDirectoryTree は N02 の回帰テスト。
// 空ディレクトリの階層が移動先に保たれることを確認する。
func TestN02MoveKeepsEmptyDirectoryTree(t *testing.T) {
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	dst := filepath.Join(dir, "dst")

	if err := moveSrcToDst(t, dir, src, dst); err != nil {
		t.Fatalf("移動がエラーになりました: %v", err)
	}
	for _, rel := range []string{"empty", "empty/nested", "files"} {
		info, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil || !info.IsDir() {
			t.Errorf("移動先のディレクトリ %s が失われました: info=%v err=%v", rel, info, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(dst, "file.txt")); err != nil || string(got) != "hello" {
		t.Errorf("移動先の file.txt が壊れています: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "files", "deep.txt")); err != nil || string(got) != "deep" {
		t.Errorf("移動先の files/deep.txt が壊れています: content=%q err=%v", got, err)
	}
	if _, err := os.Stat(src); err == nil {
		t.Errorf("移動元が残っています: %s", src)
	}
}

// TestN02MoveKeepsSymlinks は、通常の symlink と壊れた symlink を
// リンクとして保存し、リンク先の文字列を変えないことを確認する。
func TestN02MoveKeepsSymlinks(t *testing.T) {
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	dst := filepath.Join(dir, "dst")

	if err := moveSrcToDst(t, dir, src, dst); err != nil {
		t.Fatalf("移動がエラーになりました: %v", err)
	}
	for _, tc := range []struct{ name, target string }{
		{"link.txt", "file.txt"},
		{"broken", "nowhere"},
	} {
		path := filepath.Join(dst, tc.name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("移動先の symlink %s が失われました: %v", tc.name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("移動先の %s が symlink ではありません: mode=%v", tc.name, info.Mode())
			continue
		}
		if target, err := os.Readlink(path); err != nil || target != tc.target {
			t.Errorf("移動先の %s のリンク先 = %q (err=%v), want %q", tc.name, target, err, tc.target)
		}
	}
}

// TestN02MoveEnumerationErrorKeepsSource は、列挙できないディレクトリがあると
// 成功を返さず、移動元を削除しないことを確認する。
func TestN02MoveEnumerationErrorKeepsSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ではパーミッションで列挙を失敗させられません")
	}
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	secret := filepath.Join(src, "secret")
	if err := os.MkdirAll(secret, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o755) })
	dst := filepath.Join(dir, "dst")

	err := moveSrcToDst(t, dir, src, dst)
	if err == nil {
		t.Fatal("列挙できないディレクトリがあるのに移動が成功しました")
	}
	if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
		t.Errorf("列挙に失敗したのに移動元が削除されました: %v", e)
	}
}

// TestN02MoveCopyErrorKeepsSource は、コピーに失敗した場合に移動元を
// 削除しないことを確認する。移動先の親を書き込み不可にする。
func TestN02MoveCopyErrorKeepsSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ではパーミッションでコピーを失敗させられません")
	}
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	readonly := filepath.Join(dir, "ro")
	if err := os.MkdirAll(readonly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readonly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })

	err := moveSrcToDst(t, dir, src, filepath.Join(readonly, "dst"))
	if err == nil {
		t.Fatal("コピーできない移動先なのに成功しました")
	}
	if !strings.Contains(err.Error(), "作成できません") {
		t.Errorf("エラー文面 = %q", err.Error())
	}
	if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
		t.Errorf("コピーに失敗したのに移動元が削除されました: %v", e)
	}
}

// TestN02MoveRemoveErrorIsReported は、移動元の削除に失敗した場合に
// 成功を返さないことを確認する。移動元の親を書き込み不可にする。
func TestN02MoveRemoveErrorIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ではパーミッションで削除を失敗させられません")
	}
	dir := t.TempDir()
	readonly := filepath.Join(dir, "ro")
	if err := os.MkdirAll(readonly, 0o755); err != nil {
		t.Fatal(err)
	}
	src := setupMoveEntries(t, readonly)
	if err := os.Chmod(readonly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })
	dst := filepath.Join(dir, "dst")

	err := moveSrcToDst(t, dir, src, dst)
	if err == nil {
		t.Fatal("移動元を削除できないのに成功しました")
	}
	if !strings.Contains(err.Error(), "削除できません") {
		t.Errorf("エラー文面 = %q", err.Error())
	}
	if _, e := os.Stat(dst); e != nil {
		t.Errorf("移動先が作られていません: %v", e)
	}
}

// TestN02MoveWithProgressKeepsEntries は、進捗コールバック付きの移動でも
// 空ディレクトリと symlink を保つことを確認する。件数は内容を持つ entry
// (通常ファイルと symlink)だけを数え、既存の意味を変えない。
func TestN02MoveWithProgressKeepsEntries(t *testing.T) {
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	dst := filepath.Join(dir, "dst")

	got := runIn(t, dir, fmt.Sprintf(`CNT=0
●進捗CB
　CNT = CNT + 1
ここまで
「進捗CB」をファイル処理時
「%s」を「%s」にファイル移動
「件数: {CNT}」と表示`, filepath.ToSlash(src), filepath.ToSlash(dst)))
	if got != "件数: 4" {
		t.Errorf("進捗回数 = %q, want %q", got, "件数: 4")
	}
	if info, err := os.Stat(filepath.Join(dst, "empty", "nested")); err != nil || !info.IsDir() {
		t.Errorf("進捗付き移動で空ディレクトリが失われました: info=%v err=%v", info, err)
	}
}

// TestN02MoveCancelKeepsSource は、途中で強制停止したときに移動元の内容が
// 残ることを確認する。停止後の削除は行わない既存の扱いを固定する。
func TestN02MoveCancelKeepsSource(t *testing.T) {
	dir := t.TempDir()
	src := setupMoveEntries(t, dir)
	dst := filepath.Join(dir, "dst")

	got := runIn(t, dir, fmt.Sprintf(`●停止CB
　ファイル処理強制停止
ここまで
「停止CB」をファイル処理時
「%s」を「%s」にファイル移動
「完了」と表示`, filepath.ToSlash(src), filepath.ToSlash(dst)))
	if got != "完了" {
		t.Errorf("出力 = %q, want %q", got, "完了")
	}
	for _, rel := range []string{"file.txt", "files/deep.txt"} {
		if _, err := os.Stat(filepath.Join(src, filepath.FromSlash(rel))); err != nil {
			t.Errorf("停止したのに移動元の %s が失われました: %v", rel, err)
		}
	}
}

// BenchmarkN02MoveDirectory は perf 用。空ディレクトリ 10 個と通常ファイル
// 100 個を持つディレクトリの移動を、VM を含む実際の命令経路で測る。
// checkMoveTarget は非公開のため、パッケージ外テストからは実入口を通して測る。
func BenchmarkN02MoveDirectory(b *testing.B) {
	root := b.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	code := fmt.Sprintf("「%s」を「%s」にファイル上書移動", filepath.ToSlash(src), filepath.ToSlash(dst))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := buildMoveTree(src); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		host := vm.NewCUIHost(io.Discard, strings.NewReader(""), nil)
		if err := vm.RunProgram(code, "bench.nako3", host); err != nil {
			b.Fatalf("move failed: %v", err)
		}
	}
}

// buildMoveTree は空ディレクトリ 10 個と通常ファイル 100 個の木を作り直す。
func buildMoveTree(src string) error {
	if err := os.RemoveAll(src); err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		if err := os.MkdirAll(filepath.Join(src, "empty", fmt.Sprintf("e%d", i)), 0o755); err != nil {
			return err
		}
	}
	data := make([]byte, 64)
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("f%03d.txt", i)), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
