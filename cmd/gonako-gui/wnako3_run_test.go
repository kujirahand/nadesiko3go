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
