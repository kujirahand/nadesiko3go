package nodelib

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeTestZip はテスト用に任意のエントリ名を持つZIPを作る。
func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
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
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// Issue #194: 宛先を省略（既定値"."）しても全エントリを展開する。
func TestExtractZipDestDot(t *testing.T) {
	dir := t.TempDir()
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"a.txt":     "hello",
		"sub/b.txt": "world",
	})
	t.Chdir(dir)

	if err := extractZip("f.zip", "."); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s が展開されていません: %v", name, err)
		}
	}
}

// Zip Slip対策が維持されていること（".."を含むエントリは拒否し、
// 安全なエントリだけを展開した上でエラーを返す）。
func TestExtractZipZipSlip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"../evil.txt":         "evil",
		"sub/../../evil2.txt": "evil2",
		"ok/good.txt":         "good",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err == nil {
		t.Error("範囲外エントリを含むのにエラーになりませんでした")
	}
	for _, name := range []string{"evil.txt", "evil2.txt"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("展開先の外に書き込まれました: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "ok", "good.txt")); err != nil {
		t.Errorf("通常エントリが展開されていません: %v", err)
	}
}

// Issue #185: 展開先の既存シンボリックリンク経由で外へ書き込ませない。
func TestExtractZipSymlinkEscape(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 展開先の外を指す既存リンクと、既存ファイルを指すリンクを置く
	if err := os.Symlink(outside, filepath.Join(dest, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "keep.txt"), filepath.Join(dest, "victim.txt")); err != nil {
		t.Fatal(err)
	}
	// 存在しないファイルを指す壊れたリンク（dangling）も置く。
	// StatではなくLstatで検査しているので、リンク先が無くても拒否できる。
	if err := os.Symlink(filepath.Join(outside, "nonexistent.txt"), filepath.Join(dest, "broken.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"link/pwn.txt": "pwn",
		"link/sub/":    "", // ディレクトリエントリが既存リンクに当たるケース
		"victim.txt":   "overwritten",
		"broken.txt":   "dangling",
		"ok.txt":       "ok",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err == nil {
		t.Error("リンク経由エントリを含むのにエラーになりませんでした")
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn.txt")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外に書き込まれました")
	}
	if _, err := os.Lstat(filepath.Join(outside, "sub")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外にフォルダが作られました")
	}
	if _, err := os.Lstat(filepath.Join(outside, "nonexistent.txt")); !os.IsNotExist(err) {
		t.Error("壊れたリンクの先に書き込まれました")
	}
	if body, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(body) != "keep" {
		t.Errorf("リンク先の既存ファイルが上書きされました: %q err=%v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "ok.txt")); err != nil || string(body) != "ok" {
		t.Errorf("通常エントリが展開されていません: %q err=%v", body, err)
	}
}

// 展開先自身がシンボリックリンクでも、リンク先の内側へ正しく展開する。
func TestExtractZipDestSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "destlink")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{"a.txt": "hello"})

	if err := extractZip(filepath.Join(dir, "f.zip"), link); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(real, "a.txt")); err != nil || string(body) != "hello" {
		t.Errorf("リンク先へ展開されていません: %q err=%v", body, err)
	}
}

// Issue #189: 圧縮の入出力が同じ実体なら拒否し、元データを残す。
func TestCreateZipSameFile(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("zip payload")
	src := filepath.Join(dir, "f.zip")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := createZip(src, src); err == nil {
		t.Error("入出力が同じファイルでもエラーになりませんでした")
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != string(payload) {
		t.Errorf("元データが失われました: %q err=%v", got, err)
	}

	// 「./f.zip」やシンボリックリンクなど表記違いで同じ実体でも拒否する
	if err := createZip(src, filepath.Join(dir, "sub", "..", "f.zip")); err == nil {
		t.Error("表記違いの同一ファイルでもエラーになりませんでした")
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.zip")
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
		if err := createZip(src, link); err == nil {
			t.Error("シンボリックリンク経由の同一実体でもエラーになりませんでした")
		}
	}
}

// 出力が入力フォルダの内側にあると、作成中のZIP自身を梱包するため拒否する。
func TestCreateZipDestInsideSrc(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "zsrc")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := createZip(srcDir, filepath.Join(srcDir, "x.zip")); err == nil {
		t.Error("圧縮先が圧縮元フォルダの内側でもエラーになりませんでした")
	}
}

// 通常の圧縮→解凍の往復が維持されていること。
func TestZipRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "zsrc")
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(dir, "out.zip")
	if err := createZip(srcDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		filepath.Join("zsrc", "a.txt"):        "hello",
		filepath.Join("zsrc", "sub", "b.txt"): "world",
	} {
		if got, err := os.ReadFile(filepath.Join(dest, name)); err != nil || string(got) != want {
			t.Errorf("%s = %q, want %q (err=%v)", name, got, want, err)
		}
	}
}
