package runcwd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNakoRunLocation(t *testing.T) {
	base := t.TempDir()
	scriptDir := filepath.Join(base, "script")
	if err := os.Mkdir(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relBase, err := filepath.Rel(cwd, base)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(scriptDir, "main.nako3")
	missing := filepath.Join(base, "missing")
	plainFile := filepath.Join(base, "file")
	if err := os.WriteFile(plainFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, file, initial, wantFile, wantDir string
	}{
		{"保存済み", file, base, file, scriptDir},
		{"相対ファイル", filepath.Join(relBase, "script", "main.nako3"), "", file, scriptDir},
		{"親が存在しない", filepath.Join(missing, "main.nako3"), base, filepath.Join(missing, "main.nako3"), ""},
		{"未保存", "", base, "gui.nako3", base},
		{"未保存で相対パス", "", relBase, "gui.nako3", base},
		{"未保存で起動時ディレクトリがない", "", missing, "gui.nako3", ""},
		{"未保存で起動時パスがファイル", "", plainFile, "gui.nako3", ""},
		{"未保存で起動時パスが空", "", "", "gui.nako3", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, dir := NakoRunLocation(tt.file, tt.initial)
			if file != tt.wantFile || dir != tt.wantDir {
				t.Fatalf("結果 = (%q, %q), 期待 = (%q, %q)", file, dir, tt.wantFile, tt.wantDir)
			}
		})
	}
}

func TestEnterWorkDir(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	if _, err := EnterWorkDir(missing); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("移動失敗のエラー = %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic が発生しなかった")
			}
		}()
		restore, err := EnterWorkDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer restore()
		if got, err := os.Getwd(); err != nil || got != dir {
			t.Fatalf("移動先 = %q, err = %v", got, err)
		}
		panic("復元の確認")
	}()
	if got, err := os.Getwd(); err != nil || got != original {
		t.Fatalf("復元先 = %q, err = %v", got, err)
	}
}

func TestEnterWorkDirWhenGetwdFails(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	removed := filepath.Join(base, "removed")
	if err := os.Mkdir(removed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(removed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("CWDの復元: %v", err)
		}
	})
	if err := os.Remove(removed); err != nil {
		t.Skipf("カレントディレクトリを削除できない: %v", err)
	}
	if _, err := os.Getwd(); err == nil {
		t.Skip("削除後もGetwdが成功する環境")
	}
	// Getwdが失敗する場合の相対パスのフォールバックも確認する。
	file, dir := NakoRunLocation("missing/../main.nako3", "")
	wantDir := ""
	if stat, err := os.Stat("."); err == nil && stat.IsDir() {
		wantDir = "."
	}
	if file != "main.nako3" || dir != wantDir {
		t.Errorf("Abs失敗時の結果 = (%q, %q)", file, dir)
	}
	restore, err := EnterWorkDir(base)
	if err != nil {
		t.Fatal(err)
	}
	restore()
	if got, err := os.Getwd(); err != nil || got != base {
		t.Fatalf("移動先 = %q, err = %v", got, err)
	}
}
