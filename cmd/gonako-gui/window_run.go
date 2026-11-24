package main

// エディタの「ウィンドウ(GUI)」実行モードを支える (#97)。
//
// エディタが直接そのウィンドウの中でプログラムを動かすと、『母艦のウィンドウ
// 変更』がエディタ自身のウィンドウを操作してしまう。これを避けるため、
// このモードでは gonako-gui 自身を子プロセスとして起動し、そちらに新しい
// ウィンドウを作らせて実行させる。子プロセスの実行画面は、変換済みアプリと
// 同じ ui/bundled/ の資材（bundledAsyncProgramPage）を使い回す。

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// NewWindowRunResult is what runNakoInNewWindow reports back to the editor.
type NewWindowRunResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// launchNakoWindowProcess starts a new gonako-gui process that runs code in
// its own window, separate from the editor's. It returns as soon as the
// process has started; it does not wait for the program to finish.
func launchNakoWindowProcess(code, filePath string) NewWindowRunResult {
	// 保存前のプログラムは取り込み元のフォルダが無い。子プロセスを起動してから
	// 失敗させず、ここで先に保存を促す（#542）。
	if filePath == "" {
		if err := parser.CheckUnsavedRequires(code, vm.RuntimeFuncList()); err != nil {
			return NewWindowRunResult{Error: err.Error()}
		}
	}
	return launchChildRunProcess(code, filePath, "--run-window")
}

// childRunArgs は子プロセスへ渡す引数を作る。保存前のプログラム（unsaved）は
// 表示名が gui.nako3 になるだけで、取り込み元のフォルダが無い。そのことを
// --run-window-unsaved で子へ伝え、子の取り込み解決を親やエディタ内実行と
// 揃える（#542）。この目印は「ウィンドウ(GUI)」モードだけが使う。
func childRunArgs(runFlag, tmpPath, title, sourceName string, unsaved bool) []string {
	args := []string{runFlag, tmpPath, "--run-window-title", title, "--run-window-name", sourceName}
	if unsaved && runFlag == "--run-window" {
		args = append(args, "--run-window-unsaved")
	}
	return args
}

// launchChildRunProcess は gonako-gui 自身を子プロセスとして起動し、
// 一時ファイルに書いたプログラムを runFlag で指定した実行モードで動かす。
// 「ウィンドウ(GUI)」（--run-window）と「ブラウザ(wnako3)」（--run-wnako3）で共用する。
func launchChildRunProcess(code, filePath, runFlag string) NewWindowRunResult {
	self, err := os.Executable()
	if err != nil {
		return NewWindowRunResult{Error: fmt.Sprintf("自分自身の実行ファイルを取得できません: %v", err)}
	}

	tmp, err := os.CreateTemp("", "gonako-gui-run-*.nako3")
	if err != nil {
		return NewWindowRunResult{Error: fmt.Sprintf("一時ファイルを作成できません: %v", err)}
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(code); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return NewWindowRunResult{Error: fmt.Sprintf("一時ファイルへ書き込めません: %v", err)}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return NewWindowRunResult{Error: fmt.Sprintf("一時ファイルを閉じられません: %v", err)}
	}

	title := "なでしこ3"
	sourceName := "gui.nako3"
	workDir := ""
	if filePath != "" {
		title = filepath.Base(filePath)
		sourceName = filePath
		if dir := filepath.Dir(filePath); dir != "." {
			if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
				workDir = dir
			}
		}
	}

	cmd := exec.Command(self, childRunArgs(runFlag, tmpPath, title, sourceName, filePath == "")...)
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return NewWindowRunResult{Error: fmt.Sprintf("新しいウィンドウを起動できません: %v", err)}
	}
	go func() {
		_ = cmd.Wait()
		os.Remove(tmpPath)
	}()
	return NewWindowRunResult{OK: true}
}

// readStandaloneWindowSource は単独ウィンドウで実行するプログラムを読む。
// removeSource が真のときだけ、読み込み後にファイルを削除する。削除するのは
// エディタが作った一時ファイルだけであり、公開コマンド `gonako-gui run` で
// 利用者が指定したファイルは削除しない (Issue #482)。
func readStandaloneWindowSource(sourcePath string, removeSource bool) ([]byte, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	if removeSource {
		_ = os.Remove(sourcePath)
	}
	return data, nil
}

// runStandaloneWindow is the child-process entry point launched by
// launchNakoWindowProcess. It reads the program from sourcePath, opens a
// fresh native window for it, and runs it there — that window, not the
// editor's, is what 『母艦のウィンドウ変更』 controls. sourceName is the
// original .nako3 path (or a fallback), used only so compile/runtime errors
// point at the file the user actually edited instead of the temp file that
// carried the source text across processes.
func runStandaloneWindow(sourcePath, title, sourceName string, unsaved bool) {
	runStandaloneWindowWithSourceRemoval(sourcePath, title, sourceName, true, unsaved)
}

