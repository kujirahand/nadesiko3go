package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestWNako3IsEmbedded(t *testing.T) {
	info := readWNako3Info()
	if info.Version == "" {
		t.Fatal("ui/wnako3/VERSION が無い。scripts/copy-nadesiko3.sh で取り込むこと")
	}
	for _, name := range []string{"wnako3.js", "plugin_turtle.js"} {
		if _, ok := wnako3AssetName(name); !ok {
			t.Fatalf("ui/wnako3/%s が埋め込まれていない", name)
		}
	}
}

func getSite(t *testing.T, handler http.Handler, target string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(body)
}

func TestSiteHandlerServesWNako3(t *testing.T) {
	site := fstest.MapFS{
		"index.html":           {Data: []byte("<p>hello</p>")},
		"own/plugin_turtle.js": {Data: []byte("// local turtle")},
	}
	handler := newSiteHandler(site)

	if code, body := getSite(t, handler, "/"); code != http.StatusOK || !strings.Contains(body, "hello") {
		t.Fatalf("フォルダのファイルを配信できない: %d %q", code, body)
	}
	if code, body := getSite(t, handler, "/__gonako/wnako3/wnako3.js"); code != http.StatusOK || !strings.Contains(body, "nako3") {
		t.Fatalf("同梱のwnako3.jsを配信できない: %d", code)
	}
	// フォルダに無ければ、どの階層でも同梱版を返す
	if code, body := getSite(t, handler, "/sub/plugin_turtle.js"); code != http.StatusOK || strings.Contains(body, "local turtle") {
		t.Fatalf("同梱のplugin_turtle.jsで補えない: %d", code)
	}
	// フォルダに実物があればそちらを優先する
	if code, body := getSite(t, handler, "/own/plugin_turtle.js"); code != http.StatusOK || body != "// local turtle" {
		t.Fatalf("フォルダのplugin_turtle.jsを優先していない: %d %q", code, body)
	}
	for _, target := range []string{"/__gonako/wnako3/VERSION", "/__gonako/wnako3/nothing.js", "/nothing.js"} {
		if code, _ := getSite(t, handler, target); code != http.StatusNotFound {
			t.Fatalf("%s は404であるべき: %d", target, code)
		}
	}
}

// DNS rebinding対策: ループバック以外のHostを名乗る要求は断る。
func TestLoopbackOnlyRejectsForeignHost(t *testing.T) {
	handler := loopbackOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "secret")
	}))
	for host, want := range map[string]int{
		"127.0.0.1:54321":      http.StatusOK,
		"localhost:54321":      http.StatusOK,
		"[::1]:54321":          http.StatusOK,
		"127.0.0.1":            http.StatusOK,
		"attacker.example:80":  http.StatusForbidden,
		"attacker.example":     http.StatusForbidden,
		"127.0.0.1.nip.io:123": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/data.txt", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("Host %s の応答が違う: got %d want %d", host, rec.Code, want)
		}
	}
}

// async で遅れて読み込まれたwnako3でも、?run があれば一度だけ実行する。
func TestLoaderRunsLateLoadedWNako3(t *testing.T) {
	loader := readUIAsset(t, "gonako-loader.js")
	for _, required := range []string{
		"window.addEventListener('load', () => {",
		"if (navigator.nako3.checkScriptTagParam()) navigator.nako3.runNakoScript();",
		"}, { once: true });",
	} {
		if !strings.Contains(loader, required) {
			t.Fatalf("ローダーに %q が無い", required)
		}
	}
}

func TestWNako3ConfigFromIndexJSON(t *testing.T) {
	if !wnako3ConfigFromIndexJSON([]byte(`{"タイトル":"見本","wnako3":true}`)) {
		t.Fatal(`"wnako3": true を読めない`)
	}
	for _, data := range []string{`{"タイトル":"見本"}`, `{"wnako3":false}`, `not json`} {
		if wnako3ConfigFromIndexJSON([]byte(data)) {
			t.Fatalf("%s でwnako3を有効にしてはいけない", data)
		}
	}
}

func TestWNako3InitScriptCarriesConfig(t *testing.T) {
	script := wnako3InitScript(wnako3PageConfig{WNako3: true, WNako3Version: "3.8.1", GonakoVersion: "9.9.9"})
	for _, required := range []string{
		`window.__gonakoConfig = {"wnako3":true,"wnako3Version":"3.8.1","gonakoVersion":"9.9.9"};`,
		"window.gonako = {",
		"window.__gonakoCommandDone",
		"window.startNakoCommand(",
		"addPluginObject('PluginGonako'",
		"'GONAKO関数実行'",
		"'GONAKO実行'",
		"navigator.nako3.runNakoScript()",
		"/__gonako/wnako3/",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("ローダーに %q が無い", required)
		}
	}
}

// macOSのWKWebViewはalert/confirm/promptを実装していないため、wnako3の
// 『言』『尋』『文字尋』『二択』(window.alert等を直接呼ぶ)は動かない。
// PluginGonakoが同名で上書きし、Go側(gonako)を呼ぶことを確かめる（#63）。
func TestLoaderOverridesBrowserDialogCommands(t *testing.T) {
	loader := readUIAsset(t, "gonako-loader.js")
	for _, required := range []string{
		"'言': {",
		"'尋': {",
		"'文字尋': {",
		"'二択': {",
		"await call('言', s);",
		"return call('尋', s);",
		"return call('文字尋', s);",
		"return call('二択', s);",
		"function showGonakoDialog(",
		"window.__gonakoBridgeDialog = async function",
		"window.resolveGonakoDialog",
	} {
		if !strings.Contains(loader, required) {
			t.Fatalf("ローダーに %q が無い", required)
		}
	}
}

