package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileOperationsPreserveExistingFiles(t *testing.T) {
	dir := t.TempDir()
	path, err := createNewFile(dir, "日本語.nako3")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("「こんにちは」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := createNewFile(dir, "日本語.nako3"); err == nil {
		t.Fatal("既存ファイルを新規作成で上書きできてしまいます")
	}
	other, err := createNewFile(dir, "別名.nako3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renameFile(path, "別名.nako3"); err == nil {
		t.Fatal("既存ファイルの名前へ変更できてしまいます")
	}
	renamed, err := renameFile(path, "変更後.nako3")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(renamed)
	if err != nil || string(content) != "「こんにちは」と表示" {
		t.Fatalf("名前変更後の内容が違います: %q, %v", content, err)
	}
	if err := deleteFile(renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(renamed); !os.IsNotExist(err) {
		t.Fatalf("削除したファイルが残っています: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("別のファイルが影響を受けています: %v", err)
	}
}

func TestFileOperationsRejectInvalidNames(t *testing.T) {
	dir := t.TempDir()
	path, err := createNewFile(dir, "元.nako3")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", " ", ".", "..", "../外.nako3", `..\外.nako3`} {
		if _, err := createNewFile(dir, name); err == nil {
			t.Errorf("不正な名前で作成できました: %q", name)
		}
		if _, err := renameFile(path, name); err == nil {
			t.Errorf("不正な名前へ変更できました: %q", name)
		}
	}
	for _, path := range []string{"", ".", string(filepath.Separator)} {
		if err := deleteFile(path); err == nil {
			t.Errorf("削除対象にできないパスが受理されました: %q", path)
		}
	}
}
