package vm_test

// #340: 取り込んだファイル内の実行時エラーは、メインファイルではなく
// 取り込んだファイルの名前と行番号に帰属させる。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func writeReqFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runReqMain は main の内容を code として実行し、*errs.NakoError を返す。
func runReqMain(t *testing.T, dir, code string) *errs.NakoError {
	t.Helper()
	_, err := vm.RunSource(code, filepath.Join(dir, "main.nako3"), nil)
	var ne *errs.NakoError
	if !errors.As(err, &ne) {
		t.Fatalf("*errs.NakoError を期待したが %T: %v", err, err)
	}
	return ne
}

func TestRequireErrorReportsDepFile(t *testing.T) {
	dir := t.TempDir()
	dep := writeReqFile(t, dir, "dep.nako3", "#コメント\n「依存ファイルの失敗」でエラー発生\n")
	ne := runReqMain(t, dir, "!「./dep.nako3」を取り込む\n")
	if ne.File != dep || ne.Line != 1 {
		t.Errorf("File=%q Line=%d, want %q / 1", ne.File, ne.Line, dep)
	}
	if want := "dep.nako3(2行目)"; !strings.Contains(ne.Error(), want) {
		t.Errorf("Error()=%q に %q がない", ne.Error(), want)
	}
}

func TestRequireErrorSameLineDifferentDeps(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, dir, "depA.nako3", "#a\n#a\n「Aの失敗」でエラー発生\n")
	writeReqFile(t, dir, "depB.nako3", "#b\n#b\n「Bの失敗」でエラー発生\n")
	for _, tc := range []struct{ dep, msg string }{
		{"depA.nako3", "Aの失敗"},
		{"depB.nako3", "Bの失敗"},
	} {
		ne := runReqMain(t, dir, "!「./"+tc.dep+"」を取り込む\n")
		if ne.File != filepath.Join(dir, tc.dep) || ne.Line != 2 {
			t.Errorf("%s: File=%q Line=%d", tc.dep, ne.File, ne.Line)
		}
		if !strings.Contains(ne.Error(), tc.dep+"(3行目)") || !strings.Contains(ne.Error(), tc.msg) {
			t.Errorf("%s: Error()=%q", tc.dep, ne.Error())
		}
	}
}

// 1つのプログラムが2つの依存ファイルを両方取り込んだときも、
// エラーは実際に失敗した側のファイルへ帰属すること (Argonレビュー Medium-1)。
// 両依存ともエラー候補は同じ3行目に置き、取り込む順序を入れ替えた
// 2ケースで、後から登録される側 (Source: 2) のファイルを検証する。
func TestRequireErrorSecondDepInSameProgram(t *testing.T) {
	for _, tc := range []struct {
		name       string
		first      string // 先に取り込む無害な依存
		second     string // 後から取り込む失敗する依存
		secondFile string
		msg        string
	}{
		{"Bが後", "depA.nako3", "depB.nako3", "depB.nako3", "Bの失敗"},
		{"Aが後", "depB.nako3", "depA.nako3", "depA.nako3", "Aの失敗"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			harmless := "#h\n#h\n「ok」を表示\n"
			failing := "#f\n#f\n「" + tc.msg + "」でエラー発生\n"
			writeReqFile(t, dir, tc.first, harmless)
			second := writeReqFile(t, dir, tc.second, failing)
			code := "!「./" + tc.first + "」を取り込む\n!「./" + tc.second + "」を取り込む\n"
			ne := runReqMain(t, dir, code)
			if ne.File != second || ne.Line != 2 {
				t.Errorf("File=%q Line=%d, want %q / 2", ne.File, ne.Line, second)
			}
			if !strings.Contains(ne.Error(), tc.secondFile+"(3行目)") || !strings.Contains(ne.Error(), tc.msg) {
				t.Errorf("Error()=%q", ne.Error())
			}
		})
	}
}

func TestRequireErrorInMainStillReportsMain(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, dir, "dep.nako3", "#何もしない\n")
	ne := runReqMain(t, dir, "!「./dep.nako3」を取り込む\n#空行代わり\n「本体の失敗」でエラー発生\n")
	main := filepath.Join(dir, "main.nako3")
	if ne.File != main || ne.Line != 2 {
		t.Errorf("File=%q Line=%d, want %q / 2", ne.File, ne.Line, main)
	}
	if want := "main.nako3(3行目)"; !strings.Contains(ne.Error(), want) {
		t.Errorf("Error()=%q に %q がない", ne.Error(), want)
	}
}
