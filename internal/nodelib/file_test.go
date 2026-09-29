package nodelib_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// runExpectError は、dirを作業フォルダとしてプログラムを実行し、
// 発生したエラーを返す。失敗することが期待されるケースに使う。
func runExpectError(t *testing.T, dir, code string) error {
	t.Helper()
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(previous) }()

	return vm.RunProgram(code, "main.nako3", host)
}

// TestMoveDirIntoOwnChildIsRejected は #186 の回帰テスト。
// ディレクトリを自身の子孫へ移動すると、コピー後の移動元削除が
// コピー先まで巻き込んで全データを消してしまう。移動は拒否され、
// 元データが残ることを確認する。
func TestMoveDirIntoOwnChildIsRejected(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{"ファイル移動", "ファイル上書移動"} {
		err := runExpectError(t, dir, `「src」を「src/newchild」に`+cmd)
		if err == nil {
			t.Fatalf("%s: 子孫への移動がエラーになりませんでした", cmd)
		}
		// 移動は拒否され、元データが残っていること
		for _, name := range []string{"file.txt", "inner"} {
			if _, e := os.Stat(filepath.Join(src, name)); e != nil {
				t.Errorf("%s: 移動元の %s が失われました: %v", cmd, name, e)
			}
		}
	}

	// 『ファイル移動時』も同じ削除順序を持つので拒否されること
	err := runExpectError(t, dir, `●移動CB
　「呼ばれた」と表示
ここまで
「src」を「src/newchild」に「移動CB」でファイル移動時`)
	if err == nil {
		t.Fatal("ファイル移動時: 子孫への移動がエラーになりませんでした")
	}
	if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
		t.Errorf("ファイル移動時: 移動元の file.txt が失われました: %v", e)
	}
}

// TestMoveFileOntoItselfIsRejected は #195 の回帰テスト。
// 移動元と移動先に同じファイルを指定すると、コピー後の元削除で実体が消える。
// 同一実体は拒否し、元ファイルが残ることを確認する。
func TestMoveFileOntoItselfIsRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 同一パスと、表記違いの同一実体（./a.txt）の両方で拒否されること
	for _, dest := range []string{"a.txt", "./a.txt"} {
		err := runExpectError(t, dir, `「a.txt」を「`+dest+`」にファイル上書移動`)
		if err == nil {
			t.Fatalf("自己への上書移動(%s)がエラーになりませんでした", dest)
		}
		if got, e := os.ReadFile(filepath.Join(dir, "a.txt")); e != nil || string(got) != "keep" {
			t.Fatalf("元ファイルが失われました: content=%q err=%v", got, e)
		}
	}
}

// TestMoveDirIntoSymlinkInsideItself は、移動先がシンボリックリンク経由で
// 移動元の内側を指す場合も拒否されることを確認する。
func TestMoveDirIntoSymlinkInsideItself(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// link → src。つまり「link/x」は解決すると「src/x」（移動元の内側）
	if err := os.Symlink(src, filepath.Join(dir, "link")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	// link2 → src/inner。移動先そのものが移動元の内側を指すリンク
	if err := os.Symlink(filepath.Join(src, "inner"), filepath.Join(dir, "link2")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	for _, dest := range []string{"link/x", "link2"} {
		err := runExpectError(t, dir, `「src」を「`+dest+`」にファイル上書移動`)
		if err == nil {
			t.Fatalf("リンク経由で内側を指す移動(%s)がエラーになりませんでした", dest)
		}
		if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
			t.Errorf("移動元の file.txt が失われました: %v", e)
		}
	}
}

// TestMoveDirIntoOwnChildDifferentCase は、大文字小文字を区別しない
// ファイルシステム（macOSの既定APFS・WindowsのNTFS）で、表記の異なる
// パス経由の子孫移動も拒否されることを確認する (#186)。
func TestMoveDirIntoOwnChildDifferentCase(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 大文字小文字を区別するFSでは「SRC」は移動元とは別の正当なパスなので、
	// このテストは区別しないFSでのみ意味を持つ
	if _, err := os.Stat(filepath.Join(dir, "SRC")); err != nil {
		t.Skip("このファイルシステムはパスの大文字小文字を区別します")
	}
	for _, code := range []string{
		`「src」を「SRC/newchild」にファイル上書移動`,
		`「SRC」を「src/sub」にファイル上書移動`,
	} {
		err := runExpectError(t, dir, code)
		if err == nil {
			t.Fatalf("大小文字違いの子孫への移動がエラーになりませんでした: %s", code)
		}
		if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
			t.Errorf("移動元の file.txt が失われました: %v", e)
		}
	}
}

