package main

// S01: 回帰検査。native/WebViewを起動せず、sourceの構造と、GUIを要しない
// 純粋なGo部分（所有serverの失効、.appの置換）だけを確認する。

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
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
	// SR03: guardはvendor所有で、appのdrop_darwin.mには無い。
	darwin := readS01(t, "../../third_party/webview_go/trust_darwin.m")
	for _, required := range []string{
		"decidePolicyForNavigationAction",
		"decidePolicyForNavigationResponse",
		"WKNavigationActionPolicyCancel",
		"WKNavigationResponsePolicyCancel",
		"targetFrame == nil",
		"gonakoInstallNavigationGuard",
		"gonakoNativeTrustAllowsURL",
	} {
		if !strings.Contains(darwin, required) {
			t.Fatalf("trust_darwin.m に %q がありません", required)
		}
	}
	appDarwin := readS01(t, "drop_darwin.m")
	if strings.Contains(appDarwin, "gonakoInstallNavigationGuard") {
		t.Fatal("appのdrop_darwin.mにguard定義が残っています")
	}
	windows := readS01(t, "drop_windows.cc")
	for _, required := range []string{
		"get_Source",
		"gonakoOriginAllows",
		"gonakoInstallFileDropListener",
		"senderText",
	} {
		if !strings.Contains(windows, required) {
			t.Fatalf("drop_windows.cc に %q がありません", required)
		}
	}
	// token-boundaryで検査する。senderText は正しい名前であり、
	// 部分文字列 "std::string sender" だけで誤検知しない。
	senderDecl := regexp.MustCompile(`\bstd::string\s+sender\b`)
	if senderDecl.MatchString(windows) {
		t.Fatal("drop_windows.cc に sender shadow が残っています")
	}
	if !strings.Contains(readS01(t, "drop_windows.go"), "#include <stdlib.h>") {
		t.Fatal("drop_windows.go の cgo preamble に stdlib.h がありません")
	}
	if !strings.Contains(readS01(t, "drop_windows.go"), "C.free") {
		t.Fatal("drop_windows.go に C.free がありません")
	}
}

// guardはengineの生ポインタを持つため、engineの破棄時にviewが生存しているうちに
// delegateを外す。guardの寿命をengineより短くする。
func TestS01CocoaGuardDetachesBeforeEngineDies(t *testing.T) {
	darwin := readS01(t, "../../third_party/webview_go/trust_darwin.m")
	for _, required := range []string{
		"gonakoDetachNavigationGuard",
		"_engine = NULL",
		"view.navigationDelegate = nil",
	} {
		if !strings.Contains(darwin, required) {
			t.Fatalf("trust_darwin.m に %q がありません", required)
		}
	}
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	if !strings.Contains(header, "gonakoDetachNavigationGuard((void *)m_webview)") {
		t.Fatal("engineの破棄時にguardを外していません")
	}
	detachIdx := strings.Index(header, "gonakoDetachNavigationGuard((void *)m_webview)")
	releaseIdx := strings.Index(header[detachIdx:], `objc::msg_send<void>(m_webview, "release"_sel)`)
	if detachIdx < 0 || releaseIdx < 0 {
		t.Fatal("viewのrelease前にguardを外す順序が見つかりません")
	}
	// Low: コメントのファイル名が旧名（drop_darwin.m）のままになっていない。
	if strings.Contains(header, "drop_darwin.m") {
		t.Fatal("webview.h のコメントが旧ファイル名 drop_darwin.m のままです")
	}
}

// Bindの失敗はプログラムを開始する前に返す。失敗時に副作用のある実行を始めない。
func TestS01BindFailurePrecedesProgramStart(t *testing.T) {
	for _, file := range []string{"bundled.go", "window_run.go"} {
		src := readS01(t, file)
		bindErr := strings.Index(src, "tw.BindErr()")
		start := strings.Index(src, "runID := session.start")
		if bindErr < 0 || start < 0 || bindErr > start {
			t.Fatalf("%s: Bind失敗の確認がプログラム開始より後ろにあります", file)
		}
		// 生成HTMLは共有変数ではなく、同期付きのhandlerへ渡す。
		if !strings.Contains(src, "page := &ownedPageHandler{}") ||
			!strings.Contains(src, "page.SetHTML(") {
			t.Fatalf("%s: HTMLの受け渡しが同期付きhandlerになっていません", file)
		}
	}
}

