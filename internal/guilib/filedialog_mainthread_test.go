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
	source := stripCComments(string(data))

	// 別スレッドからはメインキューへ投げ、セマフォで完了を待つ。
	for _, want := range []string{
		"[NSThread isMainThread]",
		"dispatch_get_main_queue()",
		"dispatch_async",
		"dispatch_semaphore_wait",
		"dispatch_semaphore_signal",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("filedialog_darwin.m に %q が無い（メインスレッド実行が失われている）", want)
		}
	}

	start := strings.Index(source, "static void gonakoOnMainThread(")
	if start < 0 {
		t.Fatal("gonakoOnMainThread の定義が無い")
	}
	end := strings.Index(source[start:], "\n}\n")
	if end < 0 {
		t.Fatal("gonakoOnMainThread の終端が無い")
	}
	helper := source[start : start+end]
	mainThread := strings.Index(helper, "[NSThread isMainThread]")
	async := strings.Index(helper, "dispatch_async(")
	wait := strings.Index(helper, "dispatch_semaphore_wait(")
	if mainThread < 0 || async < 0 || wait < 0 || mainThread > async || mainThread > wait {
		t.Fatal("メインスレッド判定が非同期ディスパッチまたはセマフォ待ちより前にない")
	}
	// release がすべて ARC 無効の分岐内にあることを確かめる。
	// 現行ソースの否定形の has_feature ガードを前提とし、入れ子の #if は追跡しない。
	guarded := false
	foundRelease := false
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "#if !__has_feature(objc_arc)" {
			guarded = true
			continue
		}
		if strings.HasPrefix(line, "#endif") || strings.HasPrefix(line, "#else") || strings.HasPrefix(line, "#elif") {
			guarded = false
		}
		if strings.Contains(line, "dispatch_release(") {
			foundRelease = true
			if !guarded {
				t.Fatal("dispatch_release が ARC ガードの外にある")
			}
		}
	}
	if !foundRelease {
		t.Fatal("dispatch_release が無い")
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

// 文字列と文字定数の中はそのまま残し、コメントだけを空白に置き換える。
func stripCComments(source string) string {
	b := []byte(source)
	for i := 0; i < len(b); {
		if b[i] == '"' || b[i] == '\'' {
			quote := b[i]
			i++
			for i < len(b) {
				if b[i] == '\\' && i+1 < len(b) {
					i += 2
					continue
				}
				if b[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}
		if i+1 < len(b) && b[i] == '/' && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				b[i] = ' '
				i++
			}
			continue
		}
		if i+1 < len(b) && b[i] == '/' && b[i+1] == '*' {
			b[i], b[i+1] = ' ', ' '
			i += 2
			for i < len(b) {
				if i+1 < len(b) && b[i] == '*' && b[i+1] == '/' {
					b[i], b[i+1] = ' ', ' '
					i += 2
					break
				}
				if b[i] != '\n' {
					b[i] = ' '
				}
				i++
			}
			continue
		}
		i++
	}
	return string(b)
}
