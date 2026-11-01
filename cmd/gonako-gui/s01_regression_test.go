package main

// S01: 静的な回帰検査。native/WebViewを起動せず、sourceの構造だけを確認する。
// 実行はsandbox条件が揃うまでENV_PENDING（go testは未実行）。

import (
	"os"
	"strings"
	"testing"
)

func readS01(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s を読めません: %v", path, err)
	}
	return string(data)
}

func TestS01ProtectedWindowFactoryIsTheOnlyBindPath(t *testing.T) {
	trusted := readS01(t, "trusted_window.go")
	if !strings.Contains(trusted, "webview.NewProtected") {
		t.Fatal("trusted_window.go が NewProtected を使っていません")
	}
	if !strings.Contains(trusted, "Origin: server.origin") {
		t.Fatal("trusted_window.go が own listener origin を TrustSpec へ渡していません")
	}
	if !strings.Contains(trusted, "webview.NewPlain") {
		t.Fatal("trusted_window.go に plain window factory がありません")
	}
	if !strings.Contains(trusted, "t.Window.Destroy()") ||
		!strings.Contains(trusted, "t.server.Close()") {
		t.Fatal("trusted_window.go が window と server を破棄していません")
	}
	if strings.Index(trusted, "t.Window.Destroy()") > strings.Index(trusted, "t.server.Close()") {
		t.Fatal("server を window より先に閉じています")
	}
}

func TestS01URLAndErrorWindowsHaveNoBinds(t *testing.T) {
	mainSrc := readS01(t, "main.go")
	if !strings.Contains(mainSrc, "newPlainWindow(*debugFlag, windowSettings)") {
		t.Fatal("-url が plain window ではありません")
	}
	if !strings.Contains(mainSrc, "if targetURL == \"\" {\n\t\tbindPrivilegedBridge(") {
		t.Fatal("bindPrivilegedBridge が targetURL==\"\" の保護windowに限定されていません")
	}
	if strings.Count(mainSrc, "webview.New(") != 0 {
		t.Fatal("main.go に素の webview.New( が残っています")
	}
}

func TestS01AllPrivilegedBindsTakeProtected(t *testing.T) {
	src := readS01(t, "main.go") +
		readS01(t, "bundled.go") +
		readS01(t, "window_run.go") +
		readS01(t, "wnako3_run.go") +
		readS01(t, "wnako3.go") +
		readS01(t, "drop_paths.go") +
		readS01(t, "window.go") +
		readS01(t, "window_theme.go")
	for _, signature := range []string{
		"func bindPrivilegedBridge(w *webview.Protected",
		"func bindAsyncRunAPI(w *webview.Protected",
		"func installWNako3(w *webview.Protected",
		"func bindFileDrop(w *webview.Protected",
		"func applyWindowSettings(w *webview.Protected",
		"func newNativeWindowController(w *webview.Protected",
	} {
		if !strings.Contains(src, signature) {
			t.Fatalf("保護windowの署名がありません: %s", signature)
		}
	}
}

func TestS01CloseAndBindFailureAreFailClosed(t *testing.T) {
	trusted := readS01(t, "trusted_window.go")
	if !strings.Contains(trusted, "if err := w.BindErr(); err != nil") {
		t.Fatal("初期Bind失敗でwindowを破棄していません")
	}
	for _, file := range []string{"main.go", "bundled.go", "window_run.go", "wnako3_run.go"} {
		if !strings.Contains(readS01(t, file), "BindErr()") {
			t.Fatalf("%s が BindErr を確認していません", file)
		}
	}
}

func TestS01NativeGtkRelayIsPrivateWorld(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	for _, required := range []string{
		"webview_set_trust_origin",
		"WEBVIEW_TRUST_TOP_DOCUMENT",
		"register_script_message_handler_in_world",
		"webkit_user_script_new_for_world",
		"WEBKIT_USER_CONTENT_INJECT_TOP_FRAME",
		"gonako-rpc",
		"CustomEvent",
		"on_message_from",
		"on_message_guarded",
		"install_trust_guard_impl",
		"webkit_policy_decision_use",
		"webkit_policy_decision_ignore",
		"webkit_response_policy_decision_get_response",
	} {
		if !strings.Contains(header, required) {
			t.Fatalf("webview.h に %q がありません", required)
		}
	}
	if strings.Contains(header, "script-message-received::external") &&
		strings.Contains(header, "register_script_message_handler(manager,\n                                                                \"external\")") {
		// 旧handlerは一般window用に残るが、保護windowではunregisterされる。
		if !strings.Contains(header, "unregister_script_message_handler(manager") {
			t.Fatal("保護windowで旧external handlerを解除していません")
		}
	}
}

func TestS01CocoaAndWindowsGuards(t *testing.T) {
	darwin := readS01(t, "drop_darwin.m")
	for _, required := range []string{
		"decidePolicyForNavigationAction",
		"decidePolicyForNavigationResponse",
		"WKNavigationActionPolicyCancel",
		"WKNavigationResponsePolicyCancel",
		"targetFrame == nil",
		"gonakoInstallNavigationGuard",
	} {
		if !strings.Contains(darwin, required) {
			t.Fatalf("drop_darwin.m に %q がありません", required)
		}
	}
	windows := readS01(t, "drop_windows.cc")
	for _, required := range []string{
		"get_Source",
		"gonakoOriginAllows",
		"gonakoInstallFileDropListener",
	} {
		if !strings.Contains(windows, required) {
			t.Fatalf("drop_windows.cc に %q がありません", required)
		}
	}
}

func TestS01ThemeBindOnlyForProtected(t *testing.T) {
	theme := readS01(t, "window_theme.go")
	if !strings.Contains(theme, "var _ themeWebView = (*webview.Protected)(nil)") {
		t.Fatal("theme同期のBindがProtectedに限定されていません")
	}
	window := readS01(t, "window.go")
	if !strings.Contains(window, "func applyPlainWindowSettings(w *webview.Plain") {
		t.Fatal("plain window用の設定関数がありません")
	}
	if strings.Contains(window, "applyWindowTheme(w *webview.Plain") {
		t.Fatal("plain windowにtheme Bindを登録しています")
	}
}