// TestMoveAncestorOfCwdIsRejected は、作業フォルダが移動元の内側にあり、
// 移動先が相対パスの新規名の場合でも拒否されることを確認する (#186)。
// 相対パスの祖先走査は絶対パス化しないと「.」で止まってしまい、
// 移動元である実祖先を検出できない。
func TestMoveAncestorOfCwdIsRejected(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cwd は移動元 dir の内側 (dir/sub)。移動先の「x」は dir/sub/x なので
	// 移動元の子孫にあたる
	src := filepath.ToSlash(dir) // なでしこの文字列中のパスは / 区切りに揃える
	err := runExpectError(t, sub, `「`+src+`」を「x」にファイル上書移動`)
	if err == nil {
		t.Fatal("CWDの祖先への包含移動がエラーになりませんでした")
	}
	for _, name := range []string{"file.txt", "sub"} {
		if _, e := os.Stat(filepath.Join(dir, name)); e != nil {
			t.Errorf("移動元の %s が失われました: %v", name, e)
		}
	}
}

// TestMoveViaDotDotIsRejected は、移動先が「..」経由で移動元の内側に
// 落ちる場合も拒否されることを確認する (#186)。
func TestMoveViaDotDotIsRejected(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	deep := filepath.Join(src, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cwd は src/a/b。「../x」は src/a/x で移動元の子孫
	err := runExpectError(t, deep, `「`+filepath.ToSlash(src)+`」を「../x」にファイル上書移動`)
	if err == nil {
		t.Fatal("「..」経由で内側に落ちる移動がエラーになりませんでした")
	}
	if _, e := os.Stat(filepath.Join(src, "file.txt")); e != nil {
		t.Errorf("移動元の file.txt が失われました: %v", e)
	}
}

// TestMoveDotDirToSiblingAllowed は、作業フォルダ自身を兄弟パスへ移動する
// 正当な操作が誤って拒否されないことを確認する (#186 の偽陽性対策)。
func TestMoveDotDirToSiblingAllowed(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "B")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runIn(t, src, `「.」を「../B_moved」にファイル上書移動
「{"../B_moved/file.txt"を開く}」と表示`)
	if got != "data" {
		t.Errorf("got: %q, want %q", got, "data")
	}
}

// TestMoveToIndependentPathStillWorks は、包含関係のない通常の移動が
// 引き続き成功することを確認する。
func TestMoveToIndependentPathStillWorks(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `"src"のフォルダ作成
「data」を"src/file.txt"に保存
"src"を"dst"へファイル上書移動
「src存在: {"src"がフォルダ存在}」と表示
「dst中身: {"dst/file.txt"を開く}」と表示`)
	want := "src存在: false\ndst中身: data"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestFolderCreateWoJosi は #227 の回帰テスト。
// 『フォルダ作成』が本家と同じく「を」助詞を受理すること。
func TestFolderCreateWoJosi(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `「fdir」をフォルダ作成
「存在: {"fdir"がフォルダ存在}」と表示`)
	if got != "存在: true" {
		t.Errorf("got: %q, want %q", got, "存在: true")
	}
}

// TestTempFolderCreate は #228 の回帰テスト。
// 『DIRに一時フォルダ作成』で親フォルダを指定でき、名前のprefixが
// 本家と同じ nako- になること。省略時はOSのテンポラリを使うこと。
func TestTempFolderCreate(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `"base"のフォルダ作成
Tmp=「base」に一時フォルダ作成
「存在: {Tmpがフォルダ存在}」と表示
「親: {Tmpのパス抽出}」と表示
「名: {Tmpのファイル名抽出}」と表示`)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("got:\n%s", got)
	}
	if lines[0] != "存在: true" || lines[1] != "親: base" {
		t.Errorf("親フォルダ指定の一時フォルダ作成が想定外です: %q, %q", lines[0], lines[1])
	}
	if !strings.HasPrefix(lines[2], "名: nako-") {
		t.Errorf("一時フォルダ名が nako- で始まりません: %q", lines[2])
	}

	// 引数省略・空白指定はOSのテンポラリフォルダに作られる。
	// (引数を省略した命令は『それ』で補完されるので、プログラム先頭で
	// 『それ』が空文字列の状態で試す)
	got = runIn(t, dir, `Tmp=一時フォルダ作成
「無引数存在: {Tmpがフォルダ存在}」と表示
「無引数名: {Tmpのファイル名抽出}」と表示
Tmp2=「  」に一時フォルダ作成
「空白名: {Tmp2のファイル名抽出}」と表示`)
	lines = strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("got:\n%s", got)
	}
	if lines[0] != "無引数存在: true" {
		t.Errorf("無引数の一時フォルダ作成が想定外です: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "無引数名: nako-") {
		t.Errorf("無引数の一時フォルダ名が nako- で始まりません: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "空白名: nako-") {
		t.Errorf("空白指定の一時フォルダ名が nako- で始まりません: %q", lines[2])
	}
}

// TestCompressToolDefaultIs7z は #226 の回帰テスト。
// システム変数『圧縮解凍ツールパス』の既定値が本家と同じ 7z であること。
func TestCompressToolDefaultIs7z(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `圧縮解凍ツールパスを表示`)
	if got != "7z" {
		t.Errorf("圧縮解凍ツールパス = %q, want %q", got, "7z")
	}
}
