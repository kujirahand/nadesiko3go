package editorlint

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/doctest"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func parseGonako(code, filename string) error {
	_, err := vm.ParseProgram(code, filename)
	return err
}

func gonakoOpt(sync bool) Options {
	return Options{Whole: parseGonako, Sample: parseGonako, Web: parseGonako, Sync: sync}
}

func TestCheckWithoutDocTestUnchanged(t *testing.T) {
	res := Check("「こんにちは」と表示。", "plain.nako3", "plain.nako3", gonakoOpt(false))
	if !res.OK || res.Error != "" || res.Text != "" || res.DocTest != nil {
		t.Fatalf("DocTestが無いプログラム: %#v", res)
	}
}

func TestManualDocTestPasses(t *testing.T) {
	code := "説明です。\n{{{#nako3\n「こんにちは」と表示。\n### 表示結果: こんにちは\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", gonakoOpt(true))
	if !res.OK {
		t.Fatalf("OK = false, text=%s error=%s", res.Text, res.Error)
	}
	if !strings.Contains(res.Text, "文法エラーはありません") || !strings.Contains(res.Text, "1件成功") {
		t.Fatalf("Text = %q", res.Text)
	}
	if res.DocTest != nil {
		t.Fatalf("同期実行なのに pending が付いています: %#v", res.DocTest)
	}
}

func TestManualDocTestReportsFailureLocation(t *testing.T) {
	code := "{{{#nako3\n「A」と表示。\n### 表示結果: B\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", gonakoOpt(true))
	if res.OK {
		t.Fatal("表示結果が違うのに成功しました")
	}
	if !strings.Contains(res.Text, "[DocTest失敗]") || !strings.Contains(res.Text, "memo.txt:1") {
		t.Fatalf("失敗箇所がありません: %s", res.Text)
	}
	if !strings.Contains(res.Text, "1件失敗") || !strings.Contains(res.Error, "[DocTest失敗]") {
		t.Fatalf("Text=%s Error=%s", res.Text, res.Error)
	}
}

func TestManualDocTestSyntaxErrorDoesNotRun(t *testing.T) {
	calls := 0
	opt := gonakoOpt(true)
	opt.Run = func(doctest.Test) doctest.Result {
		calls++
		return doctest.Result{OK: true}
	}
	code := "説明\n{{{#nako3\nもし「A」ならば\n「B」と表示。\n### 表示結果: B\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", opt)
	if res.OK {
		t.Fatal("サンプルの文法エラーを見落としています")
	}
	if calls != 0 {
		t.Fatalf("文法エラーなのにDocTestを%d回実行しました", calls)
	}
	if !strings.Contains(res.Error, "(3行目)") {
		t.Fatalf("ファイル上の行番号になっていません: %s", res.Error)
	}
}

func TestProgramSyntaxErrorSkipsDocTestInComment(t *testing.T) {
	calls := 0
	opt := gonakoOpt(true)
	opt.Run = func(doctest.Test) doctest.Result {
		calls++
		return doctest.Result{OK: true}
	}
	code := "もし\n/*\n{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n*/\n"
	res := Check(code, "app.nako3", "app.nako3", opt)
	if res.OK || calls != 0 {
		t.Fatalf("OK=%v calls=%d error=%s", res.OK, calls, res.Error)
	}
}

func TestUnsavedBufferUsesWholeFileGrammar(t *testing.T) {
	// 未保存のバッファは originalName が空。GUIは表示名だけ gui.nako3 にする。
	okCode := "「ready」と表示。\n/*\n{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n*/\n"
	ok := Check(okCode, "gui.nako3", "", gonakoOpt(true))
	if !ok.OK || !strings.Contains(ok.Text, "1件成功") {
		t.Fatalf("未保存の正しいプログラム: OK=%v text=%s error=%s", ok.OK, ok.Text, ok.Error)
	}
	calls := 0
	opt := gonakoOpt(true)
	opt.Run = func(doctest.Test) doctest.Result {
		calls++
		return doctest.Result{OK: true}
	}
	bad := Check("もし\n/*\n{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n*/\n", "gui.nako3", "", opt)
	if bad.OK || calls != 0 {
		t.Fatalf("未保存の文法エラーで実行した: OK=%v calls=%d error=%s", bad.OK, calls, bad.Error)
	}
}

