package main

import (
	"strings"
	"sync"
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

	first := dispatchEvent(t, session, initial.RunID, 1, "click", noEvent)
	second := dispatchEvent(t, session, initial.RunID, 1, "click", noEvent)
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
	got := session.startEvent(first.RunID, 1, "click", noEvent)
	if !strings.Contains(got.Error, "古い実行結果") {
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

	// goroutineの後片付けまで待つ（→ waitForRunExit）。
	waitForRunExit(t, session)
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
	got := session.startEvent(runID, 1, "click", noEvent)
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("実行終了まで待たされた: %v", elapsed)
	}
	if !strings.Contains(got.Error, "実行中") {
		t.Fatalf("dispatch during run = %#v", got)
	}

	// この実行はわざと終わらせずに残すが、テストを終える前にgoroutineの
	// 後片付け（os.Chdir での作業フォルダ復元）まで待つ。待たずに終えると、
	// 2秒後にプロセス全体の作業フォルダが書き換わり、その時点で走っている
	// 別のテスト（include_build_test.go など）を壊す（→ waitForRunExit）。
	waitForRunExit(t, session)
}

// バンドル版のページは done を待たずに毎回のポーリングで出力・画面操作を
// 取り込まなければならない。取りこぼすと二度と取り出せない。
func TestBundledAsyncPageConsumesStreamedOperations(t *testing.T) {
	page := bundledAsyncProgramPage(1)
	for _, want := range []string{"if (s.output) out += s.output", "if (s.operations && s.operations.length)", "apply(s.operations)"} {
		if !pageHas(page, want) {
			t.Fatalf("バンドル版ページが途中経過を取り込んでいない: %q が無い", want)
		}
	}
	if pageHas(page, "apply(r.operations") {
		t.Fatal("完了時のResultから画面操作を読んではいけない（非同期実行では常に空）")
	}
}

