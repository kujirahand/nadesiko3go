package nodelib_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// ajaxCapture は mock Transport で捕捉したリクエスト。
// 外部へは一切通信しない（http.DefaultTransport を差し替える）。
type ajaxCapture struct {
	count  int
	method string
	url    string
	header http.Header
	host   string
	body   string
}

// installAjaxCapture は応答 body を返す mock Transport を差し替え、
// 捕捉先を返す。http.DefaultTransport を差し替えるため、このファイルの
// テストで t.Parallel() は使わない。
func installAjaxCapture(t *testing.T, responseBody string) *ajaxCapture {
	t.Helper()
	captured := &ajaxCapture{}
	original := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured.count++
		captured.method = req.Method
		captured.url = req.URL.String()
		captured.header = req.Header.Clone()
		captured.host = req.Host
		if req.Body != nil {
			data, _ := io.ReadAll(req.Body)
			captured.body = string(data)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	return captured
}

// runAjaxCode はプログラムを実行し、出力とエラーを返す（runIn と違い
// エラーになるプログラムも検証できる）。
func runAjaxCode(t *testing.T, code string) (string, error) {
	t.Helper()
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err := vm.RunProgram(code, "main.nako3", host)
	return strings.TrimRight(out.String(), "\n"), err
}

// TestAjaxOptionsHeadersReflected は Issue #331 の受け入れ条件。
// 『AJAXオプション設定』で設定した Authorization が、AJAXオプション対応
// 命令の実際のリクエストで mock Transport から観測できること。
func TestAjaxOptionsHeadersReflected(t *testing.T) {
	const setOpt = `{"headers":{"Authorization":"Bearer <token>","X-Custom":"abc"}}にAJAXオプション設定` + "\n"

	programs := map[string]struct {
		code     string
		respBody string
	}{
		"AJAXテキスト取得":  {code: `「http://audit.invalid/」からAJAXテキスト取得して表示`, respBody: "ok"},
		"AJAX内容取得":    {code: `「http://audit.invalid/」からAJAX内容取得して表示`, respBody: "ok"},
		"AJAX受信":      {code: `「http://audit.invalid/」からAJAX受信して表示`, respBody: "ok"},
		"AJAX_JSON取得": {code: `J = 「http://audit.invalid/」からAJAX_JSON取得`, respBody: `{"ok":true}`},
		"AJAXバイナリ取得":  {code: `B = 「http://audit.invalid/」からAJAXバイナリ取得`, respBody: "ok"},
		"AJAX保障送信":    {code: `「http://audit.invalid/」へAJAX保障送信して表示`, respBody: "ok"},
		"HTTP保障取得":    {code: `「http://audit.invalid/」のHTTP保障取得して表示`, respBody: "ok"},
		"GET保障送信":     {code: `「http://audit.invalid/」へGET保障送信して表示`, respBody: "ok"},
		"AJAX送信時": {code: `●CB
　「受信: {対象}」と表示
ここまで
「http://audit.invalid/」へ「CB」でAJAX送信時`, respBody: "ok"},
		"AJAX受信時": {code: `●CB
　「受信: {対象}」と表示
ここまで
「http://audit.invalid/」へ「CB」でAJAX受信時`, respBody: "ok"},
		"GET送信時": {code: `●CB
　「受信: {対象}」と表示
ここまで
「http://audit.invalid/」へ「CB」でGET送信時`, respBody: "ok"},
	}
	for name, tc := range programs {
		t.Run(name, func(t *testing.T) {
			captured := installAjaxCapture(t, tc.respBody)
			if _, err := runAjaxCode(t, setOpt+tc.code); err != nil {
				t.Fatalf("%s: %v", tc.code, err)
			}
			if captured.count != 1 {
				t.Fatalf("リクエスト数 = %d, want 1", captured.count)
			}
			if got := captured.header.Get("Authorization"); got != "Bearer <token>" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer <token>")
			}
			if got := captured.header.Get("X-Custom"); got != "abc" {
				t.Errorf("X-Custom = %q, want %q", got, "abc")
			}
			if captured.method != http.MethodGet {
				t.Errorf("method = %q, want GET", captured.method)
			}
		})
	}
}

