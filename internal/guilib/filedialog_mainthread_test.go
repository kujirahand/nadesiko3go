package guilib

import (
	"os"
	"strings"
	"testing"
)

// macOS のファイルダイアログ（NSOpenPanel / NSSavePanel）は AppKit の都合で
// メインスレッド専用。『ファイル選択』などは startNakoFile やイベント処理の
// バックグラウンド goroutine から呼ばれるため、filedialog_darwin.m は
// メインスレッド判定と main queue へのディスパッチを持ち続ける必要がある。
//
// このテストは macOS 実機が無い環境（CI の Linux など）でも回帰を検出できる
// ように、ソース構造だけを確認する。実際の AppKit 動作は実機でしか確認できない。
func TestDarwinFileDialogRunsOnMainThread(t *testing.T) {
	data, err := os.ReadFile("filedialog_darwin.m")
	if err != nil {
		t.Fatalf("filedialog_darwin.m が読めない: %v", err)
	}
	source := string(data)

	// 別スレッドからはメインキューへ投げ、セマフォで完了を待つ。
	for _, want := range []string{
		"[NSThread isMainThread]",
		"dispatch_get_main_queue()",
		"dispatch_async",
		"dispatch_semaphore_wait",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("filedialog_darwin.m に %q が無い（メインスレッド実行が失われている）", want)
		}
	}

	// dispatch_sync はメインスレッドから呼ぶとデッドロックするので使わない。
	if strings.Contains(source, "dispatch_sync(") {
		t.Fatal("filedialog_darwin.m が dispatch_sync を呼んでいる（メインスレッドから呼ぶとデッドロックする）")
	}

	for _, name := range []string{"gonakoShowOpenPanel", "gonakoShowSavePanel", "gonakoShowFolderPanel"} {
		body := darwinDialogFunctionBody(t, source, name)
		mainCall := strings.Index(body, "gonakoOnMainThread(^{")
		if mainCall < 0 {
			t.Fatalf("%s がメインスレッド実行のヘルパーを通っていない", name)
		}
		// パネルの生成・runModal・ウィンドウの前面化は、すべてメインスレッド
		// 実行ブロックの内側で行う。
		for _, want := range []string{"[NSApp activateIgnoringOtherApps:YES]", "[panel runModal]"} {
			pos := strings.Index(body, want)
			if pos < 0 {
				t.Fatalf("%s に %q が無い", name, want)
			}
			if pos < mainCall {
				t.Fatalf("%s が %q をメインスレッド実行ブロックの外で呼んでいる", name, want)
			}
		}
	}
}

// darwinDialogFunctionBody は .m のソースから 1 関数分の本体を取り出す。
// 実装の目印に依存しすぎないよう、関数の閉じ括弧（行頭の "}"）までを返す。
func darwinDialogFunctionBody(t *testing.T, source, name string) string {
	t.Helper()
	start := strings.Index(source, "char *"+name+"(")
	if start < 0 {
		t.Fatalf("%s の定義が見つからない", name)
	}
	rest := source[start:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("%s の終端が見つからない", name)
	}
	return rest[:end]
}
