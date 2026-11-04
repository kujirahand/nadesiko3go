package vm_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// V01: CSV の空区切りで停止不能、複数文字区切りで値が欠落する。
//
// なでしこプログラムの入口（RunProgram）から見た振る舞いを確かめる。
// CSVの設定はプロセス内のプラグインに残るので、区切り文字を変えるテストは
// 既定へ戻してから終わる。

func v01Run(t *testing.T, code string) (string, error) {
	t.Helper()
	var out strings.Builder
	h := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err := vm.RunProgram(code, "v01_regression.nako3", h)
	return strings.TrimRight(out.String(), " \t\r\n"), err
}

func TestV01VMEmptyDelimiterIsLanguageError(t *testing.T) {
	log, err := v01Run(t, `{「delimiter」：「」}をCSVオプション設定`)
	if err == nil {
		t.Fatalf("空の区切り文字がエラーになりませんでした (log=%q)", log)
	}
	if !strings.Contains(err.Error(), "1文字") {
		t.Fatalf("言語エラーに区切り文字の条件がありません: %v", err)
	}
}

func TestV01VMMultiCharDelimiterIsLanguageErrorAndRecovers(t *testing.T) {
	// 設定は言語エラーになり、エラー監視で捕まえたあと通常のCSVは読める
	code := "エラー監視\n  {「delimiter」：「||」}をCSVオプション設定\nエラーならば\n  「設定エラー」と表示。\nここまで。\n「a||b」をCSV取得してJSONエンコードして表示"
	log, err := v01Run(t, code)
	if err != nil {
		t.Fatalf("エラー監視で捕まえられませんでした: %v", err)
	}
	want := "設定エラー\n[[\"a||b\"]]"
	if log != want {
		t.Fatalf("log = %q, want %q", log, want)
	}
}

func TestV01VMJapaneseRuneDelimiter(t *testing.T) {
	t.Cleanup(func() {
		v01Run(t, `{「delimiter」：「,」}をCSVオプション設定`)
	})
	log, err := v01Run(t, "{「delimiter」：「｜」}をCSVオプション設定\n「あ｜い」をCSV取得してJSONエンコードして表示")
	if err != nil {
		t.Fatalf("日本語1文字の区切り文字で実行エラー: %v", err)
	}
	want := `[["あ","い"]]`
	if log != want {
		t.Fatalf("log = %q, want %q", log, want)
	}
}
