package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// GONAKO関数実行/GONAKO実行の紹介サンプルが、実際にブリッジ経由で
// Go側の命令を呼び出せること（#63）。
func TestSampleWNako3BridgeDemoRunsThroughBridge(t *testing.T) {
	code, err := uiFS.ReadFile("ui/samples/16_ブラウザからGONAKOへアクセス(wnako3).nako3")
	if err != nil {
		t.Fatalf("サンプルを読み込めません: %v", err)
	}
	if !strings.Contains(string(code), "GONAKO関数実行") || !strings.Contains(string(code), "GONAKO実行") {
		t.Fatal("サンプルはGONAKO関数実行とGONAKO実行の両方を紹介すること")
	}
	// 固定名のファイルをいきなり保存・削除すると、利用者が同名ファイルを
	// 作業フォルダに置いていた場合に消してしまう。存在確認してから使うこと。
	if !strings.Contains(string(code), `「存在」を[一時ファイル名]でGONAKO関数実行`) {
		t.Fatal("サンプルは保存前にGONAKO関数実行で存在確認すること")
	}

	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	if r := bridge.call("システム時間", `[]`); !r.OK {
		t.Fatalf("システム時間の呼び出しに失敗: %+v", r)
	}
	if r := bridge.call("ファイル列挙", `["."]`); !r.OK {
		t.Fatalf("ファイル列挙の呼び出しに失敗: %+v", r)
	}
	if r := bridge.call("存在", `["gonako-bridge-demo-test.txt"]`); !r.OK || string(r.Value) != "false" {
		t.Fatalf("存在しないファイルの確認に失敗: %+v", r)
	}
	if r := bridge.call("保存", `["テスト内容","gonako-bridge-demo-test.txt"]`); !r.OK {
		t.Fatalf("保存の呼び出しに失敗: %+v", r)
	}
	defer bridge.call("ファイル削除", `["gonako-bridge-demo-test.txt"]`)
	if r := bridge.call("存在", `["gonako-bridge-demo-test.txt"]`); !r.OK || string(r.Value) != "true" {
		t.Fatalf("保存後の存在確認に失敗: %+v", r)
	}
	if r := bridge.call("開", `["gonako-bridge-demo-test.txt"]`); !r.OK || string(r.Value) != `"テスト内容"` {
		t.Fatalf("保存したファイルを読み戻せない: %+v", r)
	}
}

// 同名ファイルが既にある場合、サンプルはそれを上書き・削除せず、
// 番号を振った別名を使うこと（Devinレビュー指摘への対応）。
func TestSampleWNako3BridgeDemoAvoidsOverwritingExistingFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("作業フォルダを変更できません: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	const existing = "gonako-bridge-sample.txt"
	if err := os.WriteFile(existing, []byte("大事なユーザーのファイル"), 0o644); err != nil {
		t.Fatalf("下準備に失敗: %v", err)
	}

	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	name := existing
	for i := 0; ; {
		r := bridge.call("存在", fmt.Sprintf(`[%q]`, name))
		if !r.OK {
			t.Fatalf("存在確認に失敗: %+v", r)
		}
		if string(r.Value) == "false" {
			break
		}
		i++
		name = fmt.Sprintf("gonako-bridge-sample-%d.txt", i)
	}
	if name == existing {
		t.Fatal("既存ファイルと同じ名前を使ってしまっている")
	}

	if r := bridge.call("保存", fmt.Sprintf(`["新しい内容",%q]`, name)); !r.OK {
		t.Fatalf("保存に失敗: %+v", r)
	}
	if r := bridge.call("ファイル削除", fmt.Sprintf(`[%q]`, name)); !r.OK {
		t.Fatalf("削除に失敗: %+v", r)
	}

	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "大事なユーザーのファイル" {
		t.Fatalf("既存ファイルが上書き・削除された: err=%v data=%q", err, data)
	}
}

// エディタに実行モードの説明を表示する[?]ボタンがあること（#104）。
func TestEditorHasRunModeHelpButton(t *testing.T) {
	index := readUIAsset(t, "index.html")
	if !strings.Contains(index, `<button id="btn-run-mode-help"`) {
		t.Fatal("index.html に実行モードの[?]ボタンが無い")
	}
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"const btnRunModeHelp = document.getElementById('btn-run-mode-help');",
		"function openRunModeHelpModal()",
		"btnRunModeHelp.addEventListener('click', openRunModeHelpModal);",
		"const runModeDescriptions = {",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js に %q が無い", required)
		}
	}
	// 実行モードのセレクタにある種類は、すべて説明も持つこと。
	for _, mode := range []string{"window", "newwindow", "cli", "wnako3"} {
		if !strings.Contains(app, mode+": '") {
			t.Fatalf("実行モード %q の説明が無い可能性がある", mode)
		}
	}
}
