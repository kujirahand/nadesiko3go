package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Web検索を伴わない `gonako doc` の動作を確かめる（ネットワークは使わない）。
func TestRunDoc(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doc", "秒待"}, &stdout, &stderr); err != nil {
		t.Fatalf("doc: %v; stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"■ 秒待", "plugin_system", "マニュアル: https://nadesi.com/v3/doc/"} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に%qがありません:\n%s", want, out)
		}
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
