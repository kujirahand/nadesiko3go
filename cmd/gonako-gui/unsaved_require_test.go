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

// 別ウィンドウの子プロセスには、保存前であることを引数で伝える（#542）。
func TestChildRunArgsPassUnsaved(t *testing.T) {
	has := func(args []string) bool {
		for _, a := range args {
			if a == "--run-window-unsaved" {
				return true
			}
		}
		return false
	}
	if !has(childRunArgs("--run-window", "t.nako3", "なでしこ3", "gui.nako3", true)) {
		t.Error("保存前なのに --run-window-unsaved が渡されません")
	}
	if has(childRunArgs("--run-window", "t.nako3", "a.nako3", "/x/a.nako3", false)) {
		t.Error("保存済みなのに --run-window-unsaved が渡されています")
	}
	if has(childRunArgs("--run-wnako3", "t.nako3", "なでしこ3", "gui.nako3", true)) {
		t.Error("wnako3モードには渡さない")
	}
}

// 作業フォルダと GONAKO_PACKAGE_PATH の両方に同名ファイルがあるとき、
// 保存前の子プロセスは、エディタ内実行と同じくパッケージ版を選ぶ。
// 保存前の目印が無いと、ローカル優先の探索で作業フォルダ版を選んでしまう。
func TestUnsavedRunPrefersPackageOverWorkDirFile(t *testing.T) {
	work := t.TempDir()
	pkg := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "helper.nako3"), []byte("「ローカル版」と表示。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "helper.nako3"), []byte("「パッケージ版」と表示。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GONAKO_PACKAGE_PATH", pkg)
	code := "!「helper.nako3」を取り込む。\n"
	run := func(sourcePath string) string {
		t.Helper()
		session := &guiSession{}
		res := session.runWithWindowSource(code, "gui.nako3", sourcePath, true, nil, nil, newVirtualWindowController(), work)
		if !res.OK {
			t.Fatalf("実行に失敗: %q", res.Error)
		}
		return res.Output
	}
	if out := run(unsavedSourcePath(true)); !strings.Contains(out, "パッケージ版") {
		t.Errorf("保存前の出力 = %q, パッケージ版を期待", out)
	}
	// 目印が無い場合は作業フォルダ版になる（この差を子へ伝える必要がある理由）。
	if out := run(unsavedSourcePath(false)); !strings.Contains(out, "ローカル版") {
		t.Errorf("目印なしの出力 = %q, ローカル版を期待", out)
	}
}
