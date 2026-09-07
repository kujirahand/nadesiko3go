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
	for _, expected := range want {
		status := waitForDialog(t, session, runID)
		if status.Dialog.Kind != expected.kind {
			t.Fatalf("dialog kind = %q, want %q", status.Dialog.Kind, expected.kind)
		}
		if !session.resolveDialog(runID, status.Dialog.ID, expected.text, expected.accepted) {
			t.Fatalf("resolveDialog(%d) failed", status.Dialog.ID)
		}
	}

	status := waitForAsyncDone(t, session, runID)
	if status.Result == nil || !status.Result.OK {
		t.Fatalf("result = %#v", status.Result)
	}
	if got := status.Result.Output; got != "12.5:false\n" {
		t.Fatalf("output = %q, want %q", got, "12.5:false\n")
	}
}

func waitForDialog(t *testing.T, session *guiSession, runID uint64) AsyncRunStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := session.poll(runID)
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

func waitForAsyncDone(t *testing.T, session *guiSession, runID uint64) AsyncRunStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := session.poll(runID)
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