func TestProgramCommentDocTestRunsAfterSyntaxOK(t *testing.T) {
	code := "「ready」と表示。\n/*\n{{{#nako3\n「こんにちは」と表示。\n### 表示結果: こんにちは\n}}}\n*/\n"
	res := Check(code, "app.nako3", "app.nako3", gonakoOpt(true))
	if !res.OK || !strings.Contains(res.Text, "1件成功") {
		t.Fatalf("OK=%v text=%s error=%s", res.OK, res.Text, res.Error)
	}
}

func TestManualWithoutDocTestStillLintsWholeFile(t *testing.T) {
	// DocTestが無ければ .txt でもバッファ全体を従来どおり構文解析する。
	res := Check("もし\n", "note.txt", "note.txt", gonakoOpt(false))
	if res.OK {
		t.Fatal("DocTestの無い文法エラーを見落としています")
	}
}

func TestGoLabelAndWebLabel(t *testing.T) {
	code := "{{{#nako3\n1を表示\n### GO表示結果: 1\n}}}\n" +
		"{{{#nako3\n「div」のDOM_HTML取得して表示\n### WEB表示結果: div\n}}}\n" +
		"{{{#nako3\n1を表示\n### L表示結果: 1\n}}}\n"
	opt := gonakoOpt(true)
	// ブラウザ専用命令は、WEB表示結果の解析ではエラーにしない。
	opt.Web = func(code, filename string) error { return nil }
	res := Check(code, "memo.txt", "memo.txt", opt)
	if !res.OK {
		t.Fatalf("OK = false, text=%s error=%s", res.Text, res.Error)
	}
	if !strings.Contains(res.Text, "1件成功") {
		t.Fatalf("GO表示結果が実行されていません: %s", res.Text)
	}
	if !strings.Contains(res.Text, "WEB表示結果のDocTest 1件") || !strings.Contains(res.Text, "1件は、この文法チェックでは実行しません") {
		t.Fatalf("省略の説明がありません: %s", res.Text)
	}
}

func TestWebSampleSyntaxErrorBlocksDocTest(t *testing.T) {
	calls := 0
	opt := gonakoOpt(true)
	opt.Run = func(doctest.Test) doctest.Result {
		calls++
		return doctest.Result{OK: true}
	}
	opt.Web = func(code, filename string) error {
		if strings.Contains(code, "もし") {
			return parseGonako("もし", filename)
		}
		return nil
	}
	code := "{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n" +
		"{{{#nako3\nもし\n### WEB表示結果: x\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", opt)
	if res.OK || calls != 0 {
		t.Fatalf("OK=%v calls=%d error=%s", res.OK, calls, res.Error)
	}
}

func TestDocTestContinuesAfterFailure(t *testing.T) {
	code := "{{{#nako3\n「A」と表示。\n### 表示結果: B\n}}}\n" +
		"{{{#nako3\n「C」と表示。\n### 表示結果: C\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", gonakoOpt(true))
	if res.OK {
		t.Fatal("失敗があるのに成功しました")
	}
	if !strings.Contains(res.Text, "1件成功、1件失敗") {
		t.Fatalf("Text = %s", res.Text)
	}
}

func TestDocTestSkipsOmittedCommands(t *testing.T) {
	code := "{{{#nako3\n『1』をJS実行して表示\n### 表示結果: 1\n}}}\n"
	res := Check(code, "memo.txt", "memo.txt", gonakoOpt(true))
	if !res.OK || !strings.Contains(res.Text, "1件省略") {
		t.Fatalf("OK=%v text=%s error=%s", res.OK, res.Text, res.Error)
	}
}

func TestDocTestPromptDoesNotBlock(t *testing.T) {
	opt := gonakoOpt(true)
	opt.Timeout = 2 * time.Second
	code := "{{{#nako3\nA=「名前」と尋\nAを表示\n### 表示結果: 誰か\n}}}\n"
	start := time.Now()
	res := Check(code, "memo.txt", "memo.txt", opt)
	if time.Since(start) > time.Second {
		t.Fatalf("尋が戻りません: %s", time.Since(start))
	}
	if res.OK {
		t.Fatal("入力できない尋が成功しました")
	}
	if !strings.Contains(res.Text, "尋") && !strings.Contains(res.Text, "[DocTest失敗]") {
		t.Fatalf("Text = %s", res.Text)
	}
}

