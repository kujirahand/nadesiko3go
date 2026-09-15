package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWNako3RunHandlerServesPageAndProgram(t *testing.T) {
	site := fstest.MapFS{"data.txt": {Data: []byte("folder file")}}
	handler := newWNako3RunHandler(site, wnako3RunProgram{Name: "main.nako3", Code: "「こんにちは」を表示"})

	code, page := getSite(t, handler, wnako3RunPagePath)
	if code != http.StatusOK {
		t.Fatalf("実行画面を配信できない: %d", code)
	}
	for _, required := range []string{
		`<canvas id="turtle_cv"`,
		`/__gonako/wnako3/wnako3.js`,
		`/__gonako/wnako3/plugin_turtle.js`,
		`/__gonako-program.json`,
		"registerWNako3Plugin()",
		"loadDependencies(program.code, program.name)",
		"runAsync(program.code, program.name)",
		"addListener('stdout'",
	} {
		if !strings.Contains(page, required) {
			t.Fatalf("実行画面に %q が無い", required)
		}
	}

	code, body := getSite(t, handler, wnako3RunProgramPath)
	var program wnako3RunProgram
	if code != http.StatusOK || json.Unmarshal([]byte(body), &program) != nil ||
		program.Name != "main.nako3" || program.Code != "「こんにちは」を表示" {
		t.Fatalf("プログラムを返せない: %d %s", code, body)
	}

	// プログラムのフォルダと同梱のwnako3も配信する
	if code, body := getSite(t, handler, "/data.txt"); code != http.StatusOK || body != "folder file" {
		t.Fatalf("フォルダのファイルを配信できない: %d %q", code, body)
	}
	if code, _ := getSite(t, handler, "/plugin_turtle.js"); code != http.StatusOK {
		t.Fatalf("取り込む用のplugin_turtle.jsを配信できない: %d", code)
	}
}

func TestEditorHasWNako3RunMode(t *testing.T) {
	index := readUIAsset(t, "index.html")
	if !strings.Contains(index, `<option value="wnako3">ブラウザ(wnako3)</option>`) {
		t.Fatal("index.html に「ブラウザ(wnako3)」の実行モードが無い")
	}
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"wnako3: 'ブラウザ(wnako3)'",
		"const isWNako3Mode = runMode === 'wnako3';",
		"window.runNakoInWNako3(code, currentFilePath || '')",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js に %q が無い", required)
		}
	}
}

// 完了した通常実行も、doneを読んだ時点でセッションから捨てる（#63）。
// window.gonako.run を繰り返しても実行状態が溜まり続けないこと。
func TestPollForgetsFinishedRun(t *testing.T) {
	session := &guiSession{window: newVirtualWindowController()}
	for i := 0; i < 3; i++ {
		id := session.start("「こんにちは」を表示", "gui.nako3", false, nil, nil)
		var output string
		for {
			status := session.poll(id)
			output += status.Output
			if status.Done {
				if status.Result == nil || !status.Result.OK {
					t.Fatalf("実行に失敗しました: %+v", status.Result)
				}
				break
			}
		}
		if output != "こんにちは\n" {
			t.Fatalf("出力が違う: %q", output)
		}
	}
	session.mu.Lock()
	remaining := len(session.async)
	session.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("完了した実行状態が残っている: %d件", remaining)
	}
}

// GONAKO呼出/GONAKO実行の紹介サンプルが、実際にブリッジ経由で
// Go側の命令を呼び出せること（#63）。
func TestSampleWNako3BridgeDemoRunsThroughBridge(t *testing.T) {
	code, err := uiFS.ReadFile("ui/samples/16_ブラウザからGONAKOへアクセス(wnako3).nako3")
	if err != nil {
		t.Fatalf("サンプルを読み込めません: %v", err)
	}
	if !strings.Contains(string(code), "GONAKO呼出") || !strings.Contains(string(code), "GONAKO実行") {
		t.Fatal("サンプルはGONAKO呼出とGONAKO実行の両方を紹介すること")
	}

	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	if r := bridge.call("システム時間", `[]`); !r.OK {
		t.Fatalf("システム時間の呼び出しに失敗: %+v", r)
	}
	if r := bridge.call("ファイル列挙", `["."]`); !r.OK {
		t.Fatalf("ファイル列挙の呼び出しに失敗: %+v", r)
	}
	if r := bridge.call("保存", `["テスト内容","gonako-bridge-demo-test.txt"]`); !r.OK {
		t.Fatalf("保存の呼び出しに失敗: %+v", r)
	}
	defer bridge.call("ファイル削除", `["gonako-bridge-demo-test.txt"]`)
	if r := bridge.call("開", `["gonako-bridge-demo-test.txt"]`); !r.OK || string(r.Value) != `"テスト内容"` {
		t.Fatalf("保存したファイルを読み戻せない: %+v", r)
	}
}