// runStandaloneWindowWithSourceRemoval はGUIプログラムを専用ウィンドウで
// 実行する。removeSource が真のときは読み込み後に sourcePath を削除する
// (エディタが渡す一時ファイル用)。公開コマンド `gonako-gui run` からは
// removeSource=false で呼び、利用者のファイルを残す。
func runStandaloneWindowWithSourceRemoval(sourcePath, title, sourceName string, removeSource, unsaved bool) {
	data, err := readStandaloneWindowSource(sourcePath, removeSource)
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("プログラムを読み込めません: %v", err))
		return
	}
	runStandaloneWindowProgramFrom(data, title, sourceName, unsavedSourcePath(unsaved))
}

// unsavedSourcePath は保存前のとき、取り込み解決の起点として渡す目印を返す。
// 保存済みなら空文字で、表示名のファイル自身を起点にする。
func unsavedSourcePath(unsaved bool) string {
	if unsaved {
		return parser.UnsavedSourcePath
	}
	return ""
}

// runStandaloneWindowProgram は読み込み済みのプログラムを専用ウィンドウで
// 実行する。公開コマンド `gonako-gui run` は、事前のコンパイル確認で読んだ
// 内容をそのまま渡すためにこちらを使う。
func runStandaloneWindowProgram(data []byte, title, sourceName string) {
	runStandaloneWindowProgramFrom(data, title, sourceName, "")
}

// runStandaloneWindowProgramFrom は runStandaloneWindowProgram に、取り込み解決の
// 起点 sourcePath を足したもの。空なら sourceName を起点にする（#542）。
func runStandaloneWindowProgramFrom(data []byte, title, sourceName, sourcePath string) {
	if title == "" {
		title = "なでしこ3"
	}
	if sourceName == "" {
		sourceName = "gui.nako3"
	}
	session := &guiSession{}
	page := &ownedPageHandler{}
	server, err := startOwnedServer(page)
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("ローカルサーバーを起動できません: %v", err))
		return
	}
	tw, err := newTrustedWindowWithServer(os.Getenv("GONAKO_DEBUG") == "1", server, defaultWindowSettings(title, 960, 640))
	if err != nil {
		server.Close()
		showMessageWindow(title, fmt.Sprintf("ウィンドウを作成できません: %v", err))
		return
	}
	defer tw.Close()
	session.window = newNativeWindowController(tw.Window)
	_ = tw.Window.Bind("startNakoEvent", func(runID uint64, handle int, event string, payload guilib.EventPayload) string {
		b, _ := json.Marshal(session.startEvent(runID, handle, event, payload))
		return string(b)
	})
	// サンプルの「ウィンドウ(GUI)」と gonako-gui run は、この単独ウィンドウで
	// bundled/app.js を表示する。画面側は本体終了後に pumpNakoTimers を呼ぶ
	// ので、エディタと梱包実行と同じバインドが要る (#528)。
	_ = tw.Window.Bind("pumpNakoTimers", func(runID uint64) string {
		b, _ := json.Marshal(session.startPump(runID))
		return string(b)
	})
	_ = tw.Window.Bind("pollNakoRun", func(runID uint64) string {
		b, _ := json.Marshal(session.poll(runID))
		return string(b)
	})
	_ = tw.Window.Bind("resolveNakoDialog", func(runID, dialogID uint64, text string, accepted bool) bool {
		return session.resolveDialog(runID, dialogID, text, accepted)
	})
	// ページのリロード・移動で、応答を待っているダイアログを打ち切る。
	_ = tw.Window.Bind("cancelNakoDialogs", func() {
		session.cancelPendingDialogs()
	})
	_ = tw.Window.Bind("closeBundledWindow", func() {
		tw.Window.Terminate()
	})
	// Bindの失敗はプログラムを走らせる前に返す。ここで諦めれば、実行画面から
	// 呼べない命令を残したまま利用者のプログラムを開始しない。
	if err := tw.BindErr(); err != nil {
		tw.Close()
		showMessageWindow(title, fmt.Sprintf("Bindを登録できません: %v", err))
		return
	}
	runID := session.startWithWindowSource(string(data), sourceName, sourcePath, true, nil, nil, session.window, "")
	page.SetHTML(bundledAsyncProgramPage(runID))
	if err := tw.Navigate("/"); err != nil {
		tw.Close()
		showMessageWindow(title, fmt.Sprintf("ウィンドウを開けません: %v", err))
		return
	}
	tw.Window.Run()

	// ウィンドウが閉じられたら、ダイアログ待ちの実行を打ち切って
	// execMu を解放させる。
	session.cancelPendingDialogs()

	// ウィンドウを閉じたら、保持していた実行を退役させる（#363）。
	session.close()
}
