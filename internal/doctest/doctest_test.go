package doctest

import (
	"path/filepath"
	"testing"
)

func TestExtractDisplayResult(t *testing.T) {
	text := "説明\n{{{#nako3\n「A」と表示。\n### 表示結果: A\n### B\n「B」と表示。\n}}}\n"
	tests := Extract(text, "sample.txt")
	if len(tests) != 1 {
		t.Fatalf("Extract returned %d tests, want 1", len(tests))
	}
	got := tests[0]
	if got.Line != 2 || got.Runtime != CNako || got.Expect != "A\nB" {
		t.Fatalf("unexpected test: %#v", got)
	}
	if got.Code != "「A」と表示。\n「B」と表示。" {
		t.Fatalf("Code = %q", got.Code)
	}
}

func TestExtractWebResult(t *testing.T) {
	tests := Extract("{{{#nako3\n1を表示\n### WEB表示結果: 1\n}}}", "web.txt")
	if len(tests) != 1 || tests[0].Runtime != WNako {
		t.Fatalf("WEB表示結果 = %#v", tests)
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
	test := Test{Code: "ナデシコバージョンを表示", Expect: "3.8.1", Runtime: CNako}
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
	if got, want := len(tests), 573; got != want {
		t.Fatalf("core DocTest = %d件, want %d件", got, want)
	}
}