func TestS01OwnedServerFailureRevocation(t *testing.T) {
	trusted := readS01(t, "trusted_window.go")
	for _, required := range []string{
		"trustRevokingListener",
		"SetRevoker",
		"RevokeAndDetach",
		"LeaseValid",
		"revokeOnce",
	} {
		if !strings.Contains(trusted, required) {
			t.Fatalf("trusted_window.go に %q がありません", required)
		}
	}
	if strings.Contains(trusted, "_ = server.Serve(listener)") {
		t.Fatal("Serve error を捨てる旧起動が残っています")
	}
	if !strings.Contains(readS01(t, "../../third_party/webview_go/webview.go"), "RevokeTrust") {
		t.Fatal("Protected.RevokeTrust がありません")
	}
	if !strings.Contains(readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h"), "webview_revoke_trust") {
		t.Fatal("native webview_revoke_trust がありません")
	}
}

func TestS01CocoaUsesSecurityOrigin(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	for _, required := range []string{
		"securityOrigin",
		"allow_sender_origin",
		"sender_origin_t",
		"trust_configured",
	} {
		if !strings.Contains(header, required) {
			t.Fatalf("webview.h に %q がありません", required)
		}
	}
	if strings.Contains(header, "frameInfo.request.URL") {
		t.Fatal("Cocoa senderがrequest.URLのままです")
	}
}

