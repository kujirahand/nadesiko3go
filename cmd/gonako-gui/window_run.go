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
	workDir := ""
	if filePath != "" {
		title = filepath.Base(filePath)
		if dir := filepath.Dir(filePath); dir != "." {
			if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
				workDir = dir
			}
		}
	}

	cmd := exec.Command(self, "--run-window", tmpPath, "--run-window-title", title)
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

// runStandaloneWindow is the child-process entry point launched by
// launchNakoWindowProcess. It reads the program from sourcePath, opens a
// fresh native window for it, and runs it there — that window, not the
// editor's, is what 『母艦のウィンドウ変更』 controls.
func runStandaloneWindow(sourcePath, title string) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		showMessageWindow(title, fmt.Sprintf("プログラムを読み込めません: %v", err))
		return
	}
	os.Remove(sourcePath)

	if title == "" {
		title = "なでしこ3"
	}
	w := newAppWindow(defaultWindowSettings(title, 960, 640))
	if w == nil {
		return
	}
	defer w.Destroy()

	session := &guiSession{window: newNativeWindowController(w)}
	_ = w.Bind("startNakoEvent", func(runID uint64, handle int, event string, values map[string]string) string {
		b, _ := json.Marshal(session.startEvent(runID, handle, event, values))
		return string(b)
	})
	_ = w.Bind("pollNakoRun", func(runID uint64) string {
		b, _ := json.Marshal(session.poll(runID))
		return string(b)
	})
	_ = w.Bind("resolveNakoDialog", func(runID, dialogID uint64, text string, accepted bool) bool {
		return session.resolveDialog(runID, dialogID, text, accepted)
	})
	_ = w.Bind("closeBundledWindow", func() {
		w.Terminate()
	})

	runID := session.start(string(data), "main.nako3", true, nil, nil)
	w.SetHtml(bundledAsyncProgramPage(runID))
	w.Run()
}
