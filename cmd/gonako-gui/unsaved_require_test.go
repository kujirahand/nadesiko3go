package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 保存前のエディタ内容で、ファイルを取り込む構文があれば保存を促す（#542）。
func TestRunUnsavedEditorRequireAsksToSave(t *testing.T) {
	session := &guiSession{}
	res := runNakoFileForTest(session, "", "!「sub.nako3」を取り込む。\n「本体」と表示。", "", false)
	if res.OK {
		t.Fatal("保存前の取り込みが実行できてしまった")
	}
	if !strings.Contains(res.Error, "保存") {
		t.Fatalf("保存を促すエラーではありません: %q", res.Error)
	}
}

// 保存済みなら、取り込み元と同じフォルダの裸のファイル名を取り込める。
func TestRunSavedEditorRequireFindsSiblingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sub.nako3"), []byte("「子」と表示。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.nako3")
	session := &guiSession{}
	res := runNakoFileForTest(session, "", "!「sub.nako3」を取り込む。\n", main, false)
	if !res.OK {
		t.Fatalf("取り込みに失敗: %q", res.Error)
	}
}

func TestCheckNakoSyntaxUnsavedRequire(t *testing.T) {
	res := checkNakoSyntax("!「sub.nako3」を取り込む。\n", "", lintModeGonako)
	if res.OK || !strings.Contains(res.Error, "保存") {
		t.Fatalf("保存前の取り込み: OK=%v error=%q", res.OK, res.Error)
	}
	// 取り込みが無ければ、保存前でも他の構文エラーは通常どおり報告される。
	if res := checkNakoSyntax("もし「A」ならば\n「B」と表示。", "", lintModeGonako); res.OK || strings.Contains(res.Error, "保存") {
		t.Fatalf("構文エラー: OK=%v error=%q", res.OK, res.Error)
	}
	// 取り込みがなければ従来どおり通る。
	if res := checkNakoSyntax("「あ」と表示。\n", "", lintModeGonako); !res.OK {
		t.Fatalf("取り込みなし: %q", res.Error)
	}
}

func TestLaunchWindowUnsavedRequireAsksToSave(t *testing.T) {
	res := launchNakoWindowProcess("!「sub.nako3」を取り込む。\n", "")
	if res.OK || !strings.Contains(res.Error, "保存") {
		t.Fatalf("別ウィンドウ実行: %+v", res)
	}
}
