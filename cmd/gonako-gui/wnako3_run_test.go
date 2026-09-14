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
