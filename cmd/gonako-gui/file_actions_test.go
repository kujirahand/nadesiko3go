package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateNewFolder(t *testing.T) {
	base := t.TempDir()
	created, err := createNewFolder(base, "資料")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "資料")
	if created != want {
		t.Fatalf("created path = %q, want %q", created, want)
	}
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("folder was not created: info=%v err=%v", info, err)
	}
}

func TestCreateNewFolderRejectsInvalidName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", `a\b`} {
		if _, err := createNewFolder(t.TempDir(), name); err == nil {
			t.Errorf("createNewFolder accepted invalid name %q", name)
		}
	}
}

func TestCreateAIProject(t *testing.T) {
	projectPath := t.TempDir()
	gotPath, files, err := createAIProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != projectPath {
		t.Fatalf("project path = %q, want %q", gotPath, projectPath)
	}
	if got := aiProjectFileNames(files); len(got) != 2 || got[0] != "AGENTS.md" || got[1] != "CLAUDE.md" {
		t.Fatalf("created files = %#v", got)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, required := range []string{
			"必ず日本語で応答", "文法メモ", "gonako doc キーワード --json",
			"gonako doc キーワード --web", "gonako run main.nako3", "命令名や助詞を推測",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("%sに%qがありません", name, required)
			}
		}
	}
}

func TestCreateAIProjectDoesNotOverwriteExistingFiles(t *testing.T) {
	projectPath := t.TempDir()
	important := filepath.Join(projectPath, "AGENTS.md")
	if err := os.WriteFile(important, []byte("残す"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := createAIProject(projectPath); err == nil {
		t.Fatal("既存のAGENTS.mdを受け入れてしまった")
	}
	data, err := os.ReadFile(important)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "残す" {
		t.Fatalf("既存ファイルを上書きした: %q", data)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("片方だけ作成してしまった: %v", err)
	}
}
