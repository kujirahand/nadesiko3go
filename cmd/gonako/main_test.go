package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "gonako compat run") {
		t.Fatalf("ヘルプにcompat runがありません: %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"unknown"}, &stdout, &stderr); err == nil {
		t.Fatal("不明なコマンドでエラーになりませんでした")
	}
}

func TestRunInline(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"-e", "「やあ」と表示"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "やあ" {
		t.Errorf("出力 = %q, want \"やあ\"", got)
	}
}

func TestRunFileFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.nako3")
	if err := os.WriteFile(path, []byte("3回\n回数を表示\nここまで"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"run", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "1\n2\n3" {
		t.Errorf("出力 = %q, want \"1\\n2\\n3\"", got)
	}
}

// TestRunReportsNakoError pins that a program's own error comes back as a
// nadesiko message rather than a Go one.
func TestRunReportsNakoError(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"-e", "「わざと」でエラー発生"}, &out, &errOut)
	if err == nil {
		t.Fatal("エラーになるはずが成功した")
	}
	if got := err.Error(); got != "[実行時エラー]main.nako3(1行目): わざと" {
		t.Errorf("Error() = %q", got)
	}
}

func TestRunNeedsAFile(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"run"}, &out, &errOut); err == nil {
		t.Error("ファイル名なしのrunが通ってしまった")
	}
}
