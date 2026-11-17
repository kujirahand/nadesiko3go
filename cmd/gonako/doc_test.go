package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/commanddoc"
)

// Web検索を伴わない `gonako doc` の動作を確かめる（ネットワークは使わない）。
func TestRunDoc(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "秒待"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc: %v; stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if json.Valid(stdout.Bytes()) {
		t.Error("通常の検索がJSONになっています")
	}
	for _, want := range []string{"■ 秒待", "plugin_system", "マニュアル: https://nadesi.com/v3/doc/"} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に%qがありません:\n%s", want, out)
		}
	}
}

func TestRunDocAllJSON(t *testing.T) {
	gonako, err := commanddoc.Commands()
	if err != nil {
		t.Fatal(err)
	}
	wnako, err := commanddoc.WNakoCommands()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		target string
		list   []commanddoc.Command
	}{
		{"通常", []string{"doc", "*"}, "gonako", gonako},
		{"JSON指定", []string{"doc", "--json", "*"}, "gonako", gonako},
		{"件数制限とWeb", []string{"doc", "--limit", "3", "--web", "*"}, "gonako", gonako},
		{"wnako", []string{"doc", "--wnako", "*"}, "wnako", wnako},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(tc.args, &stdout, &stderr); err != nil {
				t.Fatalf("doc: %v; stderr=%s", err, stderr.String())
			}
			var result docResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("JSONとして読めません: %v", err)
			}
			if !reflect.DeepEqual(result.Keywords, []string{"*"}) || result.Target != tc.target {
				t.Errorf("キーワードまたは対象が違います: %v, %s", result.Keywords, result.Target)
			}
			if result.Count != len(result.Commands) || result.Count <= 100 {
				t.Errorf("件数が違います: count=%d commands=%d", result.Count, len(result.Commands))
			}
			if !reflect.DeepEqual(result.Commands, tc.list) {
				t.Error("命令一覧の内容または順序が違います")
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["count"]; !ok {
				t.Error("countがありません")
			}
			if _, ok := fields["web"]; ok {
				t.Error("webが出力されています")
			}
			if _, ok := fields["web_error"]; ok {
				t.Error("web_errorが出力されています")
			}
			found := false
			for _, cmd := range result.Commands {
				if cmd.Name == "表示" {
					found = true
				}
			}
			if !found {
				t.Error("表示がありません")
			}
		})
	}
}

func TestRunDocExpandedGlob(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile("dummy1", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("dummy2", 0700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "dummy1", "dummy2"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var result docResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Keywords, []string{"*"}) || result.Count <= 100 || result.Count != len(result.Commands) {
		t.Errorf("全件出力ではありません: keywords=%v count=%d", result.Keywords, result.Count)
	}
	if !strings.Contains(stderr.String(), `gonako doc "*"`) {
		t.Errorf("引用の注意がありません: %s", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"doc", "dummy1"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if json.Valid(stdout.Bytes()) || !strings.Contains(stdout.String(), "dummy1") || stderr.Len() != 0 {
		t.Errorf("単一ファイル名が検索されません: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// キーワードはオプションの前でも後ろでも書けること。
func TestRunDocKeywordBeforeFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "秒待", "-c", "--limit", "1"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc: %v; stderr=%s", err, stderr.String())
	}
	if got := strings.Count(stdout.String(), "■ "); got != 1 {
		t.Fatalf("--limitが効いていません: %d件\n%s", got, stdout.String())
	}
	if !strings.Contains(stdout.String(), "省略しました") {
		t.Errorf("省略の案内がありません:\n%s", stdout.String())
	}
}

func TestRunDocJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "--json", "--limit", "3", "秒待"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc --json: %v; stderr=%s", err, stderr.String())
	}
	var result docResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("JSONとして読めません: %v\n%s", err, stdout.String())
	}
	if len(result.Keywords) != 1 || result.Keywords[0] != "秒待" {
		t.Errorf("キーワードが違います: %#v", result.Keywords)
	}
	if len(result.Commands) == 0 || result.Commands[0].Name != "秒待" {
		t.Fatalf("命令が見つかりません: %#v", result.Commands)
	}
	if result.Commands[0].DocURL == "" {
		t.Error("マニュアルURLがありません")
	}
	if len(result.Web) != 0 {
		t.Errorf("--web無しでWeb検索しています: %#v", result.Web)
	}
}

func TestRunDocNoMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "存在しない命令ZZZ"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc: %v", err)
	}
	if !strings.Contains(stdout.String(), "一致する命令はありませんでした") {
		t.Errorf("見つからない旨の表示がありません:\n%s", stdout.String())
	}
}

func TestRunDocWithoutKeyword(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc"}, &stdout, &stderr); err == nil {
		t.Fatal("キーワード無しはエラーになるはずです")
	}
}

// --wnako で本家ブラウザ版(wnako3)の命令一覧を検索できること（#101）。
func TestRunDocWNako(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "カメ作成", "--wnako", "--limit", "1"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc --wnako: %v; stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"(wnako)", "■ カメ作成", "plugin_turtle"} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に%qがありません:\n%s", want, out)
		}
	}

	// --wnako を付けなければGo版(gonako)の一覧を引くこと。
	stdout.Reset()
	if err := run([]string{"doc", "カメ作成"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc: %v; stderr=%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "(wnako)") {
		t.Errorf("既定でwnakoの一覧を引いています:\n%s", stdout.String())
	}
}

func TestRunDocWNakoJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "--wnako", "--json", "--limit", "1", "表示"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc --wnako --json: %v; stderr=%s", err, stderr.String())
	}
	var result docResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("JSONとして読めません: %v\n%s", err, stdout.String())
	}
	if result.Target != "wnako" {
		t.Errorf("targetが違います: %q", result.Target)
	}
	if len(result.Commands) == 0 || result.Commands[0].Name != "表示" {
		t.Fatalf("命令が見つかりません: %#v", result.Commands)
	}
	if !strings.Contains(result.Commands[0].DocURL, "plugin_system") {
		t.Errorf("マニュアルURLが違います: %s", result.Commands[0].DocURL)
	}
}
