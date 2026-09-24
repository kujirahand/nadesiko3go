package doctest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

const fakeRuntimeEnv = "GONAKO_DOCTEST_FAKE_RUNTIME"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeRuntimeEnv); mode != "" {
		os.Exit(runFakeRuntime(mode))
	}
	os.Exit(m.Run())
}

func runFakeRuntime(mode string) int {
	switch mode {
	case "ok":
		fmt.Print("hello\n")
		return 0
	case "subcommand":
		if len(os.Args) < 3 || os.Args[1] != "run" {
			fmt.Fprintln(os.Stderr, "expected: run <file>")
			return 1
		}
		fmt.Print("hello\n")
		return 0
	case "mismatch":
		fmt.Print("nope\n")
		return 0
	case "fail":
		fmt.Fprintln(os.Stderr, "boom")
		return 1
	case "hang":
		select {}
	default:
		fmt.Fprintln(os.Stderr, "unknown fake runtime mode")
		return 2
	}
}

func TestExtractDisplayResult(t *testing.T) {
	text := "説明\n{{{#nako3\n「A」と表示。\n### 表示結果: A\n### B\n「B」と表示。\n}}}\n"
	tests := Extract(text, "sample.txt")
	if len(tests) != 1 {
		t.Fatalf("Extract returned %d tests, want 1", len(tests))
	}
	got := tests[0]
	if got.Line != 2 || got.Runtime != CNako || got.Label != LabelCNako || got.Expect != "A\nB" {
		t.Fatalf("unexpected test: %#v", got)
	}
	if got.Code != "「A」と表示。\n「B」と表示。" {
		t.Fatalf("Code = %q", got.Code)
	}
}

func TestExtractWebResult(t *testing.T) {
	tests := Extract("{{{#nako3\n1を表示\n### WEB表示結果: 1\n}}}", "web.txt")
	if len(tests) != 1 || tests[0].Runtime != WNako || tests[0].Label != LabelWNako {
		t.Fatalf("WEB表示結果 = %#v", tests)
	}
}

func TestExtractGoResult(t *testing.T) {
	tests := Extract("{{{#nako3\n1を表示\n### GO表示結果: 1\n}}}", "go.txt")
	if len(tests) != 1 || tests[0].Runtime != CNako || tests[0].Label != LabelGOnako {
		t.Fatalf("GO表示結果 = %#v", tests)
	}
}

func TestExtractCustomLabel(t *testing.T) {
	tests := Extract("{{{#nako3\n1を表示\n### L表示結果: 1\n}}}", "lnako.txt")
	if len(tests) != 1 || tests[0].Label != "L表示結果" || tests[0].Expect != "1" {
		t.Fatalf("L表示結果 = %#v", tests)
	}
}

func TestNormalizeLabel(t *testing.T) {
	for _, in := range []string{"L表示結果", "### L表示結果:", "###L表示結果："} {
		if got := NormalizeLabel(in); got != "L表示結果" {
			t.Fatalf("NormalizeLabel(%q) = %q", in, got)
		}
	}
}

func TestRunComparesOutput(t *testing.T) {
	test := Test{File: "sample.txt", Line: 1, Code: "「こんにちは」と表示", Expect: "こんにちは", Runtime: CNako}
	result := Run(test)
	if !result.OK || result.Err != nil || result.Actual != "こんにちは" {
		t.Fatalf("Run = %#v", result)
	}
}

func TestRunSkipsCommandsOutsideTheGoBackend(t *testing.T) {
	test := Test{Code: "『1+1』をJS実行して表示", Expect: "2", Runtime: CNako}
	result := Run(test)
	if !result.Skipped || result.SkipReason != "Go版で省略する命令: JS実行" {
		t.Fatalf("Run = %#v", result)
	}
}

func TestRunDoesNotSkipNadesikoConstants(t *testing.T) {
	test := Test{Code: "ナデシコバージョンを表示", Expect: version.Nadesiko, Runtime: CNako}
	result := Run(test)
	if result.Skipped || !result.OK {
		t.Fatalf("Run = %#v", result)
	}
}