func TestS01GtkResponseStrict(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	if strings.Contains(header, "is_main_frame_main_resource") {
		t.Fatal("GTK responseがmainResource分岐に依存しています")
	}
	if strings.Contains(header, "if (mainResource)") {
		t.Fatal("GTK responseにmainResource分岐が残っています")
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

// 指摘1: WebView2のdrop listenerはoriginだけでなくnative trustの現在値も見る。
// 失効後は、同じoriginをsourceとするキュー済みメッセージでもパスを返さない。
// 照会は cmd/gonako-gui の Go 関数（//export）を経由し、他packageのCシンボルを
// 直接参照しない（cgoはpackage単位でCオブジェクトを作るため）。
func TestS01WindowsDropListenerHonorsRevokedTrust(t *testing.T) {
	windows := readS01(t, "drop_windows.cc")
	for _, required := range []string{
		"gonakoTrustInstalled",
		"trustHandle",
	} {
		if !strings.Contains(windows, required) {
			t.Fatalf("drop_windows.cc に %q がありません", required)
		}
	}
	// 他packageのCシンボルを直接参照するとリンク順で未解決になりうる。
	if strings.Contains(windows, "webview_trust_installed") {
		t.Fatal("drop_windows.cc が webview_go のCシンボルを直接参照しています")
	}
	// trustの確認はorigin判定より前に行う。失効後はoriginが一致してもdenyする。
	trustIdx := strings.Index(windows, "gonakoTrustInstalled(trustHandle)")
	originIdx := strings.Index(windows, "gonakoOriginAllows(origin, senderText)")
	if trustIdx < 0 || originIdx < 0 || trustIdx > originIdx {
		t.Fatal("drop listener が origin 判定より前に trust を確認していません")
	}
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	if !strings.Contains(header, "webview_trust_installed") {
		t.Fatal("webview.h に webview_trust_installed がありません")
	}
	webviewGo := readS01(t, "../../third_party/webview_go/webview.go")
	if !strings.Contains(webviewGo, "func (p *Protected) NativeTrustHandle()") {
		t.Fatal("Protected.NativeTrustHandle がありません")
	}
	// vendor側のGo APIが、自分のpackageのCシンボルだけを使って照会する。
	if !strings.Contains(webviewGo, "func TrustInstalled(handle unsafe.Pointer) bool") {
		t.Fatal("webview.TrustInstalled がありません")
	}
	if !strings.Contains(webviewGo, "C.webview_trust_installed(") {
		t.Fatal("webview.TrustInstalled が webview_trust_installed を呼んでいません")
	}
	dropGo := readS01(t, "drop_windows.go")
	if !strings.Contains(dropGo, "w.NativeTrustHandle()") {
		t.Fatal("drop_windows.go が trust handle を listener へ渡していません")
	}
	if !strings.Contains(dropGo, "//export gonakoTrustInstalled") {
		t.Fatal("drop_windows.go に gonakoTrustInstalled の //export がありません")
	}
	if !strings.Contains(dropGo, "webview.TrustInstalled(handle)") {
		t.Fatal("gonakoTrustInstalled が webview.TrustInstalled を呼んでいません")
	}
}

// 指摘2: Serveの異常終了でも、listenerのcloseより前にnative trustを失効させる。
// wrapperではなく下のlistenerを閉じ、Serve自身のdeferred Closeだけを経路にする。
func TestS01ServeFailureRevokesTrust(t *testing.T) {
	revoked := make(chan struct{}, 1)
	s, err := startOwnedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if err != nil {
		t.Fatalf("server を起動できません: %v", err)
	}
	defer s.Close()
	s.SetRevoker(func() {
		select {
		case revoked <- struct{}{}:
		default:
		}
	})
	raw := s.listener.(*trustRevokingListener).Listener
	if err := raw.Close(); err != nil {
		t.Fatalf("listener を閉じられません: %v", err)
	}
	select {
	case <-revoked:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve の異常終了で native trust が失効しませんでした")
	}
	if s.LeaseValid() {
		t.Fatal("失効後も lease が有効です")
	}
}

// 指摘3: guard hookの既定実装はfalse。3 backendだけが明示的に実装する。
func TestS01UnimplementedTrustGuardFailsClosed(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	re := regexp.MustCompile(`virtual bool install_trust_guard_impl\(\) \{ return (true|false); \}`)
	m := re.FindStringSubmatch(header)
	if m == nil {
		t.Fatal("install_trust_guard_impl の既定実装が見つかりません")
	}
	if m[1] != "false" {
		t.Fatal("install_trust_guard_impl の既定実装が false ではありません")
	}
	if got := strings.Count(header, "bool install_trust_guard_impl() override"); got != 3 {
		t.Fatalf("guard を実装した backend が3つではありません: %d", got)
	}
}

// 指摘4: WebKitGTKのC APIは (manager, name, world_name) の3引数で、GErrorは取らない。
// Linux実機でコンパイルできないため、呼び出しの引数個数だけを静的に固定する。
func TestS01GtkRegisterHandlerCallArity(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	start := strings.Index(header, "webkit_user_content_manager_register_script_message_handler_in_world(")
	if start < 0 {
		t.Fatal("GTK の handler 登録呼び出しがありません")
	}
	rest := header[start:]
	end := strings.Index(rest, "))")
	if end < 0 {
		t.Fatal("呼び出しの終端が見つかりません")
	}
	call := rest[:end]
	if got := strings.Count(call, ","); got != 2 {
		t.Fatalf("GTK handler 登録の引数が3個ではありません: カンマ%d個", got)
	}
	if strings.Contains(call, "nullptr") {
		t.Fatal("GTK handler 登録に不要な第4引数があります")
	}
}

// 指摘6: sender originの判定はmain frameとsub frameで同一規則である。
// frameの別を判定入力にしないこと（引数ごと削除したこと）と、コメントだけの
// 空ブロックが戻らないことを固定する。
func TestS01AllowSenderOriginIgnoresFrameKind(t *testing.T) {
	header := readS01(t, "../../third_party/webview_go/libs/webview/include/webview.h")
	if !strings.Contains(header, "bool allow_sender_origin(const sender_origin_t &origin) const {") {
		t.Fatal("allow_sender_origin が frame の別を引数に取っています")
	}
	if !strings.Contains(header, "void on_message_from(const std::string &msg, const sender_origin_t &origin) {") {
		t.Fatal("on_message_from が frame の別を引数に取っています")
	}
	// 旧引数がどこかに残っていれば、空ブロックか未使用引数が復活している。
	if strings.Contains(header, "is_main_frame") {
		t.Fatal("webview.h に未使用の is_main_frame が残っています")
	}
	if strings.Contains(header, "isMainFrame") {
		t.Fatal("Cocoa が frameInfo.isMainFrame を読み続けています")
	}
	// 条件だけ書いて本体がコメントのみのブロックを、関数本体に限定して検出する。
	start := strings.Index(header, "bool allow_sender_origin(const sender_origin_t &origin) const {")
	end := strings.Index(header[start:], "\n  }\n")
	if end < 0 {
		t.Fatal("allow_sender_origin の終端が見つかりません")
	}
	body := header[start : start+end]
	commentOnly := regexp.MustCompile(`if \([^\n]*\) \{\s*(?://[^\n]*\s*)*\}`)
	if commentOnly.MatchString(body) {
		t.Fatal("allow_sender_origin に本体がコメントだけの空ブロックがあります")
	}
	if !strings.Contains(body, "origin.opaque") {
		t.Fatal("allow_sender_origin が opaque origin を拒否していません")
	}
}

