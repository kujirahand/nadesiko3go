package main

import (
	"strings"
	"testing"
	"time"
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