func TestRunSkipsEnvironmentDependentPluginList(t *testing.T) {
	test := Test{Code: "プラグイン一覧取得して表示", Runtime: CNako}
	result := Run(test)
	if !result.Skipped || result.SkipReason != "実行環境で一覧が異なる命令: プラグイン一覧取得" {
		t.Fatalf("Run = %#v", result)
	}
}

func TestRunDoesNotDrainPendingTimers(t *testing.T) {
	test := Test{
		Code:    "「開始」と表示\n「後」を1秒後\n●後\n「完了」と表示\nここまで",
		Expect:  "開始",
		Runtime: CNako,
	}
	result := Run(test)
	if !result.OK || result.Actual != "開始" {
		t.Fatalf("Run = %#v", result)
	}
}

func TestClassifyMissingCommand(t *testing.T) {
	test := Test{File: "sample.txt", Line: 1, Code: "1を未知命令", Runtime: CNako}
	result := Run(test)
	if result.OK || Classify(result) != FailMissingCommand {
		t.Fatalf("Classify = %v, err=%v", Classify(result), result.Err)
	}
}

func TestCollectCoreFixtures(t *testing.T) {
	target := filepath.Join("..", "..", "testdata", "doctest", "core")
	tests, err := Collect([]string{target}, CNako)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tests), 575; got != want {
		t.Fatalf("core DocTest = %d件, want %d件", got, want)
	}
}

func TestCollectByLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	text := "{{{#nako3\n1を表示\n### 表示結果: 1\n}}}\n" +
		"{{{#nako3\n2を表示\n### L表示結果: 2\n}}}\n" +
		"{{{#nako3\n3を表示\n### WEB表示結果: 3\n}}}\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := CollectByLabel([]string{path}, "L表示結果")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Expect != "2" {
		t.Fatalf("CollectByLabel L表示結果 = %#v", got)
	}

	def, err := CollectByLabel([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(def) != 1 || def[0].Label != LabelCNako {
		t.Fatalf("CollectByLabel default = %#v", def)
	}
}

func TestExternalRunnerOK(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "ok")
	result := ExternalRunner(os.Args[0])(Test{Code: "x", Expect: "hello"})
	if !result.OK || result.Err != nil || result.Actual != "hello" {
		t.Fatalf("ExternalRunner ok = %#v", result)
	}
}

func TestExternalRunnerMismatch(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "mismatch")
	result := ExternalRunner(os.Args[0])(Test{Code: "x", Expect: "hello"})
	if result.OK || result.Err != nil || result.Actual != "nope" {
		t.Fatalf("ExternalRunner mismatch = %#v", result)
	}
}

func TestExternalRunnerFail(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "fail")
	result := ExternalRunner(os.Args[0])(Test{Code: "x", Expect: "hello"})
	if result.OK || result.Err == nil {
		t.Fatalf("ExternalRunner fail = %#v", result)
	}
}

func TestExternalRunnerSubcommand(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "subcommand")
	ok := ExternalRunner(os.Args[0], "run")(Test{Code: "x", Expect: "hello"})
	if !ok.OK || ok.Err != nil {
		t.Fatalf("ExternalRunner run = %#v", ok)
	}
	ng := ExternalRunner(os.Args[0])(Test{Code: "x", Expect: "hello"})
	if ng.OK || ng.Err == nil {
		t.Fatalf("ExternalRunner without run should fail: %#v", ng)
	}
}

func TestExternalRunnerDoesNotSkipJS(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "ok")
	test := Test{Code: "『1+1』をJS実行して表示", Expect: "hello"}
	result := ExternalRunner(os.Args[0])(test)
	if result.Skipped {
		t.Fatalf("external runner should not skip JS: %#v", result)
	}
	if !result.OK {
		t.Fatalf("ExternalRunner JS = %#v", result)
	}
}

func TestExternalRunnerTimesOut(t *testing.T) {
	t.Setenv(fakeRuntimeEnv, "hang")
	result := runExternal(os.Args[0], nil, Test{Code: "x"}, 20*time.Millisecond)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "制限時間") {
		t.Fatalf("ExternalRunner timeout = %#v", result)
	}
}
