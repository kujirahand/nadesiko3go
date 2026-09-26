package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScript(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "utility.nako3")
	if err := os.WriteFile(source, []byte("「起動」と表示"), 0600); err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "bin")
	args := []string{"install", source, "--bin", target, "--runtime", bin}
	var out, errOut bytes.Buffer
	if err := run(args, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".ps1"
	}
	file := filepath.Join(target, "gonako-utility"+suffix)
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 != 0111 {
		t.Fatal("実行権限がありません")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), source) {
		t.Fatal("ソースパスがありません")
	}
	if err := run(args, &out, &errOut); err == nil {
		t.Fatal("既存ファイルを上書きしました")
	}
	if err := run(append(args, "--force"), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := run(append(args, "--name", "../outside"), &out, &errOut); err == nil {
		t.Fatal("パスを含む名前を受け付けました")
	}
	if err := run([]string{"install", filepath.Join(dir, "missing.nako3"), "--bin", target}, &out, &errOut); err == nil {
		t.Fatal("存在しないソースを受け付けました")
	}
}

func TestInstalledPowerShellQuotes(t *testing.T) {
	script, suffix := installedScript("C:\\space ' name\\tool.nako3", "C:\\runtime ' name\\gonako.exe", "windows")
	if suffix != ".ps1" || !strings.Contains(script, "C:\\space '' name\\tool.nako3") || !strings.Contains(script, "@args") || !strings.Contains(script, "exit $code") {
		t.Fatalf("引用と引数・終了コード: %s", script)
	}
}
