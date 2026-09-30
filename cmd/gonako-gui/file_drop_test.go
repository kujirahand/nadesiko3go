package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// ひな形の「には」構文から登録し、ドロップのたびに同じVMで処理する。
func TestFileDropSampleAndBrowserBridge(t *testing.T) {
	code, err := os.ReadFile("ui/samples/15_ファイルドロップ.nako3")
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	initial := session.run(string(code), "drop.nako3", true, nil, nil)
	if !initial.OK {
		t.Fatalf("ひな形の実行: %s", initial.Error)
	}
	if len(initial.Operations) != 1 || initial.Operations[0].Type != "listen" || initial.Operations[0].Event != "drop" {
		t.Fatalf("イベント登録: %#v", initial.Operations)
	}
	for _, files := range []string{`["最初.txt","画像.png"]`, `["次の.csv"]`} {
		result := dispatchEvent(t, session, initial.RunID, 0, "drop", map[string]string{"__gonako_drop_files": files})
		if !result.OK {
			t.Fatalf("ドロップ結果: %#v", result)
		}
		// 配列表示の書式には依存せず、各ファイル名を確認する。
		var names []string
		if err := json.Unmarshal([]byte(files), &names); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if !strings.Contains(result.Output, name) {
				t.Fatalf("ドロップ結果: %#v", result)
			}
		}
	}

	// Goが実際に出力するJSONを両方の画面へ渡す。ハンドル0の省略も含む。
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("JavaScriptの確認にはNode.jsが必要です")
	}
	operations, err := json.Marshal(initial.Operations)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "testdata/file-drop-regression.cjs")
	cmd.Stdin = bytes.NewReader(operations)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("画面へのドロップ: %v\n%s", err, output)
	}
}