func TestBundledAsyncProgramPageIncludesEventBridge(t *testing.T) {
	page := bundledAsyncProgramPage(1)
	for _, required := range []string{"window.startNakoEvent(", "window.resolveNakoDialog(id,"} {
		if !pageHas(page, required) {
			t.Fatalf("bundled page is missing %q", required)
		}
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

	wantTypes := []string{"create", "text", "html", "focus", "create", "styles", "create", "text", "listen"}
	if len(result.Operations) != len(wantTypes) {
		t.Fatalf("operations = %#v", result.Operations)
	}
	for i, want := range wantTypes {
		if got := result.Operations[i].Type; got != want {
			t.Fatalf("operation[%d].Type = %q, want %q", i, got, want)
		}
	}
	if op := result.Operations[4]; op.Tag != "aside" || op.Parent != 2 {
		t.Fatalf("DOM部品作成 operation = %#v", op)
	}
	// HTML変更で差し込まれた<strong>も部品としてハンドル8を持つため(#353)、
	// 以後の部品のハンドルは従来より1つずれる。
	if op := result.Operations[5]; op.Type != "styles" || op.Handle != 9 || op.Styles["border"] == "" {
		t.Fatalf("DOMスキン operation = %#v", op)
	}
	if op := result.Operations[6]; op.Type != "create" || op.Tag != "br" || op.Parent != 2 || op.Handle != 10 {
		t.Fatalf("自動改行 operation = %#v", op)
	}

	clicked := dispatchEvent(t, session, result.RunID, 6, "click", textPayload(map[string]string{"5": "花子"}))
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

func TestAdditionalDOMPartsSampleCompilesAndHandlesChange(t *testing.T) {
	const samplePath = "ui/samples/13_追加DOM部品.nako3"
	code, err := uiFS.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	result := session.run(string(code), samplePath, true, nil, nil)
	if !result.OK {
		t.Fatalf("sample failed: %s", result.Error)
	}

	created := map[int]guilib.Operation{}
	attributes := map[int]map[string]string{}
	for _, op := range result.Operations {
		if op.Type == "create" {
			created[op.Handle] = op
		}
		if len(op.Attributes) > 0 {
			if attributes[op.Handle] == nil {
				attributes[op.Handle] = map[string]string{}
			}
			for key, item := range op.Attributes {
				attributes[op.Handle][key] = item
			}
		}
	}
	checks := []struct {
		handle int
		tag    string
		parent int
	}{
		{5, "textarea", 2},
		{6, "span", 2},
		{7, "input", 6},
		{9, "select", 2},
		{13, "input", 2},
	}
	for _, check := range checks {
		op, ok := created[check.handle]
		if !ok || op.Tag != check.tag || op.Parent != check.parent {
			t.Fatalf("created[%d] = %#v", check.handle, op)
		}
	}
	if attributes[7]["type"] != "checkbox" || attributes[13]["type"] != "range" {
		t.Fatalf("input attributes: checkbox=%#v range=%#v", attributes[7], attributes[13])
	}

	changed := dispatchEvent(t, session, result.RunID, 9, "change", textPayload(map[string]string{
		"5": "更新したメモ", "7": "on", "9": "青", "13": "60",
	}))
	if !changed.OK {
		t.Fatalf("change failed: %s", changed.Error)
	}
	if len(changed.Operations) != 1 || changed.Operations[0].Type != "text" || changed.Operations[0].Handle != 14 || changed.Operations[0].Text != "選択した色: 青" {
		t.Fatalf("change operations = %#v", changed.Operations)
	}

	found := false
	for _, item := range getTemplateList() {
		if item.ID == "13_追加DOM部品" {
			found = true
			if item.Category != "GUI" || item.Title != "追加DOM部品" {
				t.Fatalf("template = %#v", item)
			}
		}
	}
	if !found {
		t.Fatal("additional DOM sample was not listed as a template")
	}
}

func TestDOMLifecycleOperationsReachGUISession(t *testing.T) {
	const samplePath = "ui/samples/14_DOM要素ライフサイクル.nako3"
	code, err := uiFS.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	result := (&guiSession{}).run(string(code), samplePath, true, nil, nil)
	if !result.OK {
		t.Fatalf("lifecycle program failed: %s", result.Error)
	}
	wantTypes := []string{"create", "create", "text", "attributes", "append", "append", "create", "text", "append", "remove"}
	if len(result.Operations) != len(wantTypes) {
		t.Fatalf("operations = %#v", result.Operations)
	}
	for i, want := range wantTypes {
		if got := result.Operations[i].Type; got != want {
			t.Fatalf("operation[%d].Type = %q, want %q", i, got, want)
		}
	}
	if !result.Operations[1].Detached || result.Operations[4].Parent != 2 || result.Operations[5].Parent != 4 || !result.Operations[6].Detached {
		t.Fatalf("lifecycle operations = %#v", result.Operations)
	}
}

func TestGUIAsyncDialogs(t *testing.T) {
	// #220で標準入力と統一した変換を、GUIの非同期ダイアログでも確認する。
	for _, tt := range []struct {
		name   string
		input  string
		output string
	}{
		{name: "空入力", input: "", output: "0:number:false\n"},
		{name: "空白のみ", input: "   ", output: "0:number:false\n"},
		{name: "半角数値", input: "12.5", output: "12.5:number:false\n"},
		{name: "全角数字", input: "１２.５", output: "１２.５:string:false\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := &guiSession{}
			runID := session.start(`
ダイアログキャンセル値は「きゃんせる」
「こんにちは」と言う
A=「数値を入力」と尋ねる
B=「続ける？」で二択
型=Aの変数型確認
「{A}:{型}:{B}」を表示
`, "dialog.nako3", true, nil, nil)

			want := []struct {
				kind     string
				text     string
				accepted bool
			}{
				{kind: "alert", accepted: true},
				{kind: "prompt", text: tt.input, accepted: true},
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
			if got := collector.output.String(); got != tt.output {
				t.Fatalf("streamed output = %q, want %q", got, tt.output)
			}
			if status.Result.Output != "" || len(status.Result.Operations) != 0 {
				t.Fatalf("非同期実行のResultには出力・画面操作を残さない: %#v", status.Result)
			}
		})
	}
}

func TestGUIAsyncPromptCancelValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		code string
		want string
	}{
		{
			name: "設定値は尋と文字尋に反映",
			code: `ダイアログキャンセル値は「きゃんせる」
A=「入力」と尋ねる
B=「入力」と文字尋ねる
AT= Aの変数型確認
BT= Bの変数型確認
「{A}:{AT}:{B}:{BT}」を表示`,
			want: "きゃんせる:string:きゃんせる:string\n",
		},
		{
			name: "未設定時は空文字列",
			code: `A=「入力」と尋ねる
B=「入力」と文字尋ねる
「[{A}]:[{B}]」を表示`,
			want: "[]:[]\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := &guiSession{}
			runID := session.start(tt.code, "prompt-cancel.nako3", true, nil, nil)
			collector := newAsyncCollector(session, runID)
			for range 2 {
				status := waitForDialog(t, collector)
				if status.Dialog.Kind != "prompt" {
					t.Fatalf("dialog kind = %q, want prompt", status.Dialog.Kind)
				}
				if !session.resolveDialog(runID, status.Dialog.ID, "", false) {
					t.Fatalf("resolveDialog(%d) failed", status.Dialog.ID)
				}
			}
			status := waitForAsyncDone(t, collector)
			if status.Result == nil || !status.Result.OK {
				t.Fatalf("result = %#v", status.Result)
			}
			if got := collector.output.String(); got != tt.want {
				t.Fatalf("streamed output = %q, want %q", got, tt.want)
			}
		})
	}
}

