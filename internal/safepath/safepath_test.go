package safepath

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeZipReader はテスト用にZIPエントリから zip.Reader を作る。
func makeZipReader(t *testing.T, entries map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestExtractBasic(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	r := makeZipReader(t, map[string]string{
		"a.txt":     "hello",
		"sub/b.txt": "world",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 0 {
		t.Errorf("拒否されたエントリがある: %d件", result.Rejected())
	}
	for name, want := range map[string]string{
		"a.txt":        "hello",
		"sub/b.txt":    "world",
	} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Errorf("%s が読めない: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestExtractZipSlip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	r := makeZipReader(t, map[string]string{
		"../evil.txt":         "evil",
		"sub/../../evil2.txt": "evil2",
		"ok/good.txt":         "good",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Escaped) != 2 {
		t.Errorf("Escaped = %v, want 2件", result.Escaped)
	}
	if len(result.ViaSymlink) != 0 {
		t.Errorf("ViaSymlink = %v, want 0件", result.ViaSymlink)
	}
	// 安全なエントリは展開されている
	if _, err := os.Stat(filepath.Join(dest, "ok", "good.txt")); err != nil {
		t.Errorf("通常エントリが展開されていない: %v", err)
	}
	// 脱出エントリは展開されていない
	for _, name := range []string{"evil.txt", "evil2.txt"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("展開先の外に書き込まれた: %s", name)
		}
	}
}

func TestExtractSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// 展開先の外を指す相対シンボリックリンク
	if err := os.Symlink("../outside", filepath.Join(dest, "link")); err != nil {
		t.Fatal(err)
	}
	r := makeZipReader(t, map[string]string{
		"link/pwn.txt": "pwn",
		"ok.txt":       "ok",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ViaSymlink) != 1 {
		t.Errorf("ViaSymlink = %v, want 1件", result.ViaSymlink)
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn.txt")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外に書き込まれた")
	}
	if body, err := os.ReadFile(filepath.Join(dest, "ok.txt")); err != nil || string(body) != "ok" {
		t.Errorf("通常エントリが展開されていない: %q err=%v", body, err)
	}
}

// 展開先内側を指す相対シンボリックリンクは透過的に辿られる（Issue #261 の改善）。
func TestExtractInsideSymlinkFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(dest, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dest/inlink -> inside （内側を指す相対リンク）
	if err := os.Symlink("inside", filepath.Join(dest, "inlink")); err != nil {
		t.Fatal(err)
	}
	r := makeZipReader(t, map[string]string{
		"inlink/a.txt": "hello",
		"ok.txt":       "ok",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 0 {
		t.Errorf("内側を指すリンクが拒否された: %v", result)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "inside", "a.txt")); err != nil || string(body) != "hello" {
		t.Errorf("リンク経由で展開されなかった: %q err=%v", body, err)
	}
}

// TestExtractNestedInsideSymlinkFollowed は、サブディレクトリ内のリンクが ".." を使って
// 内側を指すケースが正しく展開されることを確認する（PR #272 レビュー指摘）。
//
// 例: sub/link -> ../inside （sub/ から見て ../inside は dest/inside を指す、安全）
func TestExtractNestedInsideSymlinkFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(dest, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dest/sub/link -> ../inside （sub/ から見て ../inside は dest/inside を指す）
	if err := os.Symlink("../inside", filepath.Join(dest, "sub", "link")); err != nil {
		t.Fatal(err)
	}
	r := makeZipReader(t, map[string]string{
		"sub/link/a.txt": "hello",
		"ok.txt":         "ok",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 0 {
		t.Errorf("内側を指すネストされたリンクが拒否された: %v", result)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "inside", "a.txt")); err != nil || string(body) != "hello" {
		t.Errorf("リンク経由で展開されなかった: %q err=%v", body, err)
	}
}

// TestExtractNestedSymlinkEscape は、サブディレクトリ内のリンクが本当に脱出するケースを
// 拒否することを確認する（PR #272 レビュー指摘の回帰テスト）。
//
// 例: sub/link -> ../../outside （sub/ から見て ../../outside は dest の外を指す、脱出）
func TestExtractNestedSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dest/sub/link -> ../../outside （sub/ から見て ../../outside は dest の外を指す）
	if err := os.Symlink("../../outside", filepath.Join(dest, "sub", "link")); err != nil {
		t.Fatal(err)
	}
	r := makeZipReader(t, map[string]string{
		"sub/link/pwn.txt": "pwn",
		"ok.txt":           "ok",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ViaSymlink) != 1 {
		t.Errorf("ViaSymlink = %v, want 1件", result.ViaSymlink)
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn.txt")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外に書き込まれた")
	}
	if body, err := os.ReadFile(filepath.Join(dest, "ok.txt")); err != nil || string(body) != "ok" {
		t.Errorf("通常エントリが展開されていない: %q err=%v", body, err)
	}
}

func TestExtractStripTopDir(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	r := makeZipReader(t, map[string]string{
		"topdir/go.mod":      "module test\n",
		"topdir/pkg/main.go": "package main\n",
	})

	result, err := Extract(r, dest, ExtractOptions{StripTopDir: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 0 {
		t.Errorf("拒否されたエントリがある: %d件", result.Rejected())
	}
	for _, rel := range []string{"go.mod", "pkg/main.go"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("%s が展開されていない: %v", rel, err)
		}
	}
}

// setuid/sticky ビットがファイルモードから落とされることを確認する（#261 項目3）。
func TestExtractPermOnlyMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windowsにはsetuid/stickyビットがないため")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")

	// setuidビット付きのエントリを作る
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{
		Name:               "setuid_file.txt",
		Method:             zip.Deflate,
		ModifiedTime:       0,
		ModifiedDate:       0,
	}
	// setuid (0o4000) + 通常パーミッション (0o755) = 0o4755
	header.SetMode(0o4755)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("content")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 0 {
		t.Errorf("拒否されたエントリがある: %d件", result.Rejected())
	}

	fi, err := os.Stat(filepath.Join(dest, "setuid_file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Perm() で取ったパーミッションに setuid が含まれていないことを確認
	mode := fi.Mode().Perm()
	if mode&0o4000 != 0 {
		t.Errorf("setuidビットが残っている: %o", mode)
	}
	if mode&0o777 != 0o755 {
		t.Errorf("パーミッションが違う: %o, want 0o755", mode)
	}
}

func TestExtractAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	r := makeZipReader(t, map[string]string{
		"/tmp/evil.txt": "evil",
		"ok.txt":        "ok",
	})

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Escaped) != 1 {
		t.Errorf("Escaped = %v, want 1件", result.Escaped)
	}
}

func TestStripTopDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"a/b/c", "b/c"},
		{"a/", ""},
		{"a", ""},
		{"a/b/", "b/"},
	}
	for _, tt := range tests {
		got := stripTopDir(tt.input)
		if got != tt.want {
			t.Errorf("stripTopDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// 大量の不正エントリでも ExtractResult に正しく分類されることを確認。
func TestExtractManyRejected(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	entries := make(map[string]string)
	for i := 0; i < 10; i++ {
		entries[filepath.Join("..", "evil"+strings.Repeat("x", i)+".txt")] = "evil"
	}
	entries["ok.txt"] = "ok"
	r := makeZipReader(t, entries)

	result, err := Extract(r, dest, ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected() != 10 {
		t.Errorf("Rejected = %d, want 10", result.Rejected())
	}
	if len(result.Escaped) != 10 {
		t.Errorf("Escaped = %d, want 10", len(result.Escaped))
	}
}

// TestExtractENOTDIRNotClassifiedAsSymlink は、通常ファイルのサブパスを作成しようとした場合
// （例: ファイル "a" に対して "a/child.txt"）に、ENOTDIR エラーがシンボリックリンク脱出として
// 分類されず、通常のエラーとして返されることを確認する（PR #272 レビュー指摘）。
func TestExtractENOTDIRNotClassifiedAsSymlink(t *testing.T) {
	destDir := t.TempDir()

	// 先に通常ファイル "a" を作成
	err := os.WriteFile(filepath.Join(destDir, "a"), []byte("file a"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// ZIP には "a/child.txt" を含める（ファイル "a" の下にディレクトリを作ろうとする）
	r := makeZipReader(t, map[string]string{
		"a/child.txt": "child",
	})

	result, err := Extract(r, destDir, ExtractOptions{})

	// エラーが返されることを確認
	if err == nil {
		t.Fatal("ENOTDIR/EEXIST エラーが返されなかった")
	}
	// エラーメッセージはプラットフォーム依存なので、具体的な文言はチェックしない
	// 重要なのは、ViaSymlink に分類されないこと

	// result が nil の場合、ViaSymlink/Escaped は空とみなせる
	if result != nil {
		// シンボリックリンク脱出ではないので、ViaSymlink には分類されない
		if len(result.ViaSymlink) != 0 {
			t.Errorf("ENOTDIR/EEXIST が ViaSymlink に分類された: %v", result.ViaSymlink)
		}
		if len(result.Escaped) != 0 {
			t.Errorf("パス脱出でもないのに Escaped に分類された: %v", result.Escaped)
		}
	}
}

// TestSymlinkTargetEscapes は symlinkTargetEscapes 関数のユニットテスト。
// リンクの位置を基準にターゲットを解決して、root から出るかどうかを判定することを確認する。
func TestSymlinkTargetEscapes(t *testing.T) {
	tests := []struct {
		name     string
		linkPath string
		target   string
		want     bool
	}{
		// 絶対パスは常に脱出
		{"absolute path", "link", "/tmp/evil", true},
		{"absolute path in sub", "sub/link", "/tmp/evil", true},

		// ルート内を指す相対パスは安全
		{"relative inside from root", "link", "inside", false},
		{"relative inside with ..", "sub/link", "../inside", false}, // sub/../inside = inside
		{"relative inside complex", "a/b/link", "../../inside", false}, // a/b/../../inside = inside

		// ルート外を指す相対パスは脱出
		{"relative escape from root", "link", "../outside", true},
		{"relative escape from sub", "sub/link", "../../outside", true}, // sub/../../outside = ../outside
		{"relative escape complex", "a/b/link", "../../../outside", true}, // a/b/../../../outside = ../../outside

		// ターゲットが ".." 自体
		{"target is ..", "link", "..", true},
		{"target is .. from sub", "sub/link", "../..", true},

		// 特殊ケース: ターゲットが "."
		{"target is .", "link", ".", false},
		{"target is ./inside", "link", "./inside", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := symlinkTargetEscapes(tt.linkPath, tt.target)
			if got != tt.want {
				t.Errorf("symlinkTargetEscapes(%q, %q) = %v, want %v", tt.linkPath, tt.target, got, tt.want)
			}
		})
	}
}
