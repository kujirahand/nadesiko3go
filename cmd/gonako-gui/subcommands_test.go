package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #551: GUIを起動しないサブコマンドのディスパッチを検証する。

func runSub(t *testing.T, args ...string) (bool, string, string, error) {
	t.Helper()
	var out, errOut strings.Builder
	handled, err := runSubcommand(args, &out, &errOut)
	return handled, out.String(), errOut.String(), err
}

func TestSubcommandHelpAndVersion(t *testing.T) {
	handled, out, _, err := runSub(t, "help")
	if !handled || err != nil || !strings.Contains(out, "サブコマンド") || !strings.Contains(out, "doctest") {
		t.Fatalf("help: handled=%v err=%v out=%q", handled, err, out)
	}
	handled, out, _, err = runSub(t, "help", "doc")
	if !handled || err != nil || !strings.Contains(out, "gonako-gui doc") {
		t.Fatalf("help doc: handled=%v err=%v out=%q", handled, err, out)
	}
	if handled, _, _, err = runSub(t, "help", "sonna"); !handled || err == nil {
		t.Fatalf("help unknown: handled=%v err=%v", handled, err)
	}
	for _, name := range []string{"version", "-v", "--version"} {
		handled, out, _, err = runSub(t, name)
		if !handled || err != nil || !strings.Contains(out, guiVersionText()) {
			t.Fatalf("%s: handled=%v err=%v out=%q", name, handled, err, out)
		}
	}
	// 全サブコマンドに個別ヘルプがあること
	for _, name := range subcommandNames {
		if _, ok := subcommandUsage[name]; !ok {
			t.Fatalf("subcommandUsage に %s がない", name)
		}
		if name2, ok := isSubcommandHelp([]string{name, "--help"}); !ok || name2 != name {
			t.Fatalf("isSubcommandHelp(%s) = %q,%v", name, name2, ok)
		}
	}
}

func TestSubcommandUnknownAndPassthrough(t *testing.T) {
	handled, _, errOut, err := runSub(t, "sonna-command")
	if !handled || err == nil || !strings.Contains(errOut, "サブコマンド") {
		t.Fatalf("unknown: handled=%v err=%v errOut=%q", handled, err, errOut)
	}
	// 従来の引数は main に委ねる (handled=false)
	for _, args := range [][]string{
		{},
		{"-dir", "x"},
		{"./my-app/"},
		{"missing.nako3"},
		{"https://example.com"},
	} {
		if handled, _, _, _ := runSub(t, args...); handled {
			t.Fatalf("args=%q がサブコマンドとして処理された", args)
		}
	}
}

func TestSubcommandDoc(t *testing.T) {
	handled, out, _, err := runSub(t, "doc", "秒待")
	if !handled || err != nil || !strings.Contains(out, "秒待") {
		t.Fatalf("doc: handled=%v err=%v out=%q", handled, err, out)
	}
	handled, out, _, err = runSub(t, "doc", "--json", "秒待")
	if !handled || err != nil || !strings.Contains(out, `"keywords"`) {
		t.Fatalf("doc --json: handled=%v err=%v out=%q", handled, err, out)
	}
	if handled, _, _, err = runSub(t, "doc"); !handled || err == nil {
		t.Fatalf("doc no keyword: handled=%v err=%v", handled, err)
	}
}