// イベントハンドラの中の『言う』で固まらないこと（#59）。同期実行していた
// 頃は、ダイアログの応答を待つイベントと、イベントの終了を待つ画面とが
// 互いに待ち合ってウィンドウごと固まっていた。
func TestEventHandlerDialogDoesNotDeadlock(t *testing.T) {
	session := &guiSession{}
	code := `「あ」のボタン作成
それのクリック時には
　「あ」と言う
ここまで`
	initial := session.run(code, "gui.nako3", true, nil, nil)
	if !initial.OK {
		t.Fatalf("initial result = %#v", initial)
	}

	started := session.startEvent(initial.RunID, 1, "click", noEvent)
	if started.Error != "" || started.RunID == 0 {
		t.Fatalf("startEvent = %#v", started)
	}
	collector := newAsyncCollector(session, started.RunID)
	status := waitForDialog(t, collector)
	if status.Dialog.Kind != "alert" || status.Dialog.Message != "あ" {
		t.Fatalf("dialog = %#v", status.Dialog)
	}
	if !session.resolveDialog(started.RunID, status.Dialog.ID, "", true) {
		t.Fatal("resolveDialog に失敗した")
	}
	if done := waitForAsyncDone(t, collector); done.Result == nil || !done.Result.OK {
		t.Fatalf("event result = %#v", done.Result)
	}

	// 応答を返せば次のイベントも普通に動く＝execMuが解放されている。
	if again := session.startEvent(initial.RunID, 1, "click", noEvent); again.Error != "" {
		t.Fatalf("ダイアログ応答後にイベントが動かない: %#v", again)
	}
}

// ロック待ちの間に別の実行へ切り替わったら、古いVMへイベントを送らないこと。
// 実行対象の確認をロック取得より前に済ませてしまうと、待っている50msの間に
// 差し替わった分を見落とし、前の画面の状態を書き換えてしまう。
func TestEventRejectedWhenRunSwitchesWhileWaitingForLock(t *testing.T) {
	session := &guiSession{}
	first := session.run("「前」のボタン作成", "gui.nako3", true, nil, nil)
	if !first.OK {
		t.Fatalf("initial result = %#v", first)
	}

	// 実行中に相当する状態を作り、イベントをロック待ちに入らせる。
	session.execMu.Lock()
	started := make(chan EventStart, 1)
	go func() { started <- session.startEvent(first.RunID, 1, "click", noEvent) }()

	// 待っている間に、別の実行が始まったことにして実行対象を差し替える。
	time.Sleep(5 * time.Millisecond)
	session.mu.Lock()
	session.active = &guiExecution{id: first.RunID + 1}
	session.mu.Unlock()
	session.execMu.Unlock()

	got := <-started
	if !strings.Contains(got.Error, "古い実行結果") {
		t.Fatalf("差し替わった実行にイベントを送っている: %#v", got)
	}
	// 断ったイベントはexecMuを握ったままにしない。
	if !session.lockExecWithin(time.Second) {
		t.Fatal("断ったイベントがexecMuを解放していない")
	}
	session.execMu.Unlock()
}

