package compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesUnsupportedResults(t *testing.T) {
	casesDir := t.TempDir()
	outDir := t.TempDir()
	fixture := `{"group":"01_sample","description":"sample","cases":[{"name":"one","code":"1を表示"},{"name":"two","code":"2を表示"}]}`
	if err := os.WriteFile(filepath.Join(casesDir, "01_sample.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(casesDir, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Groups != 1 || summary.Cases != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "01_sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	var output OutputGroup
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	got := output.Results["one"]
	if got.Status != "error" || got.Error == nil || got.Error.Type != "UnsupportedError" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestLoadRejectsDuplicateCaseNames(t *testing.T) {
	casesDir := t.TempDir()
	fixture := `{"group":"bad","cases":[{"name":"same","code":"1"},{"name":"same","code":"2"}]}`
	if err := os.WriteFile(filepath.Join(casesDir, "bad.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(casesDir); err == nil {
		t.Fatal("重複したケース名を受理しました")
	}
}

func TestLoadRejectsMissingCode(t *testing.T) {
	casesDir := t.TempDir()
	fixture := `{"group":"bad","cases":[{"name":"missing"}]}`
	if err := os.WriteFile(filepath.Join(casesDir, "bad.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(casesDir); err == nil {
		t.Fatal("codeのないケースを受理しました")
	}
}

func TestSyncedFixturesLoad(t *testing.T) {
	groups, err := Load(filepath.Join("..", "..", "testdata", "compat", "cases"))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, group := range groups {
		total += len(group.Cases)
	}
	if len(groups) != 11 || total != 239 {
		t.Fatalf("同期fixtureの件数が変わりました: %d groups, %d cases", len(groups), total)
	}
}

func TestSyncedFixturesReachStage1ParseGoal(t *testing.T) {
	outDir := t.TempDir()
	_, err := Run(filepath.Join("..", "..", "testdata", "compat", "cases"), outDir)
	if err != nil {
		t.Fatal(err)
	}

	wantErrors := map[string]string{
		"文法エラー-未解決の単語":    "NakoSyntaxError",
		"文法エラー-閉じ括弧なし":    "NakoSyntaxError",
		"字句解析エラー-文字列の入れ子": "NakoLexerError",
		"文法エラー-ここまでの不足":   "NakoSyntaxError",
	}
	gotErrors := map[string]string{}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var output OutputGroup
		if err := json.Unmarshal(data, &output); err != nil {
			t.Fatal(err)
		}
		for name, result := range output.Results {
			if result.Error != nil && result.Error.Type != "UnsupportedError" {
				gotErrors[name] = result.Error.Type
			}
		}
	}
	if len(gotErrors) != len(wantErrors) {
		t.Fatalf("解析エラー件数 = %d, want %d: %#v", len(gotErrors), len(wantErrors), gotErrors)
	}
	for name, wantType := range wantErrors {
		if gotErrors[name] != wantType {
			t.Errorf("%s: %q != %q", name, gotErrors[name], wantType)
		}
	}
}
