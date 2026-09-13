package wasmrt

import (
	"errors"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
)

func TestRunPrintsAndNotifies(t *testing.T) {
	var lines []string
	h := &Host{OnPrint: func(s string) { lines = append(lines, s) }}
	err := Run("「こんにちは」を表示。\n1+2*3を表示。\n「a,b」をCSV取得してJSONエンコードして表示。\n", "", h)
	if err != nil {
		t.Fatalf("実行に失敗しました: %v", err)
	}
	want := "こんにちは\n7\n[[\"a\",\"b\"]]\n"
	if got := h.Output(); got != want {
		t.Fatalf("出力が違います: got %q, want %q", got, want)
	}
	if len(lines) != 3 || lines[1] != "7" {
		t.Fatalf("OnPrintの通知が違います: %q", lines)
	}
}

func TestRunReportsNakoError(t *testing.T) {
	h := &Host{}
	err := Run("「あ」を表示。\n存在しない命令。\n", "test.nako3", h)
	var ne *errs.NakoError
	if !errors.As(err, &ne) {
		t.Fatalf("NakoErrorが返りませんでした: %v", err)
	}
	if ne.File != "test.nako3" {
		t.Fatalf("ファイル名が違います: %q", ne.File)
	}
}

func TestNodeLibIsNotAvailable(t *testing.T) {
	// コア機能だけに絞っているので、ファイル操作の命令は使えない
	if _, ok := Registry().Lookup("ファイル名一覧取得"); ok {
		t.Fatal("nodelibの命令が含まれています")
	}
}

func TestSayUsesDialogOrFallsBack(t *testing.T) {
	h := &Host{}
	if err := Run("「やあ」と言う。", "", h); err != nil {
		t.Fatal(err)
	}
	if h.Output() != "やあ\n" {
		t.Fatalf("ダイアログ未対応なら表示に切り替わるはずです: %q", h.Output())
	}

	var got string
	h = &Host{Dialog: func(kind, message string) (string, bool, bool, error) {
		got = kind + ":" + message
		return "", true, true, nil
	}}
	if err := Run("「やあ」と言う。", "", h); err != nil {
		t.Fatal(err)
	}
	if got != "alert:やあ" || h.Output() != "" {
		t.Fatalf("ダイアログに渡りませんでした: %q / %q", got, h.Output())
	}
}