// noEvent は入力欄を持たないイベント(クリックなど)に渡す空のpayload。
var noEvent = guilib.EventPayload{}

// textPayload は文字列だけの入力欄(ハンドル→値)をpayloadにする。
func textPayload(values map[string]string) guilib.EventPayload {
	fields := make(map[string]guilib.EventField, len(values))
	for handle, text := range values {
		fields[handle] = guilib.EventField{Value: text}
	}
	return guilib.EventPayload{Values: fields}
}

// dispatchEvent は画面と同じ手順でイベントを最後まで進め、届いた出力と
// 画面操作をまとめて返す。非同期実行なので、ポーリングでしか受け取れない。
func dispatchEvent(t *testing.T, session *guiSession, runID uint64, handle int, event string, payload guilib.EventPayload) RunResult {
	t.Helper()
	started := session.startEvent(runID, handle, event, payload)
	if started.Error != "" {
		return RunResult{RunID: runID, Error: started.Error}
	}
	collector := newAsyncCollector(session, started.RunID)
	status := waitForAsyncDone(t, collector)
	result := RunResult{RunID: runID}
	if status.Result != nil {
		result = *status.Result
	}
	result.Output = collector.output.String()
	result.Operations = collector.ops
	return result
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
			// done が見えた時点でも、実行goroutineはまだ作業フォルダを
			// 戻している最中かもしれない。ここで待たないと、後片付けが
			// 次のテストの作業フォルダを書き換えてしまう（→ waitForRunExit）。
			waitForRunExit(t, c.session)
			return status
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for run")
	return AsyncRunStatus{}
}

