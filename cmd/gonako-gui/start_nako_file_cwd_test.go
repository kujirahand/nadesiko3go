package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startNakoFileForTest / runNakoFileForTest は、bindPrivilegedBridge が
// 登録する startNakoFile / runNakoFile バインディングの本体と同じ呼び出しを、
// WebView を介さずに行うテスト用ヘルパー。
//
// bindPrivilegedBridge は特権バインドの保護のため *webview.Protected を
// 要求するようになり（S01 の回帰テストで固定）、偽WebViewでは呼べなく
// なった。バインディングの本体は nakoRunLocation で作業ディレクトリを
// 決めて guiSession へ渡すだけなので、その呼び出しをここで再現し、
// 振る舞いの検証は guiSession 側で行う。
func startNakoFileForTest(session *guiSession, initialWorkingDir, code, filePath string, windowMode bool) uint64 {
	runFile, workDir := nakoRunLocation(filePath, initialWorkingDir)
	return session.startWithWindow(code, runFile, windowMode, nil, nil, newVirtualWindowController(), workDir)
}

func runNakoFileForTest(session *guiSession, initialWorkingDir, code, filePath string, windowMode bool) RunResult {
	runFile, workDir := nakoRunLocation(filePath, initialWorkingDir)
	return session.runWithWindowInDir(code, runFile, windowMode, nil, nil, newVirtualWindowController(), workDir)
}

// samePath はシンボリックリンク（macOSの/private/tmpなど）を解決して比べる。
func samePath(a, b string) bool {
	resolve := func(p string) string {
		if p == "" {
			return ""
		}
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return resolved
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return dir
}

// chdirForTest はテスト中のCWDを変更し、テスト終了時に必ず元へ戻す。
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	original := mustGetwd(t)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}

// waitAsyncRun は非同期実行が終わるまでpollし、ストリームされた出力と
// 最終結果を返す。出力はpollのたびに取り出さないと失われる。
func waitAsyncRun(t *testing.T, session *guiSession, runID uint64) (string, RunResult) {
	t.Helper()
	var out strings.Builder
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status := session.poll(runID)
		out.WriteString(status.Output)
		if !status.Done {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if status.Result == nil {
			t.Fatalf("run %d は終了しましたがResultがありません", runID)
		}
		return out.String(), *status.Result
	}
	t.Fatalf("run %d が時間内に終了しませんでした", runID)
	return "", RunResult{}
}

