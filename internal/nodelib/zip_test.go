package nodelib

import (
	"archive/zip"
	"os"
	"path/filepath"
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
