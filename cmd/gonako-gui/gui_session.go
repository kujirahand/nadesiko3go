package main

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

type guiHost struct {
	mu     sync.Mutex
	out    strings.Builder
	in     *bufio.Reader
	args   []string
	bundle *bundle.Bundle
	screen *guilib.Screen
	render bool
	dialog func(kind, message string) (string, bool, error)

	exitCode int
	exited   bool
}

func newGUIHost(screen *guilib.Screen, render bool, args []string, packed *bundle.Bundle) *guiHost {
	return &guiHost{
		in: bufio.NewReader(strings.NewReader("")), args: args,
		bundle: packed, screen: screen, render: render,
	}
}

func (h *guiHost) Print(text string) {
	h.mu.Lock()
	h.out.WriteString(text)
	h.out.WriteByte('\n')
	h.mu.Unlock()
	if h.render {
		h.screen.DisplayText(text)
	}
}

func (h *guiHost) Write(text string) {
	h.mu.Lock()
	h.out.WriteString(text)
	h.mu.Unlock()
	if h.render {
		h.screen.DisplayText(text)
	}
}

func (h *guiHost) ReadLine() (string, error) {
	line, err := h.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (h *guiHost) ShowDialog(kind, message string) (string, bool, bool, error) {
	if h.dialog == nil {
		return "", false, false, nil
	}
	answer, accepted, err := h.dialog(kind, message)
	return answer, accepted, true, err
}

func (h *guiHost) Exit(code int)  { h.exitCode, h.exited = code, true }
func (h *guiHost) Args() []string { return append([]string(nil), h.args...) }
func (h *guiHost) ReadResource(name string) ([]byte, bool) {
	if h.bundle == nil {
		return nil, false
	}
	return h.bundle.ReadResource(name)
}
func (h *guiHost) Now() time.Time { return time.Now() }

func (h *guiHost) drainOutput() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	text := h.out.String()
	h.out.Reset()
	return text
}

// Compile-time assertion for accidental Host API drift.
var _ vm.Host = (*guiHost)(nil)

type guiExecution struct {
	id      uint64
	screen  *guilib.Screen
	host    *guiHost
	machine *vm.VM
	workDir string
}

type guiSession struct {
	mu     sync.Mutex
	execMu sync.Mutex
	nextID uint64
	active *guiExecution
	async  map[uint64]*guiAsyncRun
}