// waitForCWD は実行goroutineのdeferによるCWD復元を待つ。
func waitForCWD(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := os.Getwd()
		if err == nil && samePath(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("実行後のCWD = %q, want %q", got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// execWorkDir はrunCompiledNowが記録した作業ディレクトリを返す。
func execWorkDir(session *guiSession) string {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.active == nil {
		return ""
	}
	return session.active.workDir
}

func TestNakoRunLocation(t *testing.T) {
	base := t.TempDir()
	scriptDir := filepath.Join(base, "script")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	savedFile := filepath.Join(scriptDir, "main.nako3")
	missingFile := filepath.Join(base, "missing", "main.nako3")

	tests := []struct {
		name              string
		filePath          string
		initialWorkingDir string
		wantRunFile       string
		wantWorkDir       string
	}{
		{
			name:              "保存済みファイルは親ディレクトリで実行する",
			filePath:          savedFile,
			initialWorkingDir: base,
			wantRunFile:       savedFile,
			wantWorkDir:       scriptDir,
		},
		{
			name:              "未保存は起動時のディレクトリで実行する",
			filePath:          "",
			initialWorkingDir: base,
			wantRunFile:       "gui.nako3",
			wantWorkDir:       base,
		},
		{
			name:              "親ディレクトリが無ければCWDへフォールバックする",
			filePath:          missingFile,
			initialWorkingDir: base,
			wantRunFile:       missingFile,
			wantWorkDir:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runFile, workDir := nakoRunLocation(tt.filePath, tt.initialWorkingDir)
			if runFile != tt.wantRunFile {
				t.Fatalf("runFile = %q, want %q", runFile, tt.wantRunFile)
			}
			if workDir != tt.wantWorkDir {
				t.Fatalf("workDir = %q, want %q", workDir, tt.wantWorkDir)
			}
		})
	}
}

// startNakoFileは実行前のCWDではなく、スクリプトのあるディレクトリでVMを
// 動かし、終了後にはプロセスCWDを呼び出し前へ戻さなければならない。
func TestStartNakoFileRunsInScriptDirAndRestoresCWD(t *testing.T) {
	original := mustGetwd(t)
	scriptDir := t.TempDir()
	runDir := t.TempDir()
	chdirForTest(t, runDir)

	session := &guiSession{}
	start := func(code, filePath string, windowMode bool) uint64 {
		return startNakoFileForTest(session, original, code, filePath, windowMode)
	}

	// 実行の途中でUIスレッド側のCWD復元に邪魔されないこと（実行中ずっと
	// scriptDirであること）を確かめるため、待機を挟んで2回取得する。
	runID := start("「{カレントディレクトリ取得}」を表示\n0.1秒待つ\n「{カレントディレクトリ取得}」を表示", filepath.Join(scriptDir, "main.nako3"), false)
	out, result := waitAsyncRun(t, session, runID)
	if !result.OK {
		t.Fatalf("実行に失敗しました: %#v", result)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("出力行数 = %d, want 2 (output=%q)", len(lines), out)
	}
	for i, line := range lines {
		if got := strings.TrimSpace(line); !samePath(got, scriptDir) {
			t.Fatalf("%d回目のVMのCWD = %q, want %q", i+1, got, scriptDir)
		}
	}
	if got := execWorkDir(session); !samePath(got, scriptDir) {
		t.Fatalf("exec.workDir = %q, want %q", got, scriptDir)
	}
	waitForCWD(t, runDir)
}

// 未保存（filePath == ""）のときは、GUI起動時のディレクトリが
// 作業ディレクトリとして使われること。
func TestStartNakoFileUsesInitialWorkingDirForUnsavedFile(t *testing.T) {
	initialDir := t.TempDir()
	runDir := t.TempDir()
	chdirForTest(t, runDir)

	session := &guiSession{}
	start := func(code, filePath string, windowMode bool) uint64 {
		return startNakoFileForTest(session, initialDir, code, filePath, windowMode)
	}

	runID := start(`「{カレントディレクトリ取得}」を表示`, "", false)
	out, result := waitAsyncRun(t, session, runID)
	if !result.OK {
		t.Fatalf("実行に失敗しました: %#v", result)
	}
	if got := strings.TrimSpace(out); !samePath(got, initialDir) {
		t.Fatalf("VMのCWD = %q, want %q", got, initialDir)
	}
	waitForCWD(t, runDir)
}

// 連続して非同期実行した場合でも、それぞれのスクリプトディレクトリで
// 実行され、完了後は呼び出し前のCWDへ戻ること（先行実行の復元が
// 後続実行の作業ディレクトリを壊さないこと）。
func TestStartNakoFileSerializesConsecutiveRuns(t *testing.T) {
	original := mustGetwd(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	runDir := t.TempDir()
	chdirForTest(t, runDir)

	session := &guiSession{}
	start := func(code, filePath string, windowMode bool) uint64 {
		return startNakoFileForTest(session, original, code, filePath, windowMode)
	}

	code := "「{カレントディレクトリ取得}」を表示\n0.05秒待つ"
	runA := start(code, filepath.Join(dirA, "a.nako3"), false)
	runB := start(code, filepath.Join(dirB, "b.nako3"), false)
	if runA == runB {
		t.Fatalf("連続実行のrunIDが同じです: %d", runA)
	}

	outA, resultA := waitAsyncRun(t, session, runA)
	if !resultA.OK {
		t.Fatalf("先行実行に失敗しました: %#v", resultA)
	}
	outB, resultB := waitAsyncRun(t, session, runB)
	if !resultB.OK {
		t.Fatalf("後続実行に失敗しました: %#v", resultB)
	}
	if got := strings.TrimSpace(outA); !samePath(got, dirA) {
		t.Fatalf("先行実行のCWD = %q, want %q", got, dirA)
	}
	if got := strings.TrimSpace(outB); !samePath(got, dirB) {
		t.Fatalf("後続実行のCWD = %q, want %q", got, dirB)
	}
	waitForCWD(t, runDir)
}

// 非同期実行A（startNakoFile）がexecMuを保持してディレクトリAで動いている
// 最中に、同期実行B（runNakoFile）を呼んだ場合の回帰テスト。
//
// Bがロックを取る前にos.ChdirでCWDを書き換えると、Aの作業ディレクトリが
// 壊れ、A終了時の復元の後でBが誤ったディレクトリで動き、最後にBが
// ディレクトリAへ「復元」してCWDが残留する。CWDの移動と復元は
// どちらもexecMuの内側で行わなければならない。
func TestRunNakoFileDuringAsyncRunReturnsBusyAndKeepsAsyncWorkDir(t *testing.T) {
	original := mustGetwd(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	runDir := t.TempDir()
	chdirForTest(t, runDir)

	session := &guiSession{}
	start := func(code, filePath string, windowMode bool) uint64 {
		return startNakoFileForTest(session, original, code, filePath, windowMode)
	}
	run := func(code, filePath string, windowMode bool) RunResult {
		return runNakoFileForTest(session, original, code, filePath, windowMode)
	}

	// Aは待機を挟んで2回CWDを表示する。同期実行に割り込まれても、
	// 両方とも自分の作業ディレクトリで動くことを示す。
	codeA := "「{カレントディレクトリ取得}」を表示\n0.2秒待つ\n「{カレントディレクトリ取得}」を表示"
	runA := start(codeA, filepath.Join(dirA, "a.nako3"), false)

	// AがexecMuを取ってdirAへ移動するまで待つ。これで同期実行の呼び出しは
	// 必ずAの実行中（ロック保持中）になる。
	waitForCWD(t, dirA)

	// 同期実行は、実行中の非同期実行を待たずに errExecBusy で返らなければ
	// ならない（UIスレッド上で無制限に待つとメインキューと待ち合って
	// 固まるため、待ってでも実行する仕様にはしない）。
	resultB := run("「{カレントディレクトリ取得}」を表示", filepath.Join(dirB, "b.nako3"), false)
	if resultB.OK {
		t.Fatalf("同期実行が成功してしまいました（busyで返るはず）: %#v", resultB)
	}
	if resultB.Error != errExecBusy {
		t.Fatalf("同期実行のエラー = %q, want errExecBusy (%q)", resultB.Error, errExecBusy)
	}

	// Aは割り込まれず、両回ともdirAで動く。
	outA, resultA := waitAsyncRun(t, session, runA)
	if !resultA.OK {
		t.Fatalf("非同期実行に失敗しました: %#v", resultA)
	}
	lines := strings.Split(strings.TrimSpace(outA), "\n")
	if len(lines) != 2 {
		t.Fatalf("非同期実行の出力行数 = %d, want 2 (output=%q)", len(lines), outA)
	}
	for i, line := range lines {
		if got := strings.TrimSpace(line); !samePath(got, dirA) {
			t.Fatalf("非同期実行の%d回目のCWD = %q, want %q", i+1, got, dirA)
		}
	}

	// Aの完了後、プロセスCWDは元のディレクトリへ戻っていること。
	waitForCWD(t, runDir)

	// 実行が終われば、同じ同期実行は今度は成功し、dirBで動く
	// （busyは一時的な拒否であり、壊れた状態ではない）。
	resultB2 := run("「{カレントディレクトリ取得}」を表示", filepath.Join(dirB, "b.nako3"), false)
	if !resultB2.OK {
		t.Fatalf("非同期実行の完了後の同期実行に失敗しました: %#v", resultB2)
	}
	if got := strings.TrimSpace(resultB2.Output); !samePath(got, dirB) {
		t.Fatalf("同期実行のCWD = %q, want %q", got, dirB)
	}
	if got := execWorkDir(session); !samePath(got, dirB) {
		t.Fatalf("同期実行のexec.workDir = %q, want %q", got, dirB)
	}
	waitForCWD(t, runDir)
}
