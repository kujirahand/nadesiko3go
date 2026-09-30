package nodelib_test

import (
	"fmt"
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
