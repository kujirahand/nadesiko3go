package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/webview/webview_go"
)

// fakeWebView はbindPrivilegedBridgeが登録するBind関数を捕まえるだけの
// 偽WebView。WebViewを開かずにstartNakoFileなどのバインディングを
// 直接呼び出せるようにする。
type fakeWebView struct {
	mu       sync.Mutex
	bindings map[string]any
}

func (f *fakeWebView) Run()                   {}
func (f *fakeWebView) Terminate()             {}
func (f *fakeWebView) Dispatch(func())        {}
func (f *fakeWebView) Destroy()               {}
func (f *fakeWebView) Window() unsafe.Pointer { return nil }
func (f *fakeWebView) NativeHandle(webview.NativeHandleKind) unsafe.Pointer {
	return nil
}
func (f *fakeWebView) SetTitle(string)                {}
func (f *fakeWebView) SetSize(int, int, webview.Hint) {}
func (f *fakeWebView) Navigate(string)                {}
func (f *fakeWebView) SetHtml(string)                 {}
func (f *fakeWebView) Init(string)                    {}
func (f *fakeWebView) Eval(string)                    {}

func (f *fakeWebView) Bind(name string, fn any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bindings == nil {
		f.bindings = map[string]any{}
	}
	f.bindings[name] = fn
	return nil
}

func (f *fakeWebView) Unbind(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bindings, name)
	return nil
}

func (f *fakeWebView) binding(t *testing.T, name string) any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	fn, ok := f.bindings[name]
	if !ok {
		t.Fatalf("バインディング %q が登録されていません", name)
	}
	return fn
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
	w := &fakeWebView{}
	bindPrivilegedBridge(w, session, nil, original, "", false)
	start, ok := w.binding(t, "startNakoFile").(func(string, string, bool) uint64)
	if !ok {
		t.Fatal("startNakoFile のシグネチャが func(string, string, bool) uint64 ではありません")
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
	w := &fakeWebView{}
	bindPrivilegedBridge(w, session, nil, initialDir, "", false)
	start, ok := w.binding(t, "startNakoFile").(func(string, string, bool) uint64)
	if !ok {
		t.Fatal("startNakoFile のシグネチャが func(string, string, bool) uint64 ではありません")
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
	w := &fakeWebView{}
	bindPrivilegedBridge(w, session, nil, original, "", false)
	start, ok := w.binding(t, "startNakoFile").(func(string, string, bool) uint64)
	if !ok {
		t.Fatal("startNakoFile のシグネチャが func(string, string, bool) uint64 ではありません")
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
func TestRunNakoFileDuringAsyncRunKeepsEachWorkDir(t *testing.T) {
	original := mustGetwd(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	runDir := t.TempDir()
	chdirForTest(t, runDir)

	session := &guiSession{}
	w := &fakeWebView{}
	bindPrivilegedBridge(w, session, nil, original, "", false)
	start, ok := w.binding(t, "startNakoFile").(func(string, string, bool) uint64)
	if !ok {
		t.Fatal("startNakoFile のシグネチャが func(string, string, bool) uint64 ではありません")
	}
	run, ok := w.binding(t, "runNakoFile").(func(string, string, bool) string)
	if !ok {
		t.Fatal("runNakoFile のシグネチャが func(string, string, bool) string ではありません")
	}

	// Aは待機を挟んで2回CWDを表示し、Bに割り込まれていないことを示す。
	codeA := "「{カレントディレクトリ取得}」を表示\n0.2秒待つ\n「{カレントディレクトリ取得}」を表示"
	runA := start(codeA, filepath.Join(dirA, "a.nako3"), false)

	// AがexecMuを取ってdirAへ移動するまで待つ。これでBの呼び出しは
	// 必ずAの実行中（ロック保持中）になる。
	waitForCWD(t, dirA)

	rawB := run("「{カレントディレクトリ取得}」を表示", filepath.Join(dirB, "b.nako3"), false)
	var resultB RunResult
	if err := json.Unmarshal([]byte(rawB), &resultB); err != nil {
		t.Fatalf("runNakoFile の結果をJSONとして解釈できません: %v (raw=%q)", err, rawB)
	}
	if !resultB.OK {
		t.Fatalf("同期実行に失敗しました: %#v", resultB)
	}
	if got := strings.TrimSpace(resultB.Output); !samePath(got, dirB) {
		t.Fatalf("同期実行のCWD = %q, want %q", got, dirB)
	}
	if got := execWorkDir(session); !samePath(got, dirB) {
		t.Fatalf("同期実行のexec.workDir = %q, want %q", got, dirB)
	}

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

	// 両方の完了後、プロセスCWDは元のディレクトリへ戻っていること。
	waitForCWD(t, runDir)
}
