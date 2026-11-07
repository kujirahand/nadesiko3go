package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --longtime-error を付けても、通常のプログラムがそのまま実行できることと、
// フラグが実行対象より前の位置でのみ取り除かれることを確認する (#449)。

func writeLongtimeSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	filename := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(filename, []byte("「OK」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestRunLongtimeErrorWithRunSubcommand(t *testing.T) {
	filename := writeLongtimeSource(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"run", "--longtime-error", filename}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "OK") {
		t.Fatalf("stdout = %q, want OK", got)
	}
}

func TestRunLongtimeErrorBeforeRunSubcommand(t *testing.T) {
	filename := writeLongtimeSource(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--longtime-error", "run", filename}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "OK") {
		t.Fatalf("stdout = %q, want OK", got)
	}
}

func TestRunLongtimeErrorDirectFile(t *testing.T) {
	filename := writeLongtimeSource(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--longtime-error", filename}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "OK") {
		t.Fatalf("stdout = %q, want OK", got)
	}
}

func TestRunLongtimeErrorInline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-e", "--longtime-error", "「OK」と表示"}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "OK") {
		t.Fatalf("stdout = %q, want OK", got)
	}
}

func TestRunLongtimeErrorRejectsNonRunSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--longtime-error", "build", "app.nako3"}, &stdout, &stderr); err == nil {
		t.Fatal("実行系以外で --longtime-error が黙って無視された")
	}
}

func TestTakeLongtimeErrorKeepsProgramArgs(t *testing.T) {
	// ファイル名より後ろの --longtime-error はプログラムへの引数として残す。
	rest, got := takeLongtimeError([]string{"run", "app.nako3", "--longtime-error"})
	if got {
		t.Fatal("ファイル名の後ろのフラグを取り除いてしまった")
	}
	if len(rest) != 3 || rest[2] != "--longtime-error" {
		t.Fatalf("rest = %v, want プログラム引数を保持", rest)
	}
	// フラグが無い場合はそのまま。
	rest, got = takeLongtimeError([]string{"run", "app.nako3"})
	if got || len(rest) != 2 {
		t.Fatalf("rest = %v, longtimeError = %v", rest, got)
	}
}