// TestAjaxOptionsDefaults はオプション未指定・空文字列・空辞書のときに
// 従来どおり GET・追加ヘッダーなしになること。
func TestAjaxOptionsDefaults(t *testing.T) {
	for name, prefix := range map[string]string{
		"未指定":  "",
		"空文字列": "\"\"にAJAXオプション設定\n",
		"空辞書":  "{}にAJAXオプション設定\n",
	} {
		t.Run(name, func(t *testing.T) {
			captured := installAjaxCapture(t, "ok")
			code := prefix + `「http://audit.invalid/」からAJAXテキスト取得して表示`
			if _, err := runAjaxCode(t, code); err != nil {
				t.Fatalf("%v", err)
			}
			if captured.method != http.MethodGet {
				t.Errorf("method = %q, want GET", captured.method)
			}
			if got := captured.header.Get("Authorization"); got != "" {
				t.Errorf("Authorization = %q, want 空", got)
			}
			if captured.body != "" {
				t.Errorf("body = %q, want 空", captured.body)
			}
		})
	}
}

// TestAjaxOptionsDirectAssignment は『AJAXオプション』へ直接代入した場合も
// 反映され、設定命令より後の代入が優先されること。
func TestAjaxOptionsDirectAssignment(t *testing.T) {
	captured := installAjaxCapture(t, "ok")
	code := `{"headers":{"Authorization":"Bearer first"}}にAJAXオプション設定
{"headers":{"Authorization":"Bearer second"}}をAJAXオプションに代入
「http://audit.invalid/」からAJAXテキスト取得して表示`
	if _, err := runAjaxCode(t, code); err != nil {
		t.Fatal(err)
	}
	if got := captured.header.Get("Authorization"); got != "Bearer second" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer second")
	}
}

// TestAjaxOptionsMethodAndBody は method と body の対応範囲。
// method は小文字で書いても大文字化され、body は文字列がそのまま送られる。
func TestAjaxOptionsMethodAndBody(t *testing.T) {
	captured := installAjaxCapture(t, "ok")
	code := `{"method":"post","body":"hello body","headers":{"Content-Type":"text/plain","Authorization":"Bearer <token>"}}にAJAXオプション設定
「http://audit.invalid/」からAJAXテキスト取得して表示`
	if _, err := runAjaxCode(t, code); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured.method)
	}
	if captured.body != "hello body" {
		t.Errorf("body = %q, want %q", captured.body, "hello body")
	}
	if got := captured.header.Get("Content-Type"); got != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
	if got := captured.header.Get("Authorization"); got != "Bearer <token>" {
		t.Errorf("Authorization = %q", got)
	}
}

// TestAjaxOptionsErrors は未対応項目や不正な値を無言で捨てず、
// リクエストを送る前にエラーにすること。
func TestAjaxOptionsErrors(t *testing.T) {
	cases := []struct {
		name    string
		setOpt  string
		wantErr string
	}{
		{"未対応キー", `{"timeout":30}にAJAXオプション設定`, "項目『timeout』には対応していません"},
		{"辞書でないオプション", `"dummy"にAJAXオプション設定`, "AJAXオプションには辞書を指定してください"},
		{"headersが辞書でない", `{"headers":"x"}にAJAXオプション設定`, "headersには辞書を指定してください"},
		{"methodが文字列でない", `{"method":123}にAJAXオプション設定`, "methodには文字列を指定してください"},
		{"bodyが文字列でない", `{"method":"POST","body":123}にAJAXオプション設定`, "bodyには文字列を指定してください"},
		{"GETでbody", `{"body":"x"}にAJAXオプション設定`, "bodyはmethodがGET・HEADのとき指定できません"},
		{"HEADでbody", `{"method":"HEAD","body":"x"}にAJAXオプション設定`, "bodyはmethodがGET・HEADのとき指定できません"},
		{"methodが不正", `{"method":"PO ST"}にAJAXオプション設定`, "method『PO ST』が不正です"},
		{"ヘッダー名が不正", `{"headers":{"X Test":"v"}}にAJAXオプション設定`, "ヘッダー名『X Test』が不正です"},
		{"ヘッダー値が辞書", `{"headers":{"X-Test":{"a":1}}}にAJAXオプション設定`, "ヘッダー『X-Test』の値には文字列を指定してください"},
		{"項目の重複", `{"method":"GET","Method":"POST"}にAJAXオプション設定`, "項目『Method』が重複しています"},
		{"ヘッダーの重複", `{"headers":{"X-Test":"a","x-test":"b"}}にAJAXオプション設定`, "ヘッダー『x-test』が重複しています"},
		{"Content-Length指定", `{"headers":{"Content-Length":"5"}}にAJAXオプション設定`, "ヘッダー『Content-Length』は指定できません"},
		{"ヘッダー値に制御文字", "H = {}\nH[\"X-Test\"] = \"a\" & (1のCHR) & \"b\"\nO = {}\nO[\"headers\"] = H\nOにAJAXオプション設定", "ヘッダー『X-Test』の値が不正です"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captured := installAjaxCapture(t, "ok")
			code := tc.setOpt + "\n" + `「http://audit.invalid/」からAJAXテキスト取得して表示`
			_, err := runAjaxCode(t, code)
			if err == nil {
				t.Fatalf("エラーにならなかった: %s", tc.setOpt)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("エラー = %v, want 部分文字列 %q", err, tc.wantErr)
			}
			if captured.count != 0 {
				t.Errorf("エラー時にリクエストが送られた: count=%d", captured.count)
			}
		})
	}
}

