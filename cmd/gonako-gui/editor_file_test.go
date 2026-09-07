package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestReadEditorFileUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	content := "名前,個数\nりんご,3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result := readEditorFile(path, false)
	if !result.OK || result.IsBinary || result.Content != content || result.Encoding != "UTF-8" {
		t.Fatalf("unexpected UTF-8 result: %#v", result)
	}
}

func TestReadEditorFileNonUTF8RequiresConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	want := "名前,個数\nりんご,3\n"
	encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	probe := readEditorFile(path, false)
	if !probe.OK || !probe.IsBinary || probe.Content != "" {
		t.Fatalf("non-UTF-8 probe must not return content: %#v", probe)
	}

	opened := readEditorFile(path, true)
	if !opened.OK || opened.IsBinary || !opened.Converted || opened.Content != want || opened.Encoding != "Shift_JIS" {
		t.Fatalf("unexpected forced read result: %#v", opened)
	}
}

func TestWriteEditorFilePreservesShiftJIS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	want := "名前,個数\nみかん,5\n"
	if err := writeEditorFile(path, want, editorEncodingShiftJIS); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) == want {
		t.Fatal("Shift_JIS file was written as UTF-8")
	}
	decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), written)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != want {
		t.Fatalf("decoded saved content = %q, want %q", decoded, want)
	}
}

func TestWriteEditorFileDefaultsToUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	want := "こんにちは\n"
	if err := writeEditorFile(path, want, editorEncodingUTF8); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != want {
		t.Fatalf("saved content = %q, want %q", written, want)
	}
}
