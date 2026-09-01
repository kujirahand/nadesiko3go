package compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesExecutionResults(t *testing.T) {
	casesDir := t.TempDir()
	outDir := t.TempDir()
	fixture := `{"group":"01_sample","description":"sample","cases":[` +
		`{"name":"one","code":"1を表示"},` +
		`{"name":"vars","code":"A=[1,2]","vars":["A"]},` +
		`{"name":"error","code":"「わざと」でエラー発生"}]}`
	if err := os.WriteFile(filepath.Join(casesDir, "01_sample.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(casesDir, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Groups != 1 || summary.Cases != 3 {
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

	if got := output.Results["one"]; got.Status != "ok" || got.Log != "1" {
		t.Errorf("表示の結果 = %+v, want status=ok log=1", got)
	}
	if got := output.Results["vars"]; got.Vars == nil {
		t.Errorf("varsが出力されていない: %+v", got)
	}
	got := output.Results["error"]
	if got.Status != "error" || got.Error == nil || got.Error.Type != "NakoRuntimeError" {
		t.Errorf("エラーの結果 = %+v, want NakoRuntimeError", got)
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
	// 件数が動いたら、本家の再同期があったということ。SOURCE と一緒に見直す。
	if len(groups) != 11 || total != 241 {
		t.Fatalf("同期fixtureの件数が変わりました: %d groups, %d cases", len(groups), total)
	}
}

// passingGroups are the groups that must pass in full at the current stage.
// Stage 2 added 01-03 and 07; stage 3 adds the rest except the ones that need
// regular expressions (stage 4) and timers (stage 5).
var passingGroups = []string{
	"01_literal", "02_operator", "03_type_convert",
	"04_string", "05_array", "06_dict", "07_flow", "09_error",
}

// knownFailures are cases that cannot pass as the fixture stands.
var knownFailures = map[string]string{
	// 期待値がTS版の生成したJavaScriptのソースそのもの。『(3をF)』はFを
	// 呼び出さずFの値を表示するだけなので、他の実装では再現できない。
	// 本家へ報告済み (kujirahand/nadesiko3#2456)。
	"無名関数-変数に代入": "期待値がTS版の生成コードに依存している",
}

// TestFixtureGroupsPass runs the synced fixtures and compares them with the
// expected values the TypeScript version generated. It encodes the completion
// condition of the current stage, so a regression is caught immediately.
func TestFixtureGroupsPass(t *testing.T) {
	const casesDir = "../../testdata/compat/cases"
	const expectedDir = "../../testdata/compat/expected"
	if _, err := os.Stat(casesDir); err != nil {
		t.Skip("差分fixtureが同期されていません")
	}
	outDir := t.TempDir()
	if _, err := Run(casesDir, outDir); err != nil {
		t.Fatal(err)
	}

	groups, err := Load(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Group{}
	for _, g := range groups {
		byName[g.Group] = g
	}

	for _, name := range passingGroups {
		t.Run(name, func(t *testing.T) {
			group, ok := byName[name]
			if !ok {
				t.Fatalf("グループ %s がありません", name)
			}
			want := readResults(t, filepath.Join(expectedDir, name+".json"))
			got := readResults(t, filepath.Join(outDir, name+".json"))
			for _, c := range group.Cases {
				if _, skip := c.Unsupported["go"]; skip {
					continue
				}
				if _, skip := c.IntentionalDiff["go"]; skip {
					continue
				}
				if _, skip := knownFailures[c.Name]; skip {
					continue
				}
				compareResult(t, c.Name, want[c.Name], got[c.Name])
			}
		})
	}
}

// readResults loads one group's results as raw JSON, so that the comparison
// sees exactly what was written rather than a re-encoded copy.
func readResults(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out.Results
}

// compareResult checks the fields SPEC.md says an implementation must match.
func compareResult(t *testing.T, name string, wantRaw, gotRaw json.RawMessage) {
	t.Helper()
	var want, got Result
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("%s: 期待値を読めません: %v", name, err)
	}
	if err := json.Unmarshal(gotRaw, &got); err != nil {
		t.Fatalf("%s: 結果を読めません: %v", name, err)
	}
	if got.Status != want.Status {
		t.Errorf("%s: status = %q, want %q", name, got.Status, want.Status)
		return
	}
	if got.Log != want.Log {
		t.Errorf("%s: log = %q, want %q", name, got.Log, want.Log)
	}
	if want.Status == "error" {
		wantMsg, gotMsg := "", ""
		if want.Error != nil {
			wantMsg = want.Error.Message
		}
		if got.Error != nil {
			gotMsg = got.Error.Message
		}
		if gotMsg != wantMsg {
			t.Errorf("%s: エラー文面\n got %q\nwant %q", name, gotMsg, wantMsg)
		}
	}
	if !sameVars(wantRaw, gotRaw) {
		t.Errorf("%s: vars が一致しません\n got %s\nwant %s", name, varsOf(gotRaw), varsOf(wantRaw))
	}
}

// sameVars compares the vars objects structurally, ignoring key order.
func sameVars(wantRaw, gotRaw json.RawMessage) bool {
	return string(canonicalVars(wantRaw)) == string(canonicalVars(gotRaw))
}

func varsOf(raw json.RawMessage) string { return string(canonicalVars(raw)) }

// canonicalVars re-encodes the vars object so that map key order does not
// affect the comparison.
func canonicalVars(raw json.RawMessage) []byte {
	var holder struct {
		Vars map[string]any `json:"vars"`
	}
	if err := json.Unmarshal(raw, &holder); err != nil {
		return []byte("(読み取り失敗)")
	}
	out, err := json.Marshal(holder.Vars)
	if err != nil {
		return []byte("(書き出し失敗)")
	}
	return out
}
