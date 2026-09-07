package main

import (
	"os"
	"path/filepath"
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
