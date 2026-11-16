package main

// wnako3の命令ブリッジ（commandBridge）のダイアログ打ち切りに関する回帰テスト。
//
// ページをリロードすると、gonako-loader.jsのbeforeunload/pagehideが
// cancelGonakoDialogsを呼ぶ。ここがcommandBridge.showDialogの待機を解かないと、
// 待機中のcallがb.muを握ったままbridgeDialogTimeout（30分）残り、新ページから
// の命令呼び出しがすべて止まる（レビュー指摘）。

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// ダイアログ待機中にcancelPendingDialogsが呼ばれたら、待機中のcallが
// 非nilのエラーで終わってb.muを解放し、次のcallがデッドロックせず通ること。
func TestCommandBridgeCancelPendingDialogsUnblocksCall(t *testing.T) {
	evaluated := make(chan string, 4)
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(js string) {
		evaluated <- js
	})

	done := make(chan BridgeResult, 1)
	go func() { done <- bridge.call("言", `["こんにちは"]`) }()

	select {
	case js := <-evaluated:
		if !strings.Contains(js, "__gonakoBridgeDialog") {
			t.Fatalf("ダイアログ表示のEvalではない: %s", js)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ダイアログ表示のEvalが呼ばれていない")
	}
	waitForBridgeDialog(t, bridge)

	// ページのリロードに相当する打ち切り。
	if n := bridge.cancelPendingDialogs(); n != 1 {
		t.Fatalf("打ち切ったダイアログ数 = %d, want 1", n)
	}

	select {
	case result := <-done:
		if result.OK {
			t.Fatalf("打ち切られたcallが成功扱いになった: %+v", result)
		}
		if !strings.Contains(result.Error, "中断") {
			t.Fatalf("打ち切りのエラー文面 = %q", result.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("打ち切り後もcallが戻らない（b.muが解放されない）")
	}

	// 待っていたcallがb.muを解放しているので、次の呼び出しはすぐ通ること。
	next := make(chan BridgeResult, 1)
	go func() { next <- bridge.call("文字数", `["abc"]`) }()
	select {
	case result := <-next:
		if !result.OK || string(result.Value) != "3" {
			t.Fatalf("打ち切り後の呼び出し結果が違う: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("打ち切り後もb.muが解放されず、次の呼び出しが進まない")
	}

	// 打ち切りの後に始まった呼び出しのダイアログは、普通に応答できること。
	// 世代を進めたせいで新しいダイアログまで打ち切ってはいけない。
	dialogDone := make(chan BridgeResult, 1)
	go func() { dialogDone <- bridge.call("言", `["もう一度"]`) }()
	select {
	case js := <-evaluated:
		if !strings.Contains(js, "__gonakoBridgeDialog") {
			t.Fatalf("2度目のダイアログ表示のEvalではない: %s", js)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("打ち切り後の新しい呼び出しがダイアログを出さない")
	}
	id := waitForBridgeDialog(t, bridge)
	if !bridge.resolveDialog(id, "", true) {
		t.Fatal("打ち切り後の新しいダイアログに応答できない")
	}
	select {
	case result := <-dialogDone:
		if !result.OK {
			t.Fatalf("打ち切り後の新しいダイアログ呼び出しが失敗: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("新しいダイアログへの応答後もcallが完了しない")
	}

	// 待機が無いときの打ち切りは0件。
	if n := bridge.cancelPendingDialogs(); n != 0 {
		t.Fatalf("待機が無いのに %d 件打ち切った", n)
	}
}

// resolveDialogとcancelPendingDialogsが同時に来ても、パニックせずハングせず、
// どちらか一方だけが成立すること（二重送信・close済みチャネルへの送信をしない）。
func TestCommandBridgeResolveAndCancelRaceIsSafe(t *testing.T) {
	for i := 0; i < 200; i++ {
		// showDialogを直接呼ぶのでVMの準備は不要。世代を毎回0から始めるため
		// 反復ごとに新しいブリッジを作る。
		bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
		type outcome struct {
			text     string
			accepted bool
			err      error
		}
		shown := make(chan outcome, 1)
		go func() {
			text, accepted, err := bridge.showDialog("confirm", "競合")
			shown <- outcome{text: text, accepted: accepted, err: err}
		}()
		id := waitForBridgeDialog(t, bridge)

		var wg sync.WaitGroup
		var resolved, canceled bool
		wg.Add(2)
		go func() {
			defer wg.Done()
			resolved = bridge.resolveDialog(id, "ok", true)
		}()
		go func() {
			defer wg.Done()
			canceled = bridge.cancelPendingDialogs() > 0
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
			if canceled && (got.err == nil || got.accepted || got.text != "") {
				t.Fatalf("打ち切りがエラーになっていない: %#v", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("resolve/cancelの競合でshowDialogが戻らない")
		}
	}
}

// タイムアウトと打ち切りが競合しても、打ち切りのエラーが失われないこと。
// abandonDialogが「登録が消えている＝決着済み」としてチャネルを読む経路の確認。
func TestCommandBridgeAbandonDialogKeepsCanceledAnswer(t *testing.T) {
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	answer := make(chan dialogAnswer, 1)
	bridge.dialogAnswers = map[uint64]chan dialogAnswer{1: answer}

	if n := bridge.cancelPendingDialogs(); n != 1 {
		t.Fatalf("打ち切ったダイアログ数 = %d, want 1", n)
	}
	result, ok := bridge.abandonDialog(1, answer)
	if !ok {
		t.Fatal("打ち切り済みのダイアログが決着済みとして扱われていない")
	}
	if result.err == nil || !strings.Contains(result.err.Error(), "中断") {
		t.Fatalf("打ち切りのエラーが失われている: %+v", result)
	}
}

// 打ち切りの走査が終わった直後にダイアログが登録されても、取り残されないこと。
// 世代が変わっていれば応答は来ないと分かるので、待ちに入らずエラーで終わる。
func TestCommandBridgeDialogRegisteredAfterCancellationFails(t *testing.T) {
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	// callが始まった時点の世代を控えた状態を作る。
	bridge.callEpoch = bridge.dialogEpoch
	// その後にリロード（打ち切り）が起きた。まだ待機は登録されていない。
	bridge.cancelPendingDialogs()

	done := make(chan error, 1)
	go func() {
		_, _, err := bridge.showDialog("alert", "取り残し")
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

// タイムアウトとcancelPendingDialogsが同時に起きても、パニック・ハングせず
// 必ず非nilのエラーで戻ること。
func TestCommandBridgeTimeoutAndCancelRaceIsSafe(t *testing.T) {
	old := bridgeDialogTimeout
	bridgeDialogTimeout = time.Millisecond
	defer func() { bridgeDialogTimeout = old }()

	for i := 0; i < 100; i++ {
		bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
		done := make(chan error, 1)
		go func() {
			_, _, err := bridge.showDialog("alert", "競合")
			done <- err
		}()
		bridge.cancelPendingDialogs()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("タイムアウト/打ち切りがエラーになっていない")
			}
		case <-time.After(time.Second):
			t.Fatal("タイムアウト/打ち切りの競合でshowDialogが戻らない")
		}
	}
}

// ページ遷移より前に始まった呼び出し（b.muを待っていたものも含む）は、
// 遷移後のページで応答できないので、ダイアログを出さずにエラーで終わること。
func TestCommandBridgeStaleCallFailsWithoutWaiting(t *testing.T) {
	bridge := newCommandBridge(newVirtualWindowController(), nil, func(string) {})
	// リロードに相当する打ち切り。この後に古い世代の呼び出しが走る。
	bridge.cancelPendingDialogs()

	done := make(chan BridgeResult, 1)
	go func() { done <- bridge.callAtEpoch("言", `["古い呼び出し"]`, 0) }()
	select {
	case result := <-done:
		if result.OK {
			t.Fatalf("古い世代の呼び出しが成功扱いになった: %+v", result)
		}
		if !strings.Contains(result.Error, "中断") {
			t.Fatalf("エラー文面 = %q", result.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("古い世代の呼び出しがダイアログ待ちのまま戻らない")
	}

	// b.muが解放されているので、新しい世代の呼び出しは通る。
	if result := bridge.call("文字数", `["abc"]`); !result.OK || string(result.Value) != "3" {
		t.Fatalf("後続の呼び出しが失敗: %+v", result)
	}
}

// waitForBridgeDialog は commandBridge にダイアログが登録されるまで待ち、
// そのIDを返す。
func waitForBridgeDialog(t *testing.T, bridge *commandBridge) uint64 {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		bridge.dialogMu.Lock()
		for id := range bridge.dialogAnswers {
			bridge.dialogMu.Unlock()
			return id
		}
		bridge.dialogMu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ダイアログが登録されなかった")
	return 0
}
