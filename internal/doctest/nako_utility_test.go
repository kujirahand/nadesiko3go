package doctest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// なでしこ版を実際のランタイムで起動し、抽出・照合と終了状態を確認する。
func TestNakoUtility(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "gonako runtime")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/gonako")
	build.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOROOT=") {
			build.Env = append(build.Env, entry)
		}
	}
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("ビルド: %v\n%s", err, output)
	}
	source := filepath.Join(root, "gonako-package/doctest/index.nako3")
	fixture := filepath.Join(dir, "sample.txt")
	text := "{{{#nako3\r\n「前」と表示。\r\n### 表示結果： 前\r\n### 後\r\n「後」と表示。\r\n}}}\r\n{{{#nako3\r\n「独自」と表示。\r\n### L表示結果: 独自\r\n}}}\r\n{{{#nako3\r\n「説明」と表示。\r\n}}}\r\n"
	if err := os.WriteFile(fixture, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(wantCode int, want string, args ...string) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{source}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GONAKO_DOCTEST_RUNTIME="+binary)
		output, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != wantCode || !strings.Contains(string(output), want) {
			t.Fatalf("args=%q: code=%d, 出力=%s; 期待=%d, %s", args, code, output, wantCode, want)
		}
	}
	run(0, "1件成功", fixture)
	run(0, "1件成功", "--runtime", binary, "--subcommand", "run", "--label", "### L表示結果：", fixture)
	run(0, "1件成功", "--label=L表示結果", fixture)
	run(2, "引数が不足", "--runtime")
	run(2, "maxは", "-max=-1", fixture)
	run(2, "未対応の引数", "--unknown")
	run(2, "一緒に", "--subcommand=run", fixture)
	run(2, "見つかりません", filepath.Join(dir, "missing.txt"))
	if err := os.WriteFile(fixture, []byte("{{{#nako3\n「違う」と表示。\n### 表示結果: 期待\n}}}\n{{{#nako3\n「違う」と表示。\n### 表示結果: 期待\n}}}"), 0600); err != nil {
		t.Fatal(err)
	}
	run(1, "2件失敗", "-max", "1", fixture)
	if err := os.WriteFile(fixture, []byte("{{{#nako3\n「JS実行」と表示。\n### 表示結果: JS実行\n}}}"), 0600); err != nil {
		t.Fatal(err)
	}
	run(0, "1件省略", fixture)
	run(0, "1件成功・0件省略", "--runtime="+binary, fixture)
	// ビルドした実行ファイルから埋め込みの省略形で取り込む。
	cmd := exec.Command(binary, "-e", "!「doctest」を取り込む。", "--help")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "使い方:") {
		t.Fatalf("埋込取込: %v\n%s", err, output)
	}
}
