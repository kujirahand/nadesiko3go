package nodelib_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestAjaxJSONTrailingGarbage は Issue #207 の回帰テスト。
// 『AJAX_JSON取得』は応答内の最初のJSON値だけを解析し、後続の値を
// 無視していた。JSON.parse と同じく、先頭の値の後に空白以外が残る
// 応答はエラーになる。
func TestAjaxJSONTrailingGarbage(t *testing.T) {
	// body を応答するHTTPサーバーをその都度立てて『AJAX_JSON取得』を実行する。
	// http.DefaultTransport などのグローバル状態を書き換えないので、
	// 他のテストと並行実行しても干渉しない。
	run := func(body string) error {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		var out strings.Builder
		host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
		return vm.RunProgram(
			fmt.Sprintf(`J = "%s"からAJAX_JSON取得`, server.URL),
			"main.nako3",
			host,
		)
	}

	// 先頭のJSON値の後に別の値やゴミが続く応答はエラー
	for _, body := range []string{
		`{"ok":true} {"bad":true}`,
		`{"ok":true} garbage`,
		`1 2`,
		`[1, 2, 3] [4, 5, 6]`,
	} {
		err := run(body)
		if err == nil {
			t.Errorf("応答 %q がエラーにならなかった", body)
			continue
		}
		if !strings.Contains(err.Error(), "JSONデコードに失敗しました。") {
			t.Errorf("応答 %q のエラー文面が『JSONデコードに失敗しました。』でない: %v", body, err)
		}
	}

	// 単一のJSON値と、末尾の空白だけなら従来どおり読み取る
	for _, body := range []string{
		`{"ok":true}`,
		"{\"ok\":true}  \n\t",
		`[1, 2, 3]`,
	} {
		if err := run(body); err != nil {
			t.Errorf("応答 %q は通るはず: %v", body, err)
		}
	}
}

// closedServerURL は接続だけを受け付けないURLを返す。待ち受けを閉じた
// ポートへ接続すると、HTTPクライアントは即座に接続エラーになる。
func closedServerURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return "http://" + addr
}

// TestAjaxFailureCallback は接続に失敗したとき『AJAX失敗時』で登録した
// コールバックが呼ばれ、実行時エラーにならないことを確かめる回帰テスト。
// 以前は AJAX:ONERROR を SetSysVar で保存していたため、プログラムが
// その名前の大域変数を持たない限り黙って捨てられ、コールバックが
// 呼ばれずにエラーになっていた。
func TestAjaxFailureCallback(t *testing.T) {
	dir := t.TempDir()
	code := fmt.Sprintf(`
●(メッセージで)成功CB
　「成功: {メッセージ}」と表示
ここまで
●(メッセージで)失敗CB
　「失敗: {メッセージ}」と表示
ここまで
「失敗CB」のAJAX失敗時
URL = "%s"
URLへ「成功CB」でAJAX送信時
URLへ「成功CB」でGET送信時
URLへ「成功CB」でAJAX受信時
Params = {}
Params["name"] = "test"
「成功CB」でURLへParamsをPOST送信時
「成功CB」でURLへParamsをPOSTフォーム送信時
`, closedServerURL(t))

	got := runIn(t, dir, code)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("エラーコールバックが5回呼ばれるはず: got %q", got)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "失敗: ") {
			t.Errorf("エラーコールバックの出力でない: %q", line)
		}
	}
	if strings.Contains(got, "成功") {
		t.Errorf("失敗時に成功コールバックが呼ばれた: %q", got)
	}
}

// TestAjaxFailureCallbackTarget は引数を宣言しないコールバックでも、
// 『対象』からエラー内容を読めることを確かめる。
func TestAjaxFailureCallbackTarget(t *testing.T) {
	code := fmt.Sprintf(`
●成功CB
　「成功」と表示
ここまで
●失敗CB
　「失敗: {対象}」と表示
ここまで
「失敗CB」のAJAX失敗時
URL = "%s"
URLへ「成功CB」でAJAX送信時
`, closedServerURL(t))

	got := runIn(t, t.TempDir(), code)
	if !strings.HasPrefix(got, "失敗: ") {
		t.Errorf("『対象』にエラー内容が入っていない: %q", got)
	}
}

