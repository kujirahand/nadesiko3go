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

// Zip Slip対策が維持されていること（".."を含むエントリは飛ばす）。
func TestExtractZipZipSlip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"../evil.txt":         "evil",
		"sub/../../evil2.txt": "evil2",
		"ok/good.txt":         "good",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err != nil {
		t.Fatal(err)
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
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"link/pwn.txt": "pwn",
		"victim.txt":   "overwritten",
		"ok.txt":       "ok",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn.txt")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外に書き込まれました")
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