func TestBundledAsyncProgramPageUsesHTMLDialogs(t *testing.T) {
	page := bundledAsyncProgramPage(7)
	for _, want := range []string{"pollNakoRun", "resolveNakoDialog", `class="overlay"`, `data-run-id="7"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	for _, unwanted := range []string{"window.alert(", "window.prompt(", "window.confirm("} {
		if pageHas(page, unwanted) {
			t.Errorf("page unexpectedly contains native dialog %q", unwanted)
		}
	}
}

// ダイアログ待ちの実行が execMu を握ったままでも、新しい実行を始めれば
// 待機が打ち切られ、2つ目の実行がデッドロックせずに完了すること（⑫）。
//
// 打ち切りは execMu.Lock() より前に行わないと、古い実行がロックを握った
// まま解放できないため、2つ目の実行は永久に待たされる。
func TestNewRunCancelsPendingDialogAndReleasesExecMu(t *testing.T) {
	session := &guiSession{}
	firstID := session.start(`「入力」と尋ねる
「{それ}」と表示`, "first.nako3", true, nil, nil)

	first := newAsyncCollector(session, firstID)
	dialog := waitForDialog(t, first)
	if dialog.Dialog.Kind != "prompt" {
		t.Fatalf("dialog = %#v", dialog.Dialog)
	}

	// 応答を返さずに、新しい実行を始める。
	secondID := session.start(`「2つ目」と表示`, "second.nako3", true, nil, nil)
	second := newAsyncCollector(session, secondID)
	done := waitForAsyncDone(t, second)
	if done.Result == nil || !done.Result.OK {
		t.Fatalf("2つ目の実行が完了しなかった: %#v", done.Result)
	}
	if got := second.output.String(); got != "2つ目\n" {
		t.Fatalf("2つ目の出力 = %q, want %q", got, "2つ目\n")
	}

	// 打ち切られた1つ目は、利用者のキャンセルではなくエラーで終わる。
	firstDone := waitForAsyncDone(t, first)
	if firstDone.Result == nil || firstDone.Result.OK {
		t.Fatalf("打ち切られた実行が成功扱いになった: %#v", firstDone.Result)
	}
	if !strings.Contains(firstDone.Result.Error, "中断") {
		t.Fatalf("打ち切り理由が画面へ伝わっていない: %#v", firstDone.Result)
	}

	// 実行のgoroutineが作業フォルダを戻し終えるまで待つ（→ waitForRunExit）。
	waitForRunExit(t, session)
}

// ウィンドウが閉じられたとき（w.Run() から戻ったとき）に呼ぶ打ち切りで、
// 待機中の実行がエラー終了し、execMu が解放されること。
func TestCancelPendingDialogsUnblocksWaitingRun(t *testing.T) {
	session := &guiSession{}
	runID := session.start(`「閉じる前」と言う`, "close.nako3", true, nil, nil)
	collector := newAsyncCollector(session, runID)
	waitForDialog(t, collector)

	if n := session.cancelPendingDialogs(); n != 1 {
		t.Fatalf("打ち切ったダイアログ数 = %d, want 1", n)
	}
	done := waitForAsyncDone(t, collector)
	if done.Result == nil || done.Result.OK || done.Result.Error == "" {
		t.Fatalf("打ち切り後も成功扱い: %#v", done.Result)
	}

	// execMu が解放されている＝次の実行が普通に完了する。
	next := session.run(`「次の実行」と表示`, "next.nako3", true, nil, nil)
	if !next.OK || next.Output != "次の実行\n" {
		t.Fatalf("打ち切り後に次の実行が動かない: %#v", next)
	}
	// 待機が無いときの打ち切りは0件。
	if n := session.cancelPendingDialogs(); n != 0 {
		t.Fatalf("待機が無いのに %d 件打ち切った", n)
	}

	// 実行のgoroutineが作業フォルダを戻し終えるまで待つ（→ waitForRunExit）。
	waitForRunExit(t, session)
}

// 打ち切りは利用者の「キャンセル」（accepted=false, err=nil）ではなく、
// 必ずエラーとして返すこと。ここを取り違えると、打ち切りで実行が正常終了
// したように見えてしまう。
func TestCanceledDialogReturnsErrorNotCancelAnswer(t *testing.T) {
	state := &guiAsyncRun{}
	type outcome struct {
		text     string
		accepted bool
		err      error
	}
	result := make(chan outcome, 1)
	go func() {
		text, accepted, err := state.showDialog("prompt", "入力")
		result <- outcome{text: text, accepted: accepted, err: err}
	}()

	waitForStateDialog(t, state)
	if !state.cancelDialog() {
		t.Fatal("cancelDialog が待機を打ち切れなかった")
	}
	select {
	case got := <-result:
		if got.err == nil {
			t.Fatalf("打ち切りがキャンセル扱いになった: %#v", got)
		}
		if got.accepted || got.text != "" {
			t.Fatalf("打ち切りの戻り値 = %#v", got)
		}
		if !strings.Contains(got.err.Error(), "中断") {
			t.Fatalf("エラー文面 = %v", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("打ち切り後も showDialog が戻らない")
	}
}

// resolve と cancel が同時に来ても、パニックせずハングせず、どちらか一方
// だけが成立すること（チャネルの二重送信・close済みチャネルへの送信をしない）。
func TestResolveAndCancelRaceIsSafe(t *testing.T) {
	for i := 0; i < 200; i++ {
		state := &guiAsyncRun{}
		type outcome struct {
			text     string
			accepted bool
			err      error
		}
		shown := make(chan outcome, 1)
		go func() {
			text, accepted, err := state.showDialog("confirm", "競合")
			shown <- outcome{text: text, accepted: accepted, err: err}
		}()
		dialog := waitForStateDialog(t, state)

		var wg sync.WaitGroup
		var resolved, canceled bool
		wg.Add(2)
		go func() {
			defer wg.Done()
			resolved = state.resolve(dialog.ID, "ok", true)
		}()
		go func() {
			defer wg.Done()
			canceled = state.cancelDialog()
		}()
		wg.Wait()

		select {
		case got := <-shown:
			if resolved == canceled {
				t.Fatalf("resolve=%v cancel=%v が同じ結果になった: %#v", resolved, canceled, got)
			}
			if resolved && (got.err != nil || !got.accepted || got.text != "ok") {
				t.Fatalf("応答が反映されていない: %#v", got)
			}
			if canceled && got.err == nil {
				t.Fatalf("打ち切りがエラーになっていない: %#v", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("resolve/cancel の競合で showDialog が戻らない")
		}
	}
}

// 打ち切りの走査が終わった直後にダイアログが登録されても、取り残されないこと。
// 世代が変わっていれば、応答が来ないと分かるので待ちに入らずエラーで終わる。
func TestDialogRegisteredAfterCancellationFails(t *testing.T) {
	session := &guiSession{}
	state := &guiAsyncRun{session: session, dialogEpoch: session.dialogEpoch}
	// 新しい実行の開始に相当する打ち切り（この時点では待機なし）。
	session.cancelPendingDialogs()

	done := make(chan error, 1)
	go func() {
		_, _, err := state.showDialog("alert", "取り残し")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("取り残されたダイアログが応答待ちに入った")
		}
		if !strings.Contains(err.Error(), "中断") {
			t.Fatalf("エラー文面 = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("取り残されたダイアログが応答待ちのまま戻らない")
	}
}

// 応答が来ないままタイムアウトしても、打ち切りと同じくエラーで戻ること。
func TestDialogTimeoutReturnsError(t *testing.T) {
	old := guiDialogTimeout
	guiDialogTimeout = 10 * time.Millisecond
	defer func() { guiDialogTimeout = old }()

	state := &guiAsyncRun{}
	started := time.Now()
	_, accepted, err := state.showDialog("alert", "応答なし")
	if err == nil {
		t.Fatal("タイムアウトがエラーにならない")
	}
	if accepted {
		t.Fatal("タイムアウトが accepted=true になった")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("タイムアウトが効いていない: %v", elapsed)
	}
}

// バンドル版・wnako3実行画面のリロード・移動で、待機中のダイアログを
// 打ち切るフックが埋め込まれていること。
func TestBundledPageCancelsPendingDialogsOnUnload(t *testing.T) {
	page := bundledAsyncProgramPage(1)
	for _, want := range []string{"beforeunload", "pagehide", "cancelNakoDialogs", "cancelGonakoDialogs"} {
		if !pageHas(page, want) {
			t.Errorf("バンドル版ページに %q が無い", want)
		}
	}
}

// 全ページへ注入するローダーにも、同じ打ち切りフックがあること。
func TestLoaderCancelsPendingDialogsOnUnload(t *testing.T) {
	loader, err := uiFS.ReadFile(gonakoLoaderAsset)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"beforeunload", "pagehide", "cancelNakoDialogs", "cancelGonakoDialogs"} {
		if !strings.Contains(string(loader), want) {
			t.Errorf("ローダーに %q が無い", want)
		}
	}
}

// waitForRunExit は、非同期実行のgoroutineが完全に終わるまで待つ。
//
// startWithWindow のgoroutineは、実行結果を finish で公開した後に、
// 遅れて os.Chdir で作業フォルダを元へ戻し、最後に execMu を解放する。
// finish を見ただけではまだ後片付けが残っており、ここで待たずにテストを
// 終えると、プロセス全体の作業フォルダがその後のテスト（include_build_test.go
// など）の途中で書き換わってしまう。execMu を取れることが、
// 後片付けまで終わった印になる（→ startWithWindow の defer の順序）。
func waitForRunExit(t *testing.T, session *guiSession) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		session.execMu.Lock()
		session.execMu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("実行のgoroutineが終わらなかった")
	}
}

// waitForStateDialog は guiAsyncRun にダイアログが登録されるまで待つ。
func waitForStateDialog(t *testing.T, state *guiAsyncRun) *DialogRequest {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if status := state.status(); status.Dialog != nil {
			return status.Dialog
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ダイアログが登録されなかった")
	return nil
}
