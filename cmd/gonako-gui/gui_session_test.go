package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func TestGUISessionRetainsVMForBrowserEvents(t *testing.T) {
	session := &guiSession{}
	code := `カウント=0
ボタン=「加算」のボタン作成
ボタンをクリックした時には
　カウント=カウント+1
　「{カウント}回」を表示
ここまで`
	initial := session.run(code, "gui.nako3", true, nil, nil)
	if !initial.OK || initial.RunID == 0 {
		t.Fatalf("initial result = %#v", initial)
	}
	if len(initial.Operations) != 2 || initial.Operations[0].Tag != "button" || initial.Operations[1].Type != "listen" {
		t.Fatalf("initial operations = %#v", initial.Operations)
	}

	first := session.dispatch(initial.RunID, 1, "click", nil)
	second := session.dispatch(initial.RunID, 1, "click", nil)
	if !first.OK || strings.TrimSpace(first.Output) != "1回" {
		t.Fatalf("first click = %#v", first)
	}
	if !second.OK || strings.TrimSpace(second.Output) != "2回" {
		t.Fatalf("second click = %#v", second)
	}
	if len(second.Operations) != 1 || second.Operations[0].Text != "2回" {
		t.Fatalf("second operations = %#v", second.Operations)
	}
}

func TestGUISessionRejectsStaleEvents(t *testing.T) {
	session := &guiSession{}
	first := session.run("「前」のボタン作成", "gui.nako3", true, nil, nil)
	second := session.run("「後」のボタン作成", "gui.nako3", true, nil, nil)
	if first.RunID == second.RunID {
		t.Fatal("run ids must change")
	}
	got := session.dispatch(first.RunID, 1, "click", nil)
	if got.OK || !strings.Contains(got.Error, "古い実行結果") {
		t.Fatalf("stale event result = %#v", got)
	}
}

func TestSynchronousGUIRunKeepsSayFallback(t *testing.T) {
	result := (&guiSession{}).run("「テスト」と言う", "gui.nako3", true, nil, nil)
	if !result.OK || result.Output != "テスト\n" {
		t.Fatalf("result = %#v", result)
	}
}