type DialogRequest struct {
	ID      uint64 `json:"id"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// AsyncRunStatus is one poll of a running program.
//
// Output と Operations は「前回のポーリング以降に出た分」で、読み取った時点で
// 消える。非同期実行では出力も画面操作もすべてこの2つで届き、Result 側には
// 入らない（→ runCompiledStreaming）。したがってポーリングする画面は、
// done を見る前に必ず Output と Operations を処理しなければならない。
// 途中経過を捨てると、その分は二度と取り出せない。
type AsyncRunStatus struct {
	Done       bool               `json:"done"`
	Dialog     *DialogRequest     `json:"dialog,omitempty"`
	Output     string             `json:"output,omitempty"`
	Operations []guilib.Operation `json:"operations,omitempty"`
	Result     *RunResult         `json:"result,omitempty"`
}

type dialogAnswer struct {
	text     string
	accepted bool
}

type guiAsyncRun struct {
	mu           sync.Mutex
	nextDialogID uint64
	dialog       *DialogRequest
	answer       chan dialogAnswer
	done         bool
	result       RunResult

	// pendingOutput/pendingOps hold 『表示』の出力とGUI操作のうち、実行中に
	// 発生したがまだポーリングで取り出されていない分。実行が終わるまで
	// まとめて返してしまうと、ループの途中経過が画面に出ないまま終了時に
	// 一気に表示される（#48）。status() を呼ぶたびに drain して返す。
	pendingOutput string
	pendingOps    []guilib.Operation
}

// appendPending queues output/operations produced while the VM is still
// running, so the next status() poll can hand them to the frontend.
func (r *guiAsyncRun) appendPending(output string, ops []guilib.Operation) {
	if output == "" && len(ops) == 0 {
		return
	}
	r.mu.Lock()
	r.pendingOutput += output
	r.pendingOps = append(r.pendingOps, ops...)
	r.mu.Unlock()
}

func (r *guiAsyncRun) showDialog(kind, message string) (string, bool, error) {
	r.mu.Lock()
	r.nextDialogID++
	request := &DialogRequest{ID: r.nextDialogID, Kind: kind, Message: message}
	answer := make(chan dialogAnswer, 1)
	r.dialog = request
	r.answer = answer
	r.mu.Unlock()

	response := <-answer
	return response.text, response.accepted, nil
}

func (r *guiAsyncRun) resolve(dialogID uint64, text string, accepted bool) bool {
	r.mu.Lock()
	if r.dialog == nil || r.dialog.ID != dialogID || r.answer == nil {
		r.mu.Unlock()
		return false
	}
	answer := r.answer
	r.dialog = nil
	r.answer = nil
	r.mu.Unlock()
	select {
	case answer <- dialogAnswer{text: text, accepted: accepted}:
		return true
	default:
		return false
	}
}

func (r *guiAsyncRun) finish(result RunResult) {
	r.mu.Lock()
	r.result = result
	r.done = true
	r.mu.Unlock()
}

func (r *guiAsyncRun) status() AsyncRunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := AsyncRunStatus{Done: r.done}
	if r.dialog != nil {
		request := *r.dialog
		status.Dialog = &request
	}
	if r.pendingOutput != "" || len(r.pendingOps) > 0 {
		status.Output = r.pendingOutput
		status.Operations = r.pendingOps
		r.pendingOutput = ""
		r.pendingOps = nil
	}
	if r.done {
		result := r.result
		status.Result = &result
	}
	return status
}

func (s *guiSession) reserveRun(async bool) (uint64, *guiAsyncRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	var state *guiAsyncRun
	if async {
		state = &guiAsyncRun{}
		if s.async == nil {
			s.async = map[uint64]*guiAsyncRun{}
		}
		s.async[s.nextID] = state
	}
	return s.nextID, state
}

func (s *guiSession) run(code, filename string, windowMode bool, args []string, packed *bundle.Bundle) RunResult {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	id, _ := s.reserveRun(false)
	screen := guilib.NewScreen()
	plugin := guilib.NewWithScreen(screen)
	registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
	host := newGUIHost(screen, windowMode, args, packed)

	result := RunResult{OK: false, RunID: id}
	prog, err := vm.CompileWithRegistry(code, filename, registry)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	return s.runCompiledNow(prog, registry, host, screen, result)
}

func (s *guiSession) runCompiled(prog *ir.Program, args []string, packed *bundle.Bundle) RunResult {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	id, _ := s.reserveRun(false)
	screen := guilib.NewScreen()
	plugin := guilib.NewWithScreen(screen)
	registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
	host := newGUIHost(screen, true, args, packed)
	result := RunResult{OK: false, RunID: id}
	return s.runCompiledNow(prog, registry, host, screen, result)
}

func (s *guiSession) runCompiledNow(prog *ir.Program, registry *stdlib.Registry, host *guiHost, screen *guilib.Screen, result RunResult) RunResult {
	opts := vm.DefaultOptions()
	opts.RealSleep = true
	machine := vm.New(prog, registry, host, opts)
	workDir, _ := os.Getwd()
	exec := &guiExecution{id: result.RunID, screen: screen, host: host, machine: machine, workDir: workDir}
	s.mu.Lock()
	s.active = exec
	s.mu.Unlock()

	err := machine.Run()
	result.OK = err == nil
	result.Output = host.drainOutput()
	result.Operations = screen.DrainOperations()
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (s *guiSession) start(code, filename string, windowMode bool, args []string, packed *bundle.Bundle) uint64 {
	id, state := s.reserveRun(true)
	workDir, _ := os.Getwd()
	go func() {
		s.execMu.Lock()
		defer s.execMu.Unlock()
		originalDir, _ := os.Getwd()
		if workDir != "" {
			_ = os.Chdir(workDir)
		}
		defer func() {
			if originalDir != "" {
				_ = os.Chdir(originalDir)
			}
		}()

		screen := guilib.NewScreen()
		plugin := guilib.NewWithScreen(screen)
		registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
		host := newGUIHost(screen, windowMode, args, packed)
		host.dialog = state.showDialog
		result := RunResult{OK: false, RunID: id}
		prog, err := vm.CompileWithRegistry(code, filename, registry)
		if err != nil {
			result.Error = err.Error()
			state.finish(result)
			return
		}
		state.finish(s.runCompiledStreaming(state, prog, registry, host, screen, result))
	}()
	return id
}

func (s *guiSession) startCompiled(prog *ir.Program, args []string, packed *bundle.Bundle) uint64 {
	id, state := s.reserveRun(true)
	go func() {
		s.execMu.Lock()
		defer s.execMu.Unlock()
		screen := guilib.NewScreen()
		plugin := guilib.NewWithScreen(screen)
		registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
		host := newGUIHost(screen, true, args, packed)
		host.dialog = state.showDialog
		result := RunResult{OK: false, RunID: id}
		state.finish(s.runCompiledStreaming(state, prog, registry, host, screen, result))
	}()
	return id
}

// streamPendingOutputInterval is how often streamPendingOutput drains output
// while the VM runs. Short enough that 『表示』 feels immediate, long enough
// not to matter for CPU usage.
const streamPendingOutputInterval = 20 * time.Millisecond

// streamPendingOutput drains host/screen periodically while the VM is still
// running and queues what it finds on state, so a poll() call in progress
// sees output as it happens instead of only once the whole program ends
// (#48). It stops once done is closed, after one last drain to catch
// anything produced between the last tick and the program's end.
func streamPendingOutput(state *guiAsyncRun, host *guiHost, screen *guilib.Screen, done <-chan struct{}) {
	ticker := time.NewTicker(streamPendingOutputInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			state.appendPending(host.drainOutput(), screen.DrainOperations())
		case <-done:
			state.appendPending(host.drainOutput(), screen.DrainOperations())
			return
		}
	}
}

// runCompiledStreaming runs runCompiledNow while a background goroutine
// streams intermediate output to state, so an async run (start/startCompiled)
// reports 『表示』 output as it happens rather than all at once at the end.
//
// 出力と画面操作は最後に必ず全部ストリーム側へ寄せ、RunResult には残さない。
// 途中で drain された分だけがストリームに乗り、残りが Result に入る、という
// 中途半端な分かれ方をすると、ストリームを読まない画面が出力を取りこぼす。
// 実際それでバンドル版が画面を描けなくなっていた。
func (s *guiSession) runCompiledStreaming(state *guiAsyncRun, prog *ir.Program, registry *stdlib.Registry, host *guiHost, screen *guilib.Screen, result RunResult) RunResult {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamPendingOutput(state, host, screen, done)
	}()
	result = s.runCompiledNow(prog, registry, host, screen, result)
	close(done)
	wg.Wait()

	// runCompiledNow が最後に drain した分をストリームの末尾へ回す。
	// 先に流れた分より必ず新しいので、順序は保たれる。呼び出し元が
	// state.finish() するのはこの後なので、done がポーリングから見えた
	// 時点で全部が pending に載っていることも保証される。
	state.appendPending(result.Output, result.Operations)
	result.Output = ""
	result.Operations = nil
	return result
}

func (s *guiSession) poll(runID uint64) AsyncRunStatus {
	s.mu.Lock()
	state := s.async[runID]
	s.mu.Unlock()
	if state == nil {
		return AsyncRunStatus{Done: true, Result: &RunResult{OK: false, RunID: runID, Error: "実行結果が見つかりません。"}}
	}
	return state.status()
}

func (s *guiSession) resolveDialog(runID, dialogID uint64, text string, accepted bool) bool {
	s.mu.Lock()
	state := s.async[runID]
	s.mu.Unlock()
	return state != nil && state.resolve(dialogID, text, accepted)
}

// dispatchLockGrace is how long an event may wait for a run to let go of
// execMu. Short enough that the window never visibly freezes.
const dispatchLockGrace = 50 * time.Millisecond

// lockExecWithin takes execMu, giving up if it cannot within timeout.
func (s *guiSession) lockExecWithin(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if s.execMu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *guiSession) dispatch(runID uint64, handle int, event string, values map[string]string) RunResult {
	// 実行中のイベントは、少しだけ待って取れなければ断る。無制限に待つと、
	// WebViewのバインド呼び出しはUIスレッド上で処理されるため、長く走る
	// プログラムの間ウィンドウごと固まってしまう。画面部品は実行中にも
	// 描かれるので、この経路は普通に踏まれる。
	// 猶予を置くのは、実行終了(finish)からexecMu解放までの僅かな隙間で
	// 押されたクリックを、実行中だと誤って断らないため。
	if !s.lockExecWithin(dispatchLockGrace) {
		return RunResult{OK: false, RunID: runID, Error: "プログラムの実行中はイベントを処理できません。"}
	}
	defer s.execMu.Unlock()
	s.mu.Lock()
	exec := s.active
	s.mu.Unlock()
	if exec == nil || exec.id != runID {
		return RunResult{OK: false, RunID: runID, Error: "この画面は古い実行結果です。もう一度実行してください。"}
	}
	originalDir, _ := os.Getwd()
	if exec.workDir != "" {
		_ = os.Chdir(exec.workDir)
	}
	defer func() {
		if originalDir != "" {
			_ = os.Chdir(originalDir)
		}
	}()
	err := exec.screen.DispatchEvent(handle, event, values)
	result := RunResult{
		OK:         err == nil,
		RunID:      runID,
		Output:     exec.host.drainOutput(),
		Operations: exec.screen.DrainOperations(),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}