func TestSubcommandLintAndFormat(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.nako3")
	if err := os.WriteFile(good, []byte("「こんにちは」と表示\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handled, out, _, err := runSub(t, "lint", good)
	if !handled || err != nil || !strings.Contains(out, "文法エラーはありません") {
		t.Fatalf("lint: handled=%v err=%v out=%q", handled, err, out)
	}
	bad := filepath.Join(dir, "bad.nako3")
	if err := os.WriteFile(bad, []byte("もし「A」ならば\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if handled, _, _, err = runSub(t, "lint", bad); !handled || err == nil {
		t.Fatalf("lint bad: handled=%v err=%v", handled, err)
	}
	// format: 標準出力へ整形結果 (書き戻しなし)
	handled, out, _, err = runSub(t, "format", good)
	if !handled || err != nil || !strings.Contains(out, "こんにちは") {
		t.Fatalf("format: handled=%v err=%v out=%q", handled, err, out)
	}
	// format --force: 書き戻す
	handled, _, _, err = runSub(t, "format", "--force", good)
	if !handled || err != nil {
		t.Fatalf("format --force: handled=%v err=%v", handled, err)
	}
}

func TestSubcommandDoctest(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sample.txt")
	body := "■ サンプル\n\n{{{#nako3\n(1+1)を表示\n### 表示結果: 2\n}}}\n"
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	handled, out, _, err := runSub(t, "doctest", dir)
	if !handled || err != nil || !strings.Contains(out, "成功") {
		t.Fatalf("doctest: handled=%v err=%v out=%q", handled, err, out)
	}
	handled, out, _, err = runSub(t, "doctest", "--usage")
	if !handled || err != nil || !strings.Contains(out, "doctest") {
		t.Fatalf("doctest --usage: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestSubcommandBuildAndGengoAndInstall(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app.nako3")
	if err := os.WriteFile(src, []byte("「こんにちは」と表示\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// build: 土台ランタイムにテスト自身の実行ファイルを使う
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	outBin := filepath.Join(dir, "app-built")
	handled, out, _, err := runSub(t, "build", src, "--runtime", self, "--out", outBin)
	if !handled || err != nil || !strings.Contains(out, "を作りました") {
		t.Fatalf("build: handled=%v err=%v out=%q", handled, err, out)
	}
	if info, err := os.Stat(outBin); err != nil || info.Size() == 0 {
		t.Fatalf("build output: %v", err)
	}
	// build --list
	handled, out, _, err = runSub(t, "build", outBin, "--list")
	if !handled || err != nil || !strings.Contains(out, "プログラム:") {
		t.Fatalf("build --list: handled=%v err=%v out=%q", handled, err, out)
	}
	// build --list の余分な引数はエラー (SOLレビューで成立した指摘)
	if handled, _, _, err = runSub(t, "build", outBin, "--list", "extra"); !handled || err == nil {
		t.Fatalf("build --list extra: handled=%v err=%v", handled, err)
	}
	// gengo
	goOut := filepath.Join(dir, "app.go")
	handled, out, _, err = runSub(t, "gengo", src, "--out", goOut)
	if !handled || err != nil || !strings.Contains(out, "を作りました") {
		t.Fatalf("gengo: handled=%v err=%v out=%q", handled, err, out)
	}
	if data, err := os.ReadFile(goOut); err != nil || !strings.Contains(string(data), "package main") {
		t.Fatalf("gengo output: %v", err)
	}
	// install: 起動スクリプトは gonako-gui run 経由であること
	binDir := filepath.Join(dir, "bin")
	handled, out, _, err = runSub(t, "install", src, "--bin", binDir, "--runtime", self)
	if !handled || err != nil || !strings.Contains(out, "配置しました") {
		t.Fatalf("install: handled=%v err=%v out=%q", handled, err, out)
	}
	script, suffix := installedScriptGUI(src, self, "linux")
	if suffix != "" || !strings.Contains(script, " run ") || !strings.Contains(script, src) {
		t.Fatalf("installedScriptGUI: suffix=%q script=%q", suffix, script)
	}
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "gonako-gui-app" {
		t.Fatalf("install output: entries=%v err=%v", entries, err)
	}
}

func TestSubcommandCompatRun(t *testing.T) {
	// リポジトリ同梱の fixture で compat run が動くこと (作業ディレクトリは repo root 前提でなくても、存在しない場合はエラーになることだけ確認)
	handled, _, _, err := runSub(t, "compat")
	if !handled || err == nil {
		t.Fatalf("compat no sub: handled=%v err=%v", handled, err)
	}
	handled, _, _, err = runSub(t, "compat", "sonna")
	if !handled || err == nil {
		t.Fatalf("compat unknown: handled=%v err=%v", handled, err)
	}
}

func TestRunGUIInlineNoArgs(t *testing.T) {
	if err := runGUIInline(nil); err == nil {
		t.Fatal("-e 引数なしがエラーにならなかった")
	}
	if err := runGUIInline([]string{"もし「A」ならば"}); err == nil {
		t.Fatal("-e 文法エラーがエラーにならなかった")
	}
}
