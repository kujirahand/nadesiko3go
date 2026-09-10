package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
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

func TestRunCompatCommands(t *testing.T) {
	source := filepath.Join(t.TempDir(), "command_list.json")
	data := `[{"plugin":"plugin_system","type":"関数","name":"表示"},{"plugin":"plugin_system","type":"関数","name":"本家のみ"}]`
	if err := os.WriteFile(source, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"compat", "commands", "--source", source, "--source-ref", "test-ref"}, &stdout, &stderr); err != nil {
		t.Fatalf("compat commands: %v; stderr=%s", err, stderr.String())
	}
	for _, want := range []string{"比較元コミット: test-ref", "未実装 (1):\n  本家のみ", "Go版のみ"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("出力に%qがありません:\n%s", want, stdout.String())
		}
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

func TestRunFileDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "direct.nako3")
	if err := os.WriteFile(path, []byte("A=コマンドライン\n「引数: {A}」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{path, "foo", "bar"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "引数: foo,bar" {
		t.Errorf("出力 = %q, want \"引数: foo,bar\"", got)
	}
}

func TestRunFileWithShebang(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shebang.nako3")
	code := "#!/usr/bin/env gonako\n「shebang動いた」と表示"
	if err := os.WriteFile(path, []byte(code), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "shebang動いた" {
		t.Errorf("出力 = %q, want \"shebang動いた\"", got)
	}
}

// TestRunRequiresLocalFile は、`!「file」を取込` でローカルの.nako3ファイル
// を読み込み、取り込んだ関数を呼び出せることを確認する (#58)。
func TestRunRequiresLocalFile(t *testing.T) {
	dir := t.TempDir()
	lib := "●（Aを）二倍表示とは\n    (A*2)を表示\nここまで"
	if err := os.WriteFile(filepath.Join(dir, "lib.nako3"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	main := "!「lib.nako3」を取込。\n21を二倍表示。"
	path := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(path, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"run", path}, &out, &errOut); err != nil {
		t.Fatalf("run: %v; stderr=%s", err, errOut.String())
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "42" {
		t.Errorf("出力 = %q, want \"42\"", got)
	}
}

// TestRunRequiresDedupesAndAllowsCircular は、同じファイルを（直接2回、
// または循環取込を通じて）何度取り込んでも一度しか読み込まれないことを
// 確認する。本家TypeScript版のNakoRequireLoaderのグローバルなinclude
// guardと同じ挙動。
func TestRunRequiresDedupesAndAllowsCircular(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.nako3": "!「b.nako3」を取込。\n●A命令とは\n    「A」と表示\n    B命令\nここまで",
		"b.nako3": "!「a.nako3」を取込。\n●B命令とは\n    「B」と表示\nここまで",
	}
	for name, code := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	main := "!「a.nako3」を取込。\n!「a.nako3」を取込。\nA命令。"
	path := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(path, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"run", path}, &out, &errOut); err != nil {
		t.Fatalf("run: %v; stderr=%s", err, errOut.String())
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "A\nB" {
		t.Errorf("出力 = %q, want \"A\\nB\"", got)
	}
}

// TestRunRequiresMissingFileReportsError は、存在しない取込先ファイルが
// Goのエラーそのままではなくなでしこのエラーとして報告されることを確認する。
func TestRunRequiresMissingFileReportsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(path, []byte("!「no_such_file.nako3」を取込。"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err := run([]string{"run", path}, &out, &errOut)
	if err == nil {
		t.Fatal("存在しないファイルの取込でエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "no_such_file.nako3") {
		t.Errorf("エラー = %v, ファイル名を含んでいません", err)
	}
}

// TestRunRequiresDedupesRelativeAndAbsolutePaths は、同じファイルを相対パス
// と絶対パスの両方で取込しても一度しか読み込まれないことを確認する
// (PR #61 レビュー指摘の回帰テスト)。
func TestRunRequiresDedupesRelativeAndAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	side := filepath.Join(dir, "side.nako3")
	if err := os.WriteFile(side, []byte("「LOAD」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := fmt.Sprintf("!「side.nako3」を取込。\n!「%s」を取込。\n", side)
	path := filepath.Join(dir, "main.nako3")
	if err := os.WriteFile(path, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relPath, err := filepath.Rel(workDir, path)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"run", relPath}, &out, &errOut); err != nil {
		t.Fatalf("run: %v; stderr=%s", err, errOut.String())
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "LOAD" {
		t.Errorf("出力 = %q, want \"LOAD\"（相対パスと絶対パスの取込は同一ファイルとして1回だけ実行されるべき）", got)
	}
}

func TestRunVersion(t *testing.T) {
	for _, flag := range []string{"version", "-v", "--version"} {
		var out, errOut bytes.Buffer
		if err := run([]string{flag}, &out, &errOut); err != nil {
			t.Fatalf("%s failed: %v", flag, err)
		}
		if !strings.HasPrefix(out.String(), "gonako v"+Version) {
			t.Errorf("%s 出力 = %q", flag, out.String())
		}
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

func TestExistingDocTestTargetsSkipsMissingOptionalPaths(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "fixtures")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	got := existingDocTestTargets([]string{filepath.Join(dir, "manual"), existing})
	if len(got) != 1 || got[0] != existing {
		t.Fatalf("existingDocTestTargets = %v, want [%s]", got, existing)
	}
}

// TestBuildAndRunBundle pins the whole packaging path from the command line:
// build a program with resources, then run the result somewhere the resources
// do not exist.
func TestBuildAndRunBundle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("data/msg.txt", "同梱の中身")
	write("app.nako3", "A=「data/msg.txt」を開く\n「読めた: {A}」と表示")
	// 土台のランタイムは中身を問わない。バイト列を後ろに足すだけ。
	write("runtime", "ランタイムの代わり")

	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var out, errOut bytes.Buffer
	args := []string{"build", "app.nako3", "--resource", "./data", "--runtime", "runtime", "--out", "packed"}
	if err := run(args, &out, &errOut); err != nil {
		t.Fatalf("build: %v (%s)", err, errOut.String())
	}
	if !strings.Contains(out.String(), "packed を作りました") {
		t.Errorf("build の出力 = %q", out.String())
	}

	packed, err := bundle.Open(filepath.Join(dir, "packed"))
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	// リソースの実体がない場所で動かす
	elsewhere := t.TempDir()
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	var ranOut bytes.Buffer
	host := vm.NewCUIHost(&ranOut, strings.NewReader(""), nil)
	host.Bundle = packed
	if err := vm.RunCompiled(packed.Program, host); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(ranOut.String(), "\n"); got != "読めた: 同梱の中身" {
		t.Errorf("実行結果 = %q", got)
	}
}

// TestBuildAcceptsFileNameEitherSide pins that the source can be written
// before or after the options, which the flag package alone does not allow.
func TestBuildAcceptsFileNameEitherSide(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"app.nako3": "1を表示",
		"runtime":   "ランタイムの代わり",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	orders := [][]string{
		{"build", "app.nako3", "--runtime", "runtime", "--out", "a"},
		{"build", "--runtime", "runtime", "--out", "b", "app.nako3"},
		{"build", "--runtime=runtime", "app.nako3", "--out=c"},
	}
	for _, args := range orders {
		var out, errOut bytes.Buffer
		if err := run(args, &out, &errOut); err != nil {
			t.Errorf("%v: %v (%s)", args, err, errOut.String())
		}
	}
}

func TestBuildList(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "res"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "res/data.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.nako3"), []byte("1を表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime"), []byte("ランタイム"), 0o644); err != nil {
		t.Fatal(err)
	}

	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var out, errOut bytes.Buffer
	if err := run([]string{"build", "app.nako3", "--resource", "./res", "--runtime", "runtime", "--out", "pkg"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}

	var listOut, listErr bytes.Buffer
	if err := run([]string{"build", "--list", "pkg"}, &listOut, &listErr); err != nil {
		t.Fatalf("build --list: %v", err)
	}
	if !strings.Contains(listOut.String(), "プログラム: app.nako3") || !strings.Contains(listOut.String(), "res/data.txt") {
		t.Errorf("list output = %q", listOut.String())
	}
}

func TestBundledExitCode(t *testing.T) {
	dir := t.TempDir()
	code := "3で強制終了"
	prog, err := vm.CompileProgram(code, "exit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	packedPath := filepath.Join(dir, "packed")
	if err := bundle.Build(packedPath, filepath.Join(dir, "rt"), prog, "exit.nako3", ""); err != nil {
		// rt doesn't exist, create it first
		_ = os.WriteFile(filepath.Join(dir, "rt"), []byte("rt"), 0o755)
		if err := bundle.Build(packedPath, filepath.Join(dir, "rt"), prog, "exit.nako3", ""); err != nil {
			t.Fatal(err)
		}
	}
	packed, err := bundle.Open(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	var buf bytes.Buffer
	host := vm.NewCUIHost(&buf, strings.NewReader(""), nil)
	host.Bundle = packed
	err = vm.RunCompiled(packed.Program, host)
	if err != nil {
		t.Fatalf("RunCompiled returned unexpected error: %v", err)
	}
	if !host.Exited || host.ExitCode != 3 {
		t.Errorf("host.Exited = %v, host.ExitCode = %d, want true, 3", host.Exited, host.ExitCode)
	}
}
