package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func encodeTo(t *testing.T, enc string, s string) []byte {
	t.Helper()
	var tr transform.Transformer
	switch enc {
	case editorEncodingShiftJIS:
		tr = japanese.ShiftJIS.NewEncoder()
	case editorEncodingEUCJP:
		tr = japanese.EUCJP.NewEncoder()
	default:
		t.Fatalf("unknown encoding %q", enc)
	}
	encoded, _, err := transform.Bytes(tr, []byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestReadEditorFileUTF8(t *testing.T) {
	content := "名前,個数\nりんご,3\n"
	result := readEditorFile(writeTemp(t, "sample.csv", []byte(content)))
	if !result.OK || result.IsBinary || result.Converted || result.Content != content || result.Encoding != editorEncodingUTF8 {
		t.Fatalf("unexpected UTF-8 result: %#v", result)
	}
}

// Shift_JIS / EUC-JP は確認なしでUTF-8へ変換して開く（#44）
func TestReadEditorFileConvertsJapaneseEncodings(t *testing.T) {
	want := "名前,個数\nりんご,3\nミカン,５\n"
	for _, enc := range []string{editorEncodingShiftJIS, editorEncodingEUCJP} {
		path := writeTemp(t, "sample.csv", encodeTo(t, enc, want))
		result := readEditorFile(path)
		if !result.OK || result.IsBinary {
			t.Fatalf("%s should be readable: %#v", enc, result)
		}
		if !result.Converted || result.Encoding != enc || result.Content != want {
			t.Fatalf("%s conversion failed: %#v", enc, result)
		}
	}
}

// PNGなどのバイナリは内容を返さず、読み取り専用として扱う（#44）
func TestReadEditorFileRejectsBinary(t *testing.T) {
	cases := map[string][]byte{
		"image.png": append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("IHDR\x00\x00\x00\x01")...),
		"photo.jpg": append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("\x00\x10JFIF\x00\x01")...),
		"app.exe":   append([]byte{'M', 'Z'}, make([]byte, 64)...),
		"data.bin":  {0x00, 0x01, 0x02, 0x03, 0x7F, 0x00, 0xFF},
	}
	for name, data := range cases {
		result := readEditorFile(writeTemp(t, name, data))
		if !result.OK {
			t.Fatalf("%s: read failed: %#v", name, result)
		}
		if !result.IsBinary || result.Content != "" || result.Converted {
			t.Fatalf("%s must be treated as binary: %#v", name, result)
		}
	}
}

// UTF-8ともShift_JISともEUC-JPとも解釈できないものはバイナリ扱い
func TestReadEditorFileUnknownBytesAreBinary(t *testing.T) {
	result := readEditorFile(writeTemp(t, "broken.txt", []byte{'a', 0xFF, 0xFF, 'b'}))
	if !result.OK || !result.IsBinary || result.Content != "" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestWriteEditorFilePreservesJapaneseEncoding(t *testing.T) {
	want := "名前,個数\nみかん,5\n"
	for _, enc := range []string{editorEncodingShiftJIS, editorEncodingEUCJP} {
		path := filepath.Join(t.TempDir(), "sample.csv")
		if err := writeEditorFile(path, want, enc); err != nil {
			t.Fatal(err)
		}
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(written) == want {
			t.Fatalf("%s file was written as UTF-8", enc)
		}
		result := readEditorFile(path)
		if result.Encoding != enc || result.Content != want {
			t.Fatalf("%s round trip failed: %#v", enc, result)
		}
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