// TestAjaxOptionsNotAppliedToPostCommands は専用POST命令へ AJAXオプションが
// 波及しないこと。オプション側の method/body は、もし読めばエラーになる
// 組み合わせ（GET+body）にしておくので、読んだ場合は送信自体が失敗する。
func TestAjaxOptionsNotAppliedToPostCommands(t *testing.T) {
	const setOpt = `{"method":"GET","body":"must-not-be-used","headers":{"Authorization":"Bearer <token>"}}にAJAXオプション設定` + "\n"
	const params = "Params = {}\nParams[\"name\"] = \"test\"\n"

	programs := map[string]string{
		"POST送信":       setOpt + params + `Ans = 「http://audit.invalid/」へParamsをPOST送信`,
		"POST保障送信":     setOpt + params + `Ans = 「http://audit.invalid/」へParamsをPOST保障送信`,
		"POSTフォーム送信":   setOpt + params + `Ans = 「http://audit.invalid/」へParamsをPOSTフォーム送信`,
		"POSTフォーム保障送信": setOpt + params + `Ans = 「http://audit.invalid/」へParamsをPOSTフォーム保障送信`,
		"POST送信時": setOpt + params + `●CB
　「受信: {対象}」と表示
ここまで
「CB」で「http://audit.invalid/」へParamsをPOST送信時`,
		"POSTフォーム送信時": setOpt + params + `●CB
　「受信: {対象}」と表示
ここまで
「CB」で「http://audit.invalid/」へParamsをPOSTフォーム送信時`,
		"DISCORD送信": setOpt + `「http://audit.invalid/」へ「hello」をDISCORD送信`,
	}
	for name, code := range programs {
		t.Run(name, func(t *testing.T) {
			captured := installAjaxCapture(t, "ok")
			if _, err := runAjaxCode(t, code); err != nil {
				t.Fatalf("%v", err)
			}
			if captured.count != 1 {
				t.Fatalf("リクエスト数 = %d, want 1", captured.count)
			}
			if captured.method != http.MethodPost {
				t.Errorf("method = %q, want POST（専用命令の既定）", captured.method)
			}
			if got := captured.header.Get("Authorization"); got != "" {
				t.Errorf("Authorization が波及した: %q", got)
			}
			if captured.body == "must-not-be-used" {
				t.Errorf("オプションの body が波及した")
			}
		})
	}
}

