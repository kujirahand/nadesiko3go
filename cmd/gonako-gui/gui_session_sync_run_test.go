package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// UIスレッド（WebViewのバインド呼び出し）経由の同期実行が、バックグラウンド
// 実行の execMu を無制限に待たないこと。
//
// 待ってしまうと、バックグラウンド実行がネイティブのファイルダイアログを
// main queue へディスパッチしている間、UIスレッドは main queue を回せないまま
// execMu を待ち続け、互いに待ち合ってウィンドウごと固まる。
func TestSyncRunReturnsErrorWhileExecMuHeld(t *testing.T) {
	session := &guiSession{}
	session.execMu.Lock()

	started := make(chan RunResult, 1)
	go func() {
		started <- session.run("「あ」と言う", "gui.nako3", true, nil, nil)
	}()

	select {
	case result := <-started:
		if result.OK {
			t.Fatalf("実行中の同期実行が成功してしまった: %#v", result)
		}
		if !strings.Contains(result.Error, "実行中") {
			t.Fatalf("実行中であることを示すエラーでない: %q", result.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execMu を待って同期実行が固まっている")
	}

	// テストが握っている分を離してから、断った実行が execMu を握ったままに
	// していないことを確かめる。
	session.execMu.Unlock()
	if !session.lockExecWithin(time.Second) {
		t.Fatal("断った同期実行が execMu を解放していない")
	}
	session.execMu.Unlock()

	// 実行が終われば、同じセッションで普通に動く。
	result := session.run("「あ」と言う", "gui.nako3", true, nil, nil)
	if !result.OK {
		t.Fatalf("解放後の同期実行が失敗した: %#v", result)
	}
}

// 将来 runCompiled を呼び出す経路でも、実行中は同期呼び出しを断る。
func TestSyncRunCompiledReturnsErrorWhileExecMuHeld(t *testing.T) {
	session := &guiSession{}
	screen := guilib.NewScreen()
	registry := stdlib.NewRegistry(guiPluginsWith(session.pluginForScreen(screen))...)
	prog, err := vm.CompileWithRegistry("「あ」を表示", "gui.nako3", registry)
	if err != nil {
		t.Fatalf("コンパイルに失敗した: %v", err)
	}
	session.execMu.Lock()
	started := make(chan RunResult, 1)
	go func() { started <- session.runCompiled(prog, nil, nil) }()
	select {
	case result := <-started:
		if result.OK || result.Error != errExecBusy {
			t.Fatalf("実行中の同期実行の結果が不正: %#v", result)
		}
	case <-time.After(2 * time.Second):
		session.execMu.Unlock()
		t.Fatal("execMu を待って同期実行が固まっている")
	}
	session.execMu.Unlock()
	if !session.lockExecWithin(time.Second) {
		t.Fatal("断った同期実行が execMu を解放していない")
	}
	session.execMu.Unlock()
	if result := session.runCompiled(prog, nil, nil); !result.OK {
		t.Fatalf("解放後の同期実行が失敗した: %#v", result)
	}
}

// 同期実行の予約だけでは、応答待ちの非同期実行を変更しない。
func TestReserveSyncRunPreservesAsyncDialog(t *testing.T) {
	session := &guiSession{}
	runID := session.start("「入力」と尋ねる", "dialog.nako3", true, nil, nil)
	t.Cleanup(func() { session.cancelPendingDialogs() })
	collector := newAsyncCollector(session, runID)
	dialog := waitForDialog(t, collector)
	reservedID, state := session.reserveRun(false)
	if reservedID != runID+1 || state != nil {
		t.Fatalf("同期実行の予約が不正: id=%d state=%v", reservedID, state)
	}
	status := collector.poll()
	if status.Done || status.Dialog == nil || status.Dialog.ID != dialog.Dialog.ID {
		t.Fatalf("予約後のダイアログが不正: %#v", status)
	}
	if !session.resolveDialog(runID, dialog.Dialog.ID, "回答", true) {
		t.Fatal("元の実行にダイアログを応答できない")
	}
	if done := waitForAsyncDone(t, collector); done.Result == nil || !done.Result.OK || done.Result.RunID != runID {
		t.Fatalf("元の実行が完了しない: %#v", done.Result)
	}
}
