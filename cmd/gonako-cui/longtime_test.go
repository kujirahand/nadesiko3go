package main

import (
	"bytes"
	"github.com/kujirahand/nadesiko3go/internal/cliopt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --longtime-error が gonako と同じく gonako-cui でも使えることを確認する (#449)。

func TestRunLongtimeErrorCUI(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(filename, []byte("「OK」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"run", "--longtime-error", filename}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "OK") {
		t.Fatalf("stdout = %q, want OK", got)
	}
}

func TestTakeLongtimeErrorKeepsProgramArgsCUI(t *testing.T) {
	rest, got := cliopt.TakeLongtimeError([]string{"run", "app.nako3", "--longtime-error"})
	if got {
		t.Fatal("ファイル名の後ろのフラグを取り除いてしまった")
	}
	if len(rest) != 3 || rest[2] != "--longtime-error" {
		t.Fatalf("rest = %v, want プログラム引数を保持", rest)
	}
}

func TestRunLongtimeErrorRejectsNonRunSubcommandCUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--longtime-error", "doctest"}, &stdout, &stderr); err == nil {
		t.Fatal("実行系以外で --longtime-error が黙って無視された")
	}
}