// TestAjaxOptionsErrorsOtherCommands はテキスト取得以外の対応命令でも、
// 未対応項目がリクエスト送信前にエラーになること。『AJAX送信時』は
// AJAX:ONERROR 未設定時にエラーがそのまま伝わることを確認する
// （オプション検証エラーも、従来の通信エラーと同じ経路で扱う）。
func TestAjaxOptionsErrorsOtherCommands(t *testing.T) {
	const setOpt = `{"timeout":30}にAJAXオプション設定` + "\n"
	programs := map[string]string{
		"AJAX_JSON取得": setOpt + `J = 「http://audit.invalid/」からAJAX_JSON取得`,
		"AJAXバイナリ取得":  setOpt + `B = 「http://audit.invalid/」からAJAXバイナリ取得`,
		"AJAX保障送信":    setOpt + `「http://audit.invalid/」へAJAX保障送信して表示`,
		"AJAX送信時": setOpt + `●CB
　「受信: {対象}」と表示
ここまで
「http://audit.invalid/」へ「CB」でAJAX送信時`,
	}
	for name, code := range programs {
		t.Run(name, func(t *testing.T) {
			captured := installAjaxCapture(t, `{"ok":true}`)
			_, err := runAjaxCode(t, code)
			if err == nil {
				t.Fatalf("エラーにならなかった")
			}
			if !strings.Contains(err.Error(), "項目『timeout』には対応していません") {
				t.Errorf("エラー = %v", err)
			}
			if captured.count != 0 {
				t.Errorf("エラー時にリクエストが送られた: count=%d", captured.count)
			}
		})
	}
}

// TestAjaxOptionsHostHeader は headers の Host が req.Host へ反映されること。
// Go の http.Client は req.Header の Host を無視するため、そのままでは
// 「無言で捨てる」ことになる。
func TestAjaxOptionsHostHeader(t *testing.T) {
	captured := installAjaxCapture(t, "ok")
	code := `{"headers":{"Host":"example.jp"}}にAJAXオプション設定
「http://audit.invalid/」からAJAXテキスト取得して表示`
	if _, err := runAjaxCode(t, code); err != nil {
		t.Fatal(err)
	}
	if captured.host != "example.jp" {
		t.Errorf("Host = %q, want example.jp", captured.host)
	}
}

// TestAjaxOptionsNoLeakAcrossRuns は、設定したプログラムの後に設定なしの
// プログラムを実行しても、Authorization が漏れないこと。状態は実行単位の
// CommandState に持つため、同じレジストリを使い回しても漏れない。
func TestAjaxOptionsNoLeakAcrossRuns(t *testing.T) {
	captured := installAjaxCapture(t, "ok")
	withOpt := `{"headers":{"Authorization":"Bearer <token>"}}にAJAXオプション設定
「http://audit.invalid/」からAJAXテキスト取得して表示`
	withoutOpt := `「http://audit.invalid/」からAJAXテキスト取得して表示`

	// 通常の実行経路（実行ごとにレジストリを作り直す）
	if _, err := runAjaxCode(t, withOpt); err != nil {
		t.Fatal(err)
	}
	if got := captured.header.Get("Authorization"); got != "Bearer <token>" {
		t.Fatalf("1回目 Authorization = %q", got)
	}
	if _, err := runAjaxCode(t, withoutOpt); err != nil {
		t.Fatal(err)
	}
	if captured.count != 2 {
		t.Errorf("2回目のリクエスト数 = %d, want 2（累計）", captured.count)
	}
	if got := captured.header.Get("Authorization"); got != "" {
		t.Errorf("2回目に Authorization が漏れた: %q", got)
	}

	// 同じレジストリを使い回す経路（RunWithHostAndRegistry）
	registry := stdlib.NewRegistry(nodelib.New())
	run := func(code string) error {
		var out strings.Builder
		host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
		return vm.RunWithHostAndRegistry(code, "main.nako3", registry, host)
	}
	if err := run(withOpt); err != nil {
		t.Fatal(err)
	}
	if got := captured.header.Get("Authorization"); got != "Bearer <token>" {
		t.Fatalf("レジストリ再利用1回目 Authorization = %q", got)
	}
	if err := run(withoutOpt); err != nil {
		t.Fatal(err)
	}
	if captured.count != 4 {
		t.Errorf("レジストリ再利用2回目のリクエスト数 = %d, want 4（累計）", captured.count)
	}
	if got := captured.header.Get("Authorization"); got != "" {
		t.Errorf("レジストリ再利用2回目に Authorization が漏れた: %q", got)
	}
}