// TestAjaxFailureCallbackErrorPropagates は『AJAX失敗時』のコールバック内で
// 起きたエラーが握り潰されず、呼び出し元へ伝搬して後続処理が実行されない
// ことを確かめる。成功コールバックはエラーを伝搬するので、失敗コールバックも
// 同じように振る舞う必要がある。
func TestAjaxFailureCallbackErrorPropagates(t *testing.T) {
	code := fmt.Sprintf(`
●(メッセージで)失敗CB
　「失敗CB: {メッセージ}」と表示
　「わざとエラー」でエラー発生
ここまで
●(テキストで)成功CB
　「成功: {テキスト}」と表示
ここまで
「失敗CB」のAJAX失敗時
URL = "%s"
URLへ「成功CB」でAJAX送信時
「後続の処理」と表示
`, closedServerURL(t))

	got, err := runInResult(t, t.TempDir(), code)
	if err == nil {
		t.Fatalf("失敗コールバックのエラーが伝搬していない: 出力 %q", got)
	}
	if !strings.Contains(err.Error(), "わざとエラー") {
		t.Errorf("コールバックのエラー内容が失われている: %v", err)
	}
	if strings.Contains(got, "後続の処理") {
		t.Errorf("エラー後も後続処理が実行された: %q", got)
	}
	if strings.Contains(got, "成功: ") {
		t.Errorf("失敗時に成功コールバックが呼ばれた: %q", got)
	}
}

// TestAjaxFailureReregister は再登録した関数値だけが呼ばれることを確かめる。
func TestAjaxFailureReregister(t *testing.T) {
	code := fmt.Sprintf(`
●失敗CB1
 「古い」と表示
ここまで
F=関数()
 「新しい」と表示
ここまで
「失敗CB1」のAJAX失敗時
FのAJAX失敗時
URL = "%s"
URLへ「成功CB」でAJAX送信時
`, closedServerURL(t))
	if got := runIn(t, t.TempDir(), code); got != "新しい" {
		t.Errorf("再登録後の出力: %q", got)
	}
}

// TestAjaxFailureReadAndMultipart は送信後の本文読み取り失敗と送信前の
// フォーム組み立て失敗で、失敗コールバックとそのエラー伝搬を確かめる。
func TestAjaxFailureReadAndMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()
	cases := []struct{ name, call, message string }{
		{"本文読み取り", fmt.Sprintf(`"%s"へ「成功CB」でAJAX送信時`, server.URL), "unexpected EOF"},
		{"フォーム組み立て", `「成功CB」で「http://127.0.0.1:1」へ「文字列」をPOSTフォーム送信時`, "辞書型"},
	}
	for _, tc := range cases {
		for _, callbackError := range []bool{false, true} {
			name := tc.name
			if callbackError {
				name += "/コールバック内エラー"
			} else {
				name += "/正常終了"
			}
			t.Run(name, func(t *testing.T) {
				failure := ""
				if callbackError {
					failure = "「わざとエラー」でエラー発生"
				}
				code := fmt.Sprintf(`
●(メッセージで)失敗CB
 「失敗: {メッセージ}」と表示
 %s
ここまで
●成功CB
 「成功」と表示
ここまで
「失敗CB」のAJAX失敗時
%s
「後続」と表示
`, failure, tc.call)
				got, err := runInResult(t, t.TempDir(), code)
				if !strings.Contains(got, "失敗: ") || !strings.Contains(got, tc.message) || strings.Contains(got, "成功") {
					t.Errorf("失敗時の出力: %q", got)
				}
				if callbackError {
					if err == nil || !strings.Contains(err.Error(), "わざとエラー") || strings.Contains(got, "後続") {
						t.Errorf("コールバックのエラー: 出力=%q, error=%v", got, err)
					}
				} else if err != nil || !strings.Contains(got, "後続") {
					t.Errorf("コールバック後の実行: 出力=%q, error=%v", got, err)
				}
			})
		}
	}
}