func TestDocTestWaitIsVirtual(t *testing.T) {
	code := "{{{#nako3\n1秒待\n「ok」と表示\n### 表示結果: ok\n}}}\n"
	start := time.Now()
	res := Check(code, "memo.txt", "memo.txt", gonakoOpt(true))
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("秒待が実時間でした: %s", time.Since(start))
	}
	if !res.OK {
		t.Fatalf("OK = false, text=%s error=%s", res.Text, res.Error)
	}
}

func TestDocTestTimeout(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	opt := gonakoOpt(true)
	opt.Timeout = 30 * time.Millisecond
	opt.Run = func(doctest.Test) doctest.Result {
		<-release
		return doctest.Result{OK: true}
	}
	code := "{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n"
	start := time.Now()
	res := Check(code, "memo.txt", "memo.txt", opt)
	if time.Since(start) > time.Second {
		t.Fatal("制限時間で戻っていません")
	}
	if res.OK || !strings.Contains(res.Text, "制限時間") {
		t.Fatalf("OK=%v text=%s", res.OK, res.Text)
	}
}

func TestDocTestAsyncDoesNotBlockSyntaxCheck(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	opt := gonakoOpt(false)
	opt.Timeout = time.Hour
	opt.Run = func(doctest.Test) doctest.Result {
		<-release
		return doctest.Result{OK: true}
	}
	code := "{{{#nako3\n「A」と表示。\n### 表示結果: A\n}}}\n"
	start := time.Now()
	res := Check(code, "memo.txt", "memo.txt", opt)
	if time.Since(start) > time.Second {
		t.Fatal("文法チェックがDocTestの完了を待っています")
	}
	if !res.OK || res.DocTest == nil || !res.DocTest.Pending || res.DocTest.Total != 1 || res.DocTest.RunID == 0 {
		t.Fatalf("pending 応答ではありません: %#v", res)
	}
	if poll := Poll(res.DocTest.RunID); poll.Done {
		t.Fatal("まだ終わっていないDocTestが完了になっています")
	}
	free()
	deadline := time.Now().Add(2 * time.Second)
	var poll Status
	for {
		poll = Poll(res.DocTest.RunID)
		if poll.Done || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !poll.Done || !poll.OK || !strings.Contains(poll.Text, "1件成功") {
		t.Fatalf("poll = %#v", poll)
	}
	if again := Poll(res.DocTest.RunID); again.OK || again.Error == "" {
		t.Fatalf("完了後の再取得: %#v", again)
	}
}

func TestFizzBuzzManualDocTest(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "doctest", "fizzbuzz.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := Check(string(data), path, path, gonakoOpt(true))
	if !res.OK || !strings.Contains(res.Text, "2件成功") {
		t.Fatalf("OK=%v text=%s error=%s", res.OK, res.Text, res.Error)
	}
	calls := 0
	opt := gonakoOpt(true)
	opt.Run = func(doctest.Test) doctest.Result {
		calls++
		return doctest.Result{OK: true}
	}
	blocked := Check(string(data), "fizzbuzz.nako3", "fizzbuzz.nako3", opt)
	if blocked.OK || calls != 0 {
		t.Fatalf("nako3として実行してしまいました: OK=%v calls=%d error=%s", blocked.OK, calls, blocked.Error)
	}
}

func TestExtraFailuresAreHidden(t *testing.T) {
	var b strings.Builder
	for i := 0; i < detailLimit+2; i++ {
		b.WriteString("{{{#nako3\n「A」と表示。\n### 表示結果: B\n}}}\n")
	}
	res := Check(b.String(), "memo.txt", "memo.txt", gonakoOpt(true))
	if res.OK || !strings.Contains(res.Text, "ほか2件の失敗は省略しました。") {
		t.Fatalf("OK=%v text=%s", res.OK, res.Text)
	}
}