// IME変換確定のEnterでダイアログを閉じてしまわないこと（Devinレビュー指摘への対応）。
func TestLoaderDialogGuardsIMEComposition(t *testing.T) {
	loader := readUIAsset(t, "gonako-loader.js")
	for _, required := range []string{
		"compositionstart", "compositionend", "isIMEKeyEvent", "keyCode === 229", "if (isIMEKeyEvent(e)) return;",
	} {
		if !strings.Contains(loader, required) {
			t.Fatalf("ローダーに %q が無い", required)
		}
	}
}

func TestCommandBridgeCallsGoCommand(t *testing.T) {
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})

	result := bridge.call("文字数", `["あいう"]`)
	if !result.OK || string(result.Value) != "3" {
		t.Fatalf("文字数の呼び出し結果が違う: %+v", result)
	}
	result = bridge.call("表示", `["こんにちは"]`)
	if !result.OK || result.Output != "こんにちは\n" {
		t.Fatalf("表示の出力を返していない: %+v", result)
	}
	if result = bridge.call("存在しない命令", `[]`); result.OK || !strings.Contains(result.Error, "存在しない命令") {
		t.Fatalf("存在しない命令がエラーにならない: %+v", result)
	}
	if result = bridge.call("文字数", `{"a":1}`); result.OK {
		t.Fatalf("配列でない引数がエラーにならない: %+v", result)
	}
}

func TestCommandBridgeStartReportsThroughEval(t *testing.T) {
	scripts := make(chan string, 1)
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(js string) { scripts <- js })
	id := bridge.start("文字数", `["abc"]`)
	js := <-scripts

	prefix := "window.__gonakoCommandDone && window.__gonakoCommandDone("
	if !strings.HasPrefix(js, prefix) {
		t.Fatalf("Evalする文が違う: %s", js)
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(js, prefix), ")")
	idText, literal, ok := strings.Cut(rest, ", ")
	if !ok || idText != "1" || id != 1 {
		t.Fatalf("呼び出しIDが違う: id=%d js=%s", id, js)
	}
	var raw string
	if err := json.Unmarshal([]byte(literal), &raw); err != nil {
		t.Fatalf("結果が文字列リテラルでない: %v", err)
	}
	var result BridgeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil || !result.OK || string(result.Value) != "3" {
		t.Fatalf("結果が違う: %s (%v)", raw, err)
	}
}

// 『言』『尋』『二択』はダイアログを出すためctx.ShowDialogを呼ぶ。
// commandBridge.showDialogがJS側へEvalで届け、resolveDialogが
// resolveGonakoDialog（Bind）経由の応答で待ちを解くこと（#63）。
func TestCommandBridgeShowDialogRoundTrips(t *testing.T) {
	var evaluated []string
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(js string) {
		evaluated = append(evaluated, js)
	})

	done := make(chan BridgeResult, 1)
	go func() { done <- bridge.call("言", `["こんにちは"]`) }()

	deadline := time.Now().Add(2 * time.Second)
	for len(evaluated) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(evaluated) != 1 || !strings.Contains(evaluated[0], "__gonakoBridgeDialog") {
		t.Fatalf("ダイアログ表示のEvalが呼ばれていない: %v", evaluated)
	}
	if !strings.Contains(evaluated[0], `"alert"`) || !strings.Contains(evaluated[0], "こんにちは") {
		t.Fatalf("ダイアログの種類/メッセージが違う: %s", evaluated[0])
	}

	if !bridge.resolveDialog(1, "", true) {
		t.Fatal("resolveDialogが応答できない")
	}
	select {
	case result := <-done:
		if !result.OK {
			t.Fatalf("言の呼び出しが失敗: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ダイアログ応答後もcallが完了しない")
	}

	// 存在しないIDへの応答はfalseになること（二重応答対策）。
	if bridge.resolveDialog(999, "", true) {
		t.Fatal("存在しないダイアログIDへの応答がtrueになっている")
	}
}

// ページ遷移などで応答が二度と来ない場合でも、タイムアウトでb.muを解放し、
// 以後の呼び出しが永久に止まらないこと（Devinレビュー指摘への対応）。
func TestCommandBridgeShowDialogTimesOut(t *testing.T) {
	orig := bridgeDialogTimeout
	bridgeDialogTimeout = 20 * time.Millisecond
	defer func() { bridgeDialogTimeout = orig }()

	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})

	result := bridge.call("言", `["応答が来ない想定"]`)
	if result.OK || !strings.Contains(result.Error, "ダイアログの応答がありません") {
		t.Fatalf("タイムアウトがエラーになっていない: %+v", result)
	}

	// 待っていた呼び出しがb.muを解放しているので、次の呼び出しはすぐ通ること。
	done := make(chan BridgeResult, 1)
	go func() { done <- bridge.call("文字数", `["abc"]`) }()
	select {
	case next := <-done:
		if !next.OK || string(next.Value) != "3" {
			t.Fatalf("タイムアウト後の呼び出し結果が違う: %+v", next)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("タイムアウト後もb.muが解放されず、次の呼び出しが進まない")
	}

	// タイムアウト後に遅れて届いた応答は、該当なしとして無視されること。
	if bridge.resolveDialog(1, "遅延応答", true) {
		t.Fatal("タイムアウト済みのダイアログへの遅延応答がtrueになっている")
	}
}
