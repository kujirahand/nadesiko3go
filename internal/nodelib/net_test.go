package nodelib_test

import (
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
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	run := func(body string) error {
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			recorder.WriteString(body)
			return recorder.Result(), nil
		})
		var out strings.Builder
		host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
		return vm.RunProgram(`J = "http://nodelib.test/"からAJAX_JSON取得`, "main.nako3", host)
	}

	// 先頭のJSON値の後に別の値やゴミが続く応答はエラー
	for _, body := range []string{
		`{"ok":true} {"bad":true}`,
		`{"ok":true} garbage`,
		`1 2`,
	} {
		if err := run(body); err == nil {
			t.Errorf("応答 %q がエラーにならなかった", body)
		}
	}

	// 末尾の空白だけなら従来どおり読み取る
	if err := run("{\"ok\":true}  \n\t"); err != nil {
		t.Errorf("末尾が空白だけの応答は通るはず: %v", err)
	}
	if err := run(`{"ok":true}`); err != nil {
		t.Errorf("単一のJSON値は通るはず: %v", err)
	}
}
