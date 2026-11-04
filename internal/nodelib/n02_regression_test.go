package nodelib_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// skipIfNoPermissionEnforcement は chmod で権限エラーを作れない環境を飛ばす。
// Windows は os.Geteuid() が -1 を返し、0o000 や 0o555 でも列挙・作成・削除を
// 制限しないため、root と同様にスキップする。
func skipIfNoPermissionEnforcement(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("パーミッションで失敗を再現できない環境です")
	}
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
	skipIfNoPermissionEnforcement(t)
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
	skipIfNoPermissionEnforcement(t)
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
	skipIfNoPermissionEnforcement(t)
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

// TestN02MoveSymlinkedSourceKeepsContent は、移動元がディレクトリへの
// symlink でも実体の中身を列挙して移動先へ移すことを確認する。
// filepath.WalkDir は root の symlink も辿らないため、実体へ解決してから
// 列挙する必要がある。
func TestN02MoveSymlinkedSourceKeepsContent(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(real, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	dst := filepath.Join(dir, "dst")

	if err := moveSrcToDst(t, dir, link, dst); err != nil {
		t.Fatalf("移動がエラーになりました: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "file.txt")); err != nil || string(got) != "hello" {
		t.Errorf("移動先の file.txt が壊れています: content=%q err=%v", got, err)
	}
	if info, err := os.Stat(filepath.Join(dst, "inner")); err != nil || !info.IsDir() {
		t.Errorf("移動先の inner が失われました: info=%v err=%v", info, err)
	}
	if _, err := os.Lstat(link); err == nil {
		t.Errorf("移動元の symlink が残っています: %s", link)
	}
	if _, err := os.Stat(real); err != nil {
		t.Errorf("symlink の実体が失われました: %v", err)
	}
}

// TestN02MoveFromCurrentDirectoryLeavesEmptyDirectory は、移動元がカレント
// ディレクトリ自身（"."）の場合を固定する。カレントディレクトリ自身は
// 削除できないため、removeSource は "." を指すときだけ中身の削除へ切り替え、
// 元の空ディレクトリを残す。この分岐が無いと、コピーは済んでも移動全体が
// 「削除できません」で失敗する。symlink も権限も使わないので、どのOSでも
// スキップせずに実行できる。
func TestN02MoveFromCurrentDirectoryLeavesEmptyDirectory(t *testing.T) {
	for _, cmd := range []string{"ファイル移動", "ファイル上書移動"} {
		t.Run(cmd, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "work")
			if err := os.MkdirAll(filepath.Join(src, "inner"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(src, "inner", "deep.txt"), []byte("deep"), 0o644); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(root, "dst")

			code := fmt.Sprintf("「.」を「%s」に%s", filepath.ToSlash(dst), cmd)
			if err := runExpectError(t, src, code); err != nil {
				t.Fatalf("%s: カレントディレクトリからの移動がエラーになりました: %v", cmd, err)
			}
			if got, err := os.ReadFile(filepath.Join(dst, "file.txt")); err != nil || string(got) != "hello" {
				t.Errorf("移動先の file.txt が壊れています: content=%q err=%v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(dst, "inner", "deep.txt")); err != nil || string(got) != "deep" {
				t.Errorf("移動先の inner/deep.txt が壊れています: content=%q err=%v", got, err)
			}
			info, err := os.Stat(src)
			if err != nil || !info.IsDir() {
				t.Fatalf("移動元のカレントディレクトリが失われました: info=%v err=%v", info, err)
			}
			entries, err := os.ReadDir(src)
			if err != nil {
				t.Fatalf("移動元を読めません: %v", err)
			}
			if len(entries) != 0 {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				t.Errorf("移動元の中身が残っています: %v", names)
			}
		})
	}
}

// TestN02MoveSymlinkToCurrentDirectoryKeepsTarget は、移動元がカレント
// ディレクトリを指す symlink（alias -> .）のときを固定する。
// os.Stat は symlink を解決するため、移動元自身が symlink でも
// カレントディレクトリ特例に入ってしまい、リンク先（カレントディレクトリ）
// の中身を削除していた。symlink はリンクとして削除し、リンク先には
// 触れないことを確認する。
// 「zalias」は中身より後に並ぶ名前で、リンク自身が先頭に来る「alias」では
// 特例が先にリンクを消して以後の削除が不発になり、旧実装の破壊を再現できない。
// 報告された再現（alias -> .）と、実際に中身が消える並びの両方を固定する。
// 末尾スラッシュ付きの「alias/」は os.Lstat でもリンク先のディレクトリへ
// 解決されるため、filepath.Clean での正規化も同じ表で固定する。
func TestN02MoveSymlinkToCurrentDirectoryKeepsTarget(t *testing.T) {
	for _, linkName := range []string{"alias", "zalias"} {
		for _, trailingSlash := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-trailingSlash=%v", linkName, trailingSlash), func(t *testing.T) {
				work := filepath.Join(t.TempDir(), "work")
				if err := os.MkdirAll(filepath.Join(work, "inner"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(work, "file.txt"), []byte("hello"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(work, "inner", "deep.txt"), []byte("deep"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(".", filepath.Join(work, linkName)); err != nil {
					t.Skipf("シンボリックリンクを作れません: %v", err)
				}
				// 移動先はカレントディレクトリの外に置く（内側だと checkMoveTarget が拒否する）
				dst := filepath.Join(t.TempDir(), "dst")
				srcArg := linkName
				if trailingSlash {
					srcArg += "/"
				}

				code := fmt.Sprintf(`「%s」を「%s」にファイル移動`, srcArg, filepath.ToSlash(dst))
				if err := runExpectError(t, work, code); err != nil {
					t.Fatalf("symlink の移動がエラーになりました: %v", err)
				}

				// リンク先（カレントディレクトリ）の中身が残っていること
				for _, rel := range []string{"file.txt", "inner/deep.txt"} {
					if _, err := os.Stat(filepath.Join(work, filepath.FromSlash(rel))); err != nil {
						t.Errorf("リンク先の %s が失われました: %v", rel, err)
					}
				}
				// 移動元の symlink 自体はリンクとして削除されていること
				if _, err := os.Lstat(filepath.Join(work, linkName)); err == nil {
					t.Errorf("移動元の symlink が残っています: %s", filepath.Join(work, linkName))
				}
				// リンク先の中身を移動先へ移す既存の扱いは変えない
				if got, err := os.ReadFile(filepath.Join(dst, "file.txt")); err != nil || string(got) != "hello" {
					t.Errorf("移動先の file.txt が壊れています: content=%q err=%v", got, err)
				}
			})
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

// TestN02MoveDotDotAfterSymlinkKeepsUnrelatedEntries は、移動元が
// 「symlink の後ろに .. を付けた表記」の場合を固定する。
// コピー側は OS のパス解決で alias/.. = リンク先の親を指す一方、
// 削除側が filepath.Clean で字句的に畳み込むと "." や "x" になり、
// カレントディレクトリやその無関係なエントリーを削除してしまう。
// コピーした実体（リンク先の親側）だけが削除され、カレントディレクトリ
// 側が無傷であることを確認する。
func TestN02MoveDotDotAfterSymlinkKeepsUnrelatedEntries(t *testing.T) {
	t.Run("alias/..", func(t *testing.T) {
		root := t.TempDir()
		work := filepath.Join(root, "work")
		other := filepath.Join(root, "other")
		sub := filepath.Join(other, "sub")
		if err := os.MkdirAll(work, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, "keep.txt"), []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, "data.txt"), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(work, "alias")
		if err := os.Symlink(sub, alias); err != nil {
			t.Skipf("シンボリックリンクを作れません: %v", err)
		}
		dst := filepath.Join(root, "dst")

		code := fmt.Sprintf(`「alias/..」を「%s」にファイル移動`, filepath.ToSlash(dst))
		if err := runExpectError(t, work, code); err != nil {
			t.Fatalf("移動がエラーになりました: %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(dst, "data.txt")); err != nil || string(got) != "data" {
			t.Errorf("移動先の data.txt が壊れています: content=%q err=%v", got, err)
		}
		if _, err := os.Stat(filepath.Join(work, "keep.txt")); err != nil {
			t.Errorf("カレントディレクトリの keep.txt が失われました: %v", err)
		}
		if _, err := os.Lstat(alias); err != nil {
			t.Errorf("カレントディレクトリの alias が失われました: %v", err)
		}
		if _, err := os.Stat(other); !os.IsNotExist(err) {
			t.Errorf("コピー元の other が削除されていません: err=%v", err)
		}
	})

	t.Run("alias/../x", func(t *testing.T) {
		root := t.TempDir()
		work := filepath.Join(root, "work")
		other := filepath.Join(root, "other")
		sub := filepath.Join(other, "sub")
		otherX := filepath.Join(other, "x")
		if err := os.MkdirAll(filepath.Join(work, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(otherX, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, "x", "keepx.txt"), []byte("keepx"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(otherX, "moved.txt"), []byte("moved"), 0o644); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(work, "alias")
		if err := os.Symlink(sub, alias); err != nil {
			t.Skipf("シンボリックリンクを作れません: %v", err)
		}
		dst := filepath.Join(root, "dst")

		code := fmt.Sprintf(`「alias/../x」を「%s」にファイル移動`, filepath.ToSlash(dst))
		if err := runExpectError(t, work, code); err != nil {
			t.Fatalf("移動がエラーになりました: %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(dst, "moved.txt")); err != nil || string(got) != "moved" {
			t.Errorf("移動先の moved.txt が壊れています: content=%q err=%v", got, err)
		}
		if _, err := os.Stat(filepath.Join(work, "x", "keepx.txt")); err != nil {
			t.Errorf("カレントディレクトリ側の x/keepx.txt が失われました: %v", err)
		}
		if _, err := os.Lstat(alias); err != nil {
			t.Errorf("カレントディレクトリの alias が失われました: %v", err)
		}
		if _, err := os.Stat(otherX); !os.IsNotExist(err) {
			t.Errorf("コピー元の other/x が削除されていません: err=%v", err)
		}
	})
}
