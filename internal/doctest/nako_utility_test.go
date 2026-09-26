package doctest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// なでしこ版を実際のランタイムで起動し、抽出・照合と終了状態を確認する。
func TestNakoUtility(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "gonako runtime")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/gonako")
	build.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOROOT=") {
			build.Env = append(build.Env, entry)
		}
	}
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("ビルド: %v\n%s", err, output)
	}
	// installの生成物からパス・引数・終了コードがそのまま渡ることを確認する。
	toolSource := filepath.Join(dir, "空白 ' $ utility.nako3")
	if err := os.WriteFile(toolSource, []byte("コマンドラインをJSONエンコードして表示。\n7で強制終了。"), 0600); err != nil {
		t.Fatal(err)
	}
	install := exec.Command(binary, "install", toolSource, "--name", "test-tool")
	install.Dir = dir
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, output)
	}
	installed := filepath.Join(dir, "bin", "test-tool")
	var launch *exec.Cmd
	if runtime.GOOS == "windows" {
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			launch = exec.Command(pwsh, "-NoProfile", "-File", installed+".ps1", "空白 引数", "literal$(echo injected)")
		}
	} else {
		launch = exec.Command(installed, "空白 引数", "literal$(echo injected)")
	}
	if launch != nil {
		launch.Dir = dir
		output, err := launch.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 7 || strings.TrimSpace(string(output)) != `["空白 引数","literal$(echo injected)"]` {
			t.Fatalf("生成スクリプトの実行: %v\n%s", err, output)
		}
	}
	source := filepath.Join(root, "gonako-package/doctest.nako3")
	fixture := filepath.Join(dir, "sample.txt")
	text := "{{{#nako3\r\n「前」と表示。\r\n### 表示結果： 前\r\n### 後\r\n「後」と表示。\r\n}}}\r\n{{{#nako3\r\n「独自」と表示。\r\n### L表示結果: 独自\r\n}}}\r\n{{{#nako3\r\n「説明」と表示。\r\n}}}\r\n"
	if err := os.WriteFile(fixture, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	tempRoot := filepath.Join(dir, "temporary")
	if err := os.Mkdir(tempRoot, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(wantCode int, want string, args ...string) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{source}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GONAKO_DOCTEST_RUNTIME="+binary, "TMPDIR="+tempRoot, "TMP="+tempRoot, "TEMP="+tempRoot)
		output, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != wantCode || !strings.Contains(string(output), want) {
			t.Fatalf("args=%q: code=%d, 出力=%s; 期待=%d, %s", args, code, output, wantCode, want)
		}

		entries, err := os.ReadDir(tempRoot)
		if err != nil || len(entries) != 0 {
			t.Fatalf("一時フォルダが残りました: %v %v", entries, err)
		}

		err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if strings.HasPrefix(entry.Name(), ".gonako-doctest-") {
				t.Errorf("一時ソースが残りました: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

	}
	run(0, "1件成功", fixture)
	run(0, "1件成功", "--runtime", binary, "--subcommand", "run", "--label", "### L表示結果：", fixture)
	run(0, "1件成功", "--label=L表示結果", fixture)
	run(2, "引数が不足", "--runtime")
	run(2, "maxは", "-max=-1", fixture)
	run(2, "未対応の引数", "--unknown")
	run(2, "一緒に", "--subcommand=run", fixture)
	run(2, "見つかりません", filepath.Join(dir, "missing.txt"))
	if err := os.WriteFile(fixture, []byte("{{{#nako3\n「違う」と表示。\n### 表示結果: 期待\n}}}\n{{{#nako3\n「違う」と表示。\n### 表示結果: 期待\n}}}"), 0600); err != nil {
		t.Fatal(err)
	}
	run(1, "2件失敗", "-max", "1", fixture)
	if err := os.WriteFile(fixture, []byte("{{{#nako3\n「JS実行」と表示。\n### 表示結果: JS実行\n}}}"), 0600); err != nil {
		t.Fatal(err)
	}
	run(0, "1件省略", fixture)
	run(0, "1件成功・0件省略", "--runtime="+binary, fixture)

	// 子フォルダを含む探索と、元テキスト横の相対ライブラリの取込を確認する。
	nested := filepath.Join(dir, "examples", "child")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "lib.nako3"), []byte("●ライブラリ実行とは\n「隣」と表示\nここまで"), 0600); err != nil {
		t.Fatal(err)
	}
	caseText := "{{{#nako3\n!「./lib.nako3」を取り込む。\nライブラリ実行。\n### 表示結果: 隣\n}}}"
	if err := os.WriteFile(filepath.Join(nested, "case.txt"), []byte(caseText), 0600); err != nil {
		t.Fatal(err)
	}
	run(0, "1件成功", filepath.Join(dir, "examples"))
	run(0, "1件成功", "--runtime="+binary, filepath.Join(dir, "examples"))
	// 一時ソースを書いた直後に実行エラーを起こし、異常経路でも削除を確認する。
	original := source
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(data), "実行結果=実行設定をコマンド実行待機。", "「後始末試験」のエラー発生。\n実行結果=実行設定をコマンド実行待機。", 1)
	source = filepath.Join(dir, "broken-doctest.nako3")
	if err := os.WriteFile(source, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	run(2, "後始末試験", "--runtime="+binary, fixture)

	broken = strings.Replace(string(data), "コードを一時ソースに保存。", "「保存試験」のエラー発生。", 1)
	if err := os.WriteFile(source, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	run(2, "保存試験", "--runtime="+binary, fixture)
	source = original
	// ビルドした実行ファイルから埋め込みの省略形で取り込む。
	cmd := exec.Command(binary, "-e", "!「doctest」を取り込む。", "--help")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "使い方:") {
		t.Fatalf("埋込取込: %v\n%s", err, output)
	}
}
