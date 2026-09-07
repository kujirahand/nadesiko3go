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

type AsyncRunStatus struct {
	Done   bool           `json:"done"`
	Dialog *DialogRequest `json:"dialog,omitempty"`
	Result *RunResult     `json:"result,omitempty"`
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
		state.finish(s.runCompiledNow(prog, registry, host, screen, result))
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
		state.finish(s.runCompiledNow(prog, registry, host, screen, result))
	}()
	return id
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

func (s *guiSession) dispatch(runID uint64, handle int, event string, values map[string]string) RunResult {
	s.execMu.Lock()
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