// 『表示』が実行完了を待たずポーリングの途中で出てくることを確かめる。
// #48: これが無いと出力はすべてプログラム終了後にまとめて出てしまう。
func TestAsyncRunStreamsOutputWhileRunning(t *testing.T) {
	session := &guiSession{}
	runID := session.start(`3回
	「あ」と表示
	0.05秒待つ
ここまで`, "loop.nako3", false, nil, nil)

	deadline := time.Now().Add(3 * time.Second)
	sawOutputBeforeDone := false
	for time.Now().Before(deadline) {
		status := session.poll(runID)
		if status.Output != "" && !status.Done {
			sawOutputBeforeDone = true
		}
		if status.Done {
			if !status.Result.OK {
				t.Fatalf("run failed: %#v", status.Result)
			}
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !sawOutputBeforeDone {
		t.Fatal("『表示』の出力が実行完了前にポーリングへ現れなかった（#48が再発）")
	}
}

// 実行中に作られた画面部品が、ポーリングで取り出す限り1つも失われないこと。
// ストリーム分を捨てると『ボタン作成』が消え、バンドル版は何も描けないまま
// closeBundledWindow() でウィンドウを閉じてしまう。
func TestAsyncRunDeliversEveryOperationThroughStream(t *testing.T) {
	session := &guiSession{}
	runID := session.start(`ボタン=「送信」のボタン作成
0.1秒待つ
「done」と表示`, "gui.nako3", true, nil, nil)

	collector := newAsyncCollector(session, runID)
	status := waitForAsyncDone(t, collector)
	if status.Result == nil || !status.Result.OK {
		t.Fatalf("result = %#v", status.Result)
	}

	var tags []string
	for _, op := range collector.ops {
		if op.Type == "create" {
			tags = append(tags, op.Tag)
		}
	}
	if len(tags) != 2 || tags[0] != "button" || tags[1] != "div" {
		t.Fatalf("created tags = %v, want [button div]（実行中の操作が失われている）", tags)
	}
	if got := collector.output.String(); got != "done\n" {
		t.Fatalf("streamed output = %q, want %q", got, "done\n")
	}
}

// 実行中のクリックが実行終了まで待たされないこと。WebViewのバインド呼び出しは
// UIスレッド上で処理されるので、ここで待つとウィンドウごと固まる。
func TestDispatchDuringRunDoesNotBlock(t *testing.T) {
	session := &guiSession{}
	runID := session.start(`ボタン=「送信」のボタン作成
2秒待つ`, "gui.nako3", true, nil, nil)

	// 画面部品が届く＝実行中である、と分かってからクリックする。
	collector := newAsyncCollector(session, runID)
	deadline := time.Now().Add(2 * time.Second)
	for len(collector.ops) == 0 && time.Now().Before(deadline) {
		collector.poll()
		time.Sleep(time.Millisecond)
	}
	if len(collector.ops) == 0 {
		t.Fatal("実行中に画面部品が届かなかった")
	}

	start := time.Now()
	got := session.dispatch(runID, 1, "click", nil)
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("実行終了まで待たされた: %v", elapsed)
	}
	if got.OK || !strings.Contains(got.Error, "実行中") {
		t.Fatalf("dispatch during run = %#v", got)
	}
}

// バンドル版のページは done を待たずに毎回のポーリングで出力・画面操作を
// 取り込まなければならない。取りこぼすと二度と取り出せない。
func TestBundledAsyncPageConsumesStreamedOperations(t *testing.T) {
	page := bundledAsyncProgramPage(1)
	for _, want := range []string{"if(s.output)out+=s.output", "if(s.operations&&s.operations.length)", "apply(s.operations)"} {
		if !strings.Contains(page, want) {
			t.Fatalf("バンドル版ページが途中経過を取り込んでいない: %q が無い", want)
		}
	}
	if strings.Contains(page, "apply(r.operations") {
		t.Fatal("完了時のResultから画面操作を読んではいけない（非同期実行では常に空）")
	}
}

func TestBundledProgramPageEscapesEmbeddedHTML(t *testing.T) {
	result := RunResult{OK: true, RunID: 1}
	result.Operations = append(result.Operations, guilib.Operation{Type: "create", Handle: 1, Tag: "div", HTML: `</script><b>ok</b>`})
	page := bundledProgramPage(result)
	if strings.Contains(page, `</script><b>ok</b>`) {
		t.Fatal("operation JSON must not be able to close the bootstrap script")
	}
	if !strings.Contains(page, `dispatchNakoEvent`) {
		t.Fatal("bundled page must include the event bridge")
	}
}

func TestFormGUISampleCompiles(t *testing.T) {
	code, err := uiFS.ReadFile("ui/samples/10_フォームGUIアプリ.nako3")
	if err != nil {
		t.Fatal(err)
	}
	result := (&guiSession{}).run(string(code), "sample.nako3", true, nil, nil)
	if !result.OK {
		t.Fatalf("sample failed: %s", result.Error)
	}
	if len(result.Operations) == 0 {
		t.Fatal("sample produced no GUI operations")
	}
}

func TestDOMGUISampleCompilesAndHandlesEvent(t *testing.T) {
	const samplePath = "ui/samples/12_DOM操作.nako3"
	code, err := uiFS.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	result := session.run(string(code), samplePath, true, nil, nil)
	if !result.OK {
		t.Fatalf("sample failed: %s", result.Error)
	}

	wantTypes := []string{"create", "text", "html", "focus", "listen"}
	if len(result.Operations) != len(wantTypes) {
		t.Fatalf("operations = %#v", result.Operations)
	}
	for i, want := range wantTypes {
		if got := result.Operations[i].Type; got != want {
			t.Fatalf("operation[%d].Type = %q, want %q", i, got, want)
		}
	}

	clicked := session.dispatch(result.RunID, 6, "click", map[string]string{"5": "花子"})
	if !clicked.OK {
		t.Fatalf("click failed: %s", clicked.Error)
	}
	if len(clicked.Operations) != 1 || clicked.Operations[0].Type != "text" || clicked.Operations[0].Text != "こんにちは、花子さん！" {
		t.Fatalf("click operations = %#v", clicked.Operations)
	}

	found := false
	for _, item := range getTemplateList() {
		if item.ID == "12_DOM操作" {
			found = true
			if item.Category != "GUI" || item.Title != "DOM操作" {
				t.Fatalf("template = %#v", item)
			}
		}
	}
	if !found {
		t.Fatal("DOM sample was not listed as a template")
	}
}

func TestGUIAsyncDialogs(t *testing.T) {
	session := &guiSession{}
	runID := session.start(`
「こんにちは」と言う
A=「数値を入力」と尋ねる
B=「続ける？」で二択
「{A}:{B}」を表示
`, "dialog.nako3", true, nil, nil)

	want := []struct {
		kind     string
		text     string
		accepted bool
	}{
		{kind: "alert", accepted: true},
		{kind: "prompt", text: "１２.５", accepted: true},
		{kind: "confirm", accepted: false},
	}
	collector := newAsyncCollector(session, runID)
	for _, expected := range want {
		status := waitForDialog(t, collector)
		if status.Dialog.Kind != expected.kind {
			t.Fatalf("dialog kind = %q, want %q", status.Dialog.Kind, expected.kind)
		}
		if !session.resolveDialog(runID, status.Dialog.ID, expected.text, expected.accepted) {
			t.Fatalf("resolveDialog(%d) failed", status.Dialog.ID)
		}
	}

	status := waitForAsyncDone(t, collector)
	if status.Result == nil || !status.Result.OK {
		t.Fatalf("result = %#v", status.Result)
	}
	// 出力はストリーム側にだけ届く。Result には残らない。
	if got := collector.output.String(); got != "12.5:false\n" {
		t.Fatalf("streamed output = %q, want %q", got, "12.5:false\n")
	}
	if status.Result.Output != "" || len(status.Result.Operations) != 0 {
		t.Fatalf("非同期実行のResultには出力・画面操作を残さない: %#v", status.Result)
	}
}

// asyncCollector polls the way a frontend has to: Output と Operations は
// 読んだ時点で消えるので、毎回のポーリングで必ず拾って貯める。
type asyncCollector struct {
	session *guiSession
	runID   uint64
	output  strings.Builder
	ops     []guilib.Operation
}

func newAsyncCollector(session *guiSession, runID uint64) *asyncCollector {
	return &asyncCollector{session: session, runID: runID}
}

func (c *asyncCollector) poll() AsyncRunStatus {
	status := c.session.poll(c.runID)
	c.output.WriteString(status.Output)
	c.ops = append(c.ops, status.Operations...)
	return status
}

func waitForDialog(t *testing.T, c *asyncCollector) AsyncRunStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := c.poll()
		if status.Dialog != nil {
			return status
		}
		if status.Done {
			t.Fatalf("run finished before dialog: %#v", status.Result)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for dialog")
	return AsyncRunStatus{}
}

func waitForAsyncDone(t *testing.T, c *asyncCollector) AsyncRunStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := c.poll()
		if status.Done {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for run")
	return AsyncRunStatus{}
}

func TestBundledAsyncProgramPageUsesHTMLDialogs(t *testing.T) {
	page := bundledAsyncProgramPage(7)
	for _, want := range []string{"pollNakoRun", "resolveNakoDialog", `class="overlay"`, "runId=7"} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	for _, unwanted := range []string{"window.alert(", "window.prompt(", "window.confirm("} {
		if strings.Contains(page, unwanted) {
			t.Errorf("page unexpectedly contains native dialog %q", unwanted)
		}
	}
}
