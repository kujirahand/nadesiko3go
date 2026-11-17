package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kujirahand/nadesiko3go/cmd/gonako-gui/internal/runcwd"
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

// RequestGUI は既存の要求・応答経路で画面処理の完了を待つ。ユーザー用ダイアログは開かない。
func (h *guiHost) RequestGUI(kind, message string) (string, error) {
	if !h.render || h.dialog == nil {
		return "", errors.New("この命令はgonako-guiのウィンドウモードで実行してください。")
	}
	answer, accepted, err := h.dialog(kind, message)
	if err != nil {
		return "", err
	}
	if !accepted {
		return "", errors.New(answer)
	}
	return answer, nil
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
	window guilib.WindowController

	// dialogEpoch は「新しい実行の開始」「ウィンドウ終了・リロード」で
	// 待機中のダイアログを打ち切るたびに進む世代番号。
	//
	// 実行は予約時の世代を覚えておき、ダイアログを出す直前に現在の世代と
	// 比べる。打ち切りの走査が終わった直後に登録されたダイアログでも、
	// 世代が変わっていれば応答は来ないと分かるので、待ちに入らずに
	// エラーで終了して execMu を解放できる（→ registerDialog）。
	dialogEpoch uint64
}

// errDialogCanceled は、待機中のダイアログが外から打ち切られたことを表す。
//
// 利用者がダイアログの「キャンセル」を押した場合は accepted=false, err=nil
// なので、それと混同してはいけない。打ち切りは実行そのものを止めるべき
// 出来事（ウィンドウ終了・リロード・新しい実行の開始）なので、必ず
// 非nilのエラーを返してVMを停止させる。
var errDialogCanceled = errors.New("ダイアログの待機が中断されました")

// guiDialogTimeout はshowDialogが応答を待つ上限。bridge.go の
// bridgeDialogTimeout と同じ考え方で、画面が応答を返せなくなった場合に
// execMu を握ったまま永久に固まらないようにする。
// 放置されたダイアログは、この時間でエラー終了する。利用者のキャンセル
// （accepted=false, err=nil）とは区別し、必ず非nilのエラーを返す。
// 猶予は応答待ちの利用者を急かさない程度に長く取り、テストが短縮できる
// よう変数にしてある。
var guiDialogTimeout = 30 * time.Minute

func (s *guiSession) pluginForScreen(screen *guilib.Screen) *guilib.Plugin {
	return guilib.NewWithScreenAndWindow(screen, s.window)
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

// dialogAnswer は1つのダイアログの決着。
//
// err は「待機が外から打ち切られた」ことを表す。利用者がダイアログの
// 「キャンセル」を押した場合は accepted=false, err=nil なので区別できる。
// err を使うのはwnako3の命令ブリッジ（bridge.go）だけで、guiSession 側は
// cancel チャネルで打ち切りを運ぶ（→ guiAsyncRun.showDialog）。
type dialogAnswer struct {
	text     string
	accepted bool
	err      error
}

type guiAsyncRun struct {
	mu           sync.Mutex
	nextDialogID uint64
	dialog       *DialogRequest
	answer       chan dialogAnswer
	dialogCancel chan struct{}
	done         bool
	result       RunResult

	// session と dialogEpoch は、この実行を予約したセッションと、その時点の
	// 打ち切り世代。ダイアログを登録する前に世代を確かめ、既に新しい実行が
	// 始まっている（＝応答が来ない）なら待たずに打ち切る。
	session     *guiSession
	dialogEpoch uint64

	// isEvent はこの実行がクリックなどのイベント処理であることを示す。
	// イベントはクリックのたびに起きるので、画面が結果を読み終えたら
	// セッションから捨てる（→ poll）。
	isEvent bool

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

// collectPending は画面操作の取り出しと追加を同じロックで行い、操作順を保つ。
func (r *guiAsyncRun) collectPending(host *guiHost, screen *guilib.Screen) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingOutput += host.drainOutput()
	r.pendingOps = append(r.pendingOps, screen.DrainOperations()...)
}

func (r *guiAsyncRun) dialogHandler(host *guiHost) func(string, string) (string, bool, error) {
	return func(kind, message string) (string, bool, error) {
		// キャンバス作成・描画を先に届けてから、読み込みや保存を要求する。
		r.collectPending(host, host.screen)
		return r.showDialog(kind, message)
	}
}

// registerDialog は待機中のダイアログを実行状態へ登録する。
//
// 登録はセッションの dialogEpoch と排他して行う。新しい実行の開始や
// ウィンドウ終了（→ cancelPendingDialogs）が先に起きていれば、この
// ダイアログにはもう誰も応答できないので、待ちに入る前にエラーで打ち切る。
// ここを排他しないと「打ち切りの走査が終わった直後に登録されたダイアログ」が
// 取り残され、その実行が execMu を握ったままデッドロックする。
//
// ロック順は s.mu → r.mu。cancelPendingDialogs も同じ順で取る。
//
// session が nil なのは、テストが guiAsyncRun を単体で作った場合だけ。
// 本番の実行は reserveRun が必ず session と予約時の dialogEpoch を
// セットする。nil のときは世代を見ずに登録する（テストが打ち切りと応答の
// 競合をセッション無しで見るため）。本番経路はこの分岐に頼らない。
func (r *guiAsyncRun) registerDialog(request *DialogRequest, answer chan dialogAnswer, cancel chan struct{}) error {
	s := r.session
	if s == nil {
		r.mu.Lock()
		r.dialog, r.answer, r.dialogCancel = request, answer, cancel
		r.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialogEpoch != r.dialogEpoch {
		return errDialogCanceled
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dialog, r.answer, r.dialogCancel = request, answer, cancel
	return nil
}

// showDialog は画面にダイアログを出し、応答を待つ。
//
// 応答は resolve が、打ち切りは cancelDialog が運ぶ。どちらも r.mu を
// 握って登録の有無を確かめてから決着させるので、両者が同時に来ても
// 「応答が届く」「打ち切られる」のどちらか一方に必ず定まり、チャネル送信で
// ブロックしたり閉じたチャネルへ送ったりしない。
//
// 打ち切り・タイムアウトは accepted=false, err=nil ではなく、必ず非nilの
// エラーを返す。利用者のキャンセル操作と区別するため。
func (r *guiAsyncRun) showDialog(kind, message string) (string, bool, error) {
	r.mu.Lock()
	r.nextDialogID++
	request := &DialogRequest{ID: r.nextDialogID, Kind: kind, Message: message}
	r.mu.Unlock()

	answer := make(chan dialogAnswer, 1)
	cancel := make(chan struct{})
	if err := r.registerDialog(request, answer, cancel); err != nil {
		return "", false, err
	}

	timer := time.NewTimer(guiDialogTimeout)
	defer timer.Stop()
	select {
	case response := <-answer:
		return response.text, response.accepted, nil
	case <-cancel:
		return "", false, errDialogCanceled
	case <-timer.C:
		if r.abandonDialog(request.ID, answer) {
			return "", false, fmt.Errorf("ダイアログの応答がありません（%v以内に応答がなく、ページの移動などで打ち切られた可能性があります）", guiDialogTimeout)
		}
		// タイムアウトと同時に応答・打ち切りが決まっていた。決まった側を優先する。
		select {
		case response := <-answer:
			return response.text, response.accepted, nil
		case <-cancel:
			return "", false, errDialogCanceled
		}
	}
}

// resolve は画面からの応答を待機中のダイアログへ渡す。
// 該当する待機がなければ（打ち切り後・二重応答など）false を返す。
func (r *guiAsyncRun) resolve(dialogID uint64, text string, accepted bool) bool {
	r.mu.Lock()
	if r.dialog == nil || r.dialog.ID != dialogID || r.answer == nil {
		r.mu.Unlock()
		return false
	}
	answer := r.answer
	r.dialog = nil
	r.answer = nil
	r.dialogCancel = nil
	r.mu.Unlock()
	// チャネルは容量1で、登録を外した者だけが送るので、送信がブロックすることはない。
	select {
	case answer <- dialogAnswer{text: text, accepted: accepted}:
		return true
	default:
		return false
	}
}

// cancelDialog は待機中のダイアログを打ち切る。待機がなければ false。
//
// 登録の解除と cancel チャネルの close を r.mu を握ったまま行うので、
// resolve と競合しても二重に close したり、close 済みチャネルへ送ったりしない。
func (r *guiAsyncRun) cancelDialog() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dialog == nil || r.dialogCancel == nil {
		return false
	}
	close(r.dialogCancel)
	r.dialog = nil
	r.answer = nil
	r.dialogCancel = nil
	return true
}

// abandonDialog はタイムアウトした showDialog が、まだ自分が待っていることを
// 確かめて登録を外す。外せたら true（＝タイムアウトで打ち切る）。既に
// resolve か cancelDialog が決着させていれば false を返し、呼び出し側が
// その結果を受け取る。
func (r *guiAsyncRun) abandonDialog(dialogID uint64, answer chan dialogAnswer) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dialog == nil || r.dialog.ID != dialogID || r.answer != answer {
		return false
	}
	r.dialog = nil
	r.answer = nil
	r.dialogCancel = nil
	return true
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
		state = &guiAsyncRun{session: s, dialogEpoch: s.dialogEpoch}
		if s.async == nil {
			s.async = map[uint64]*guiAsyncRun{}
		}
		s.async[s.nextID] = state
	}
	return s.nextID, state
}

// cancelPendingDialogs は、応答を待っているダイアログをすべて打ち切る。
//
// ウィンドウが閉じられた・ページがリロードされた（→ beforeunload /
// pagehide）、あるいは新しい実行が始まったときに呼ぶ。打ち切られた実行は
// エラーで終了して execMu を解放するので、後続の実行がデッドロックしない。
//
// 新しい実行の開始（startWithWindow / startCompiled / runWithWindowInDir /
// runCompiled）は、予約より前にこの関数を呼ぶ。効果は2段ある。
// 1つ目は、いま応答待ちのダイアログを即時に打ち切る。
// 2つ目は、dialogEpoch を進めるので、実行中の run がこの後に出そうとする
// ダイアログも registerDialog が世代不一致で打ち切る（置き換え）。
// 古い実行はエラー終了し、新しい実行が後を継ぐ。
//
// dialogEpoch を進めるのは、この走査の直後に登録されたダイアログも確実に
// 打ち切るため。登録側（registerDialog）は同じ s.mu を取って世代を確かめる
// ので、打ち切りと登録が競合しても取りこぼさない。待機が無いときの呼び出しも
// 世代は進む（戻り値は 0）。2回目が既に打ち切ったダイアログを二重に
// エラーにはしない。beforeunload と pagehide の二重発火は、JS側の
// unloadDialogsCanceled で離脱1回につき1回にまとめる。
//
// 戻り値は打ち切ったダイアログの数。テストとログのために返している。
func (s *guiSession) cancelPendingDialogs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogEpoch++
	canceled := 0
	for _, state := range s.async {
		if state.cancelDialog() {
			canceled++
		}
	}
	return canceled
}

func (s *guiSession) run(code, filename string, windowMode bool, args []string, packed *bundle.Bundle) RunResult {
	return s.runWithWindow(code, filename, windowMode, args, packed, s.window)
}

func (s *guiSession) runWithWindow(code, filename string, windowMode bool, args []string, packed *bundle.Bundle, windows guilib.WindowController) RunResult {
	// workDirを空にして、実行開始時点のCWDで動かす（従来の挙動）。
	return s.runWithWindowInDir(code, filename, windowMode, args, packed, windows, "")
}

// runWithWindowInDir はプログラムを同期的に実行する。
//
// workDirが空なら、execMuを取った時点のCWDで動かす。workDirを渡した場合は
// そのディレクトリへ移動してから実行し、終了後に必ず元のCWDへ戻す。
// startWithWindowと同じく、CWDの移動と復元はexecMuで直列化した内側で行う。
// ロックの外で移動すると、実行中の非同期実行の作業ディレクトリを壊し、
// 最後には呼び出し元のディレクトリへ「復元」してCWDが残留する。
//
// execMuを待つのは短い猶予（syncRunLockGrace）までで、取れなければ
// errExecBusyを返す。呼び出し元はUIスレッドであり、実行中の処理を
// 待ってでも実行する仕様にすると、メインキューとの相互待ちで固まる
// （runCompiledと揃えた仕様）。
func (s *guiSession) runWithWindowInDir(code, filename string, windowMode bool, args []string, packed *bundle.Bundle, windows guilib.WindowController, workDir string) RunResult {
	// 前の実行がダイアログ待ちのままなら先に打ち切る。execMu を取る前に
	// 打ち切らないと、その実行がロックを握ったまま解放できず、ここで
	// 永久に待たされる（→ ⑫ ダイアログ待機中のデッドロック）。
	s.cancelPendingDialogs()
	// WebViewのバインド呼び出し（runNakoFile / runNakoCode）はUIスレッド上で
	// 処理される。ここで execMu を無制限に待つと、バックグラウンド実行が
	// ネイティブのファイルダイアログを main queue へディスパッチしている間、
	// UIスレッドが main queue を回せないまま互いに待ち合って固まる。
	// 少しだけ待って取れなければエラーを返し、UIスレッドを解放する。
	if !s.lockExecWithin(syncRunLockGrace) {
		id, _ := s.reserveRun(false)
		return RunResult{OK: false, RunID: id, Error: errExecBusy}
	}
	defer s.execMu.Unlock()
	// 復元はexecMuを解放する前に行う（deferはLIFOなので、この順で
	// ロック解放より先にCWDが戻る）。
	id, _ := s.reserveRun(false)
	restoreDir, err := runcwd.EnterWorkDir(workDir)
	if err != nil {
		return RunResult{OK: false, RunID: id, Error: err.Error()}
	}
	defer restoreDir()
	screen := guilib.NewScreen()
	plugin := guilib.NewWithScreenAndWindow(screen, windows)
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
	// 同期実行も、ダイアログ待ちで execMu を握ったままの実行があれば
	// 先に打ち切る。さもないとここで永久に待たされる。
	s.cancelPendingDialogs()
	// runWithWindow と同じ理由で、待ちを区切る。
	if !s.lockExecWithin(syncRunLockGrace) {
		id, _ := s.reserveRun(false)
		return RunResult{OK: false, RunID: id, Error: errExecBusy}
	}
	defer s.execMu.Unlock()
	id, _ := s.reserveRun(false)
	screen := guilib.NewScreen()
	plugin := s.pluginForScreen(screen)
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
	previous := s.active
	s.active = exec
	s.mu.Unlock()

	// 再実行では、以前の実行をここで退役させ、その実行が開いたままの
	// 接続（SQLiteなど）を解放する（#363）。差し替え後の画面イベントは
	// 古い実行IDを既に拒否するので、以前の実行のコールバックが使われる
	// ことはない。実行とイベント処理は execMu で直列化されており、
	// 有効なコールバックの実行中に退役が割り込むこともない。
	if previous != nil && previous.machine != nil {
		_ = previous.machine.Close()
	}

	err := machine.Run()
	result.OK = err == nil
	result.Output = host.drainOutput()
	result.Operations = screen.DrainOperations()
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// close はセッションの終了（ウィンドウを閉じるなど）時に、保持している
// 実行を退役させ、その接続を解放する（#363）。
func (s *guiSession) close() {
	// 実行中・イベント処理中の退役にならないよう、実行と同じロックを取る。
	// ただし終了経路で長く待つわけにはいかない（終わらないプログラムを
	// 抱えたままウィンドウを閉じた場合に、アプリ終了まで固まってしまう）。
	// ロックを取れなければ退役を諦める。その場合はプロセスの終了時に
	// OSが接続ごと回収する。
	if !s.lockExecWithin(dispatchLockGrace) {
		return
	}
	defer s.execMu.Unlock()
	s.mu.Lock()
	active := s.active
	s.active = nil
	s.mu.Unlock()
	if active != nil && active.machine != nil {
		_ = active.machine.Close()
	}
}

func (s *guiSession) start(code, filename string, windowMode bool, args []string, packed *bundle.Bundle) uint64 {
	// workDirを空にして、実行開始時点のCWDで動かす（従来の挙動）。
	return s.startWithWindow(code, filename, windowMode, args, packed, s.window, "")
}

// startWithWindow はプログラムを非同期で実行する。
//
// workDirが空なら、execMuを取った時点のCWDで動かす。workDirを渡した場合は
// そのディレクトリへ移動してから実行し、終了後に必ず元のCWDへ戻す。
// CWDはプロセス全体で1つしかないため、os.Getwd/os.ChdirはexecMuで直列化
// した内側で行う。そうしないと、後から始まった実行が先の実行の作業
// ディレクトリを拾ったり、復元が追い越したりして、実行後もCWDが
// 書き換わったまま残る（runCompiledNowが記録するexec.workDirもずれる）。
func (s *guiSession) startWithWindow(code, filename string, windowMode bool, args []string, packed *bundle.Bundle, windows guilib.WindowController, workDir string) uint64 {
	// 新しい実行の予約より前に、待機中のダイアログを打ち切る。この時点で
	// 打ち切らないと、古い実行が execMu を握ったまま応答を待ち続け、
	// この実行の goroutine がロックを取れずにデッドロックする。
	s.cancelPendingDialogs()
	id, state := s.reserveRun(true)
	go func() {
		s.execMu.Lock()
		defer s.execMu.Unlock()
		// CWDの移動と復元はexecMuを保持したまま行う（解放前に戻す）。
		restoreDir, err := runcwd.EnterWorkDir(workDir)
		if err != nil {
			state.finish(RunResult{OK: false, RunID: id, Error: err.Error()})
			return
		}
		defer restoreDir()

		screen := guilib.NewScreen()
		plugin := guilib.NewWithScreenAndWindow(screen, windows)
		registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
		host := newGUIHost(screen, windowMode, args, packed)
		host.dialog = state.dialogHandler(host)
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
	// startWithWindow と同じ理由で、ロックを取る前に待機中のダイアログを打ち切る。
	s.cancelPendingDialogs()
	id, state := s.reserveRun(true)
	go func() {
		s.execMu.Lock()
		defer s.execMu.Unlock()
		screen := guilib.NewScreen()
		plugin := s.pluginForScreen(screen)
		registry := stdlib.NewRegistry(guiPluginsWith(plugin)...)
		host := newGUIHost(screen, true, args, packed)
		host.dialog = state.dialogHandler(host)
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
			state.collectPending(host, screen)
		case <-done:
			state.collectPending(host, screen)
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
	status := state.status()
	// 実行状態は、画面が done を読んだ時点で用済み。イベントはクリックのたびに、
	// 通常実行は window.gonako.run などで繰り返し作られるので、増え続けないよう
	// ここで捨てる（#63）。done を読んだ後はどの画面ももうポーリングしない。
	// 実行後のイベントが使うVMは s.active が持っているので、捨てても影響しない。
	if status.Done {
		s.mu.Lock()
		delete(s.async, runID)
		s.mu.Unlock()
	}
	return status
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

// syncRunLockGrace は同期実行（WebViewのバインド呼び出し＝UIスレッド上）が
// execMu を待つ上限。UIスレッドを長く止めると、バックグラウンド実行が
// ネイティブのファイルダイアログ（macOSは main queue へのディスパッチ）を
// 出したときに、UIスレッドが main queue を回せずデッドロックするため短くする。
const syncRunLockGrace = 50 * time.Millisecond

// errExecBusy は同期実行が execMu を取れなかったときのエラー。
// 実行中のプログラムが終われば、もう一度実行できる。
const errExecBusy = "プログラムの実行中です。終了してからもう一度実行してください。"

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

// EventStart is what startEvent hands back to the screen: the async run id to
// poll, or the reason the event could not be started.
type EventStart struct {
	RunID uint64 `json:"runId"`
	Error string `json:"error,omitempty"`
}

// startEvent runs a screen event handler in the background and returns the
// async run id the screen polls for output・画面操作・ダイアログ.
//
// イベントを同期実行してはいけない (#59)。ハンドラの中で『言う』などが
// 呼ばれるとダイアログの応答を待つが、その応答を返すはずの画面側は
// イベント呼び出しの戻り値をまだ待っている。互いに待ち合う行き詰まりで、
// ウィンドウごと固まってしまう。通常実行と同じポーリング経路に載せれば、
// 画面はダイアログを見つけて resolveDialog を返せる。
func (s *guiSession) startEvent(runID uint64, handle int, event string, payload guilib.EventPayload) EventStart {
	// 実行中のイベントは、少しだけ待って取れなければ断る。無制限に待つと、
	// WebViewのバインド呼び出しはUIスレッド上で処理されるため、長く走る
	// プログラムの間ウィンドウごと固まってしまう。画面部品は実行中にも
	// 描かれるので、この経路は普通に踏まれる。
	// 猶予を置くのは、実行終了(finish)からexecMu解放までの僅かな隙間で
	// 押されたクリックを、実行中だと誤って断らないため。
	if !s.lockExecWithin(dispatchLockGrace) {
		return EventStart{Error: "プログラムの実行中はイベントを処理できません。"}
	}
	// 実行対象を見るのはロックを取った後。待っている間に別の実行が始まると
	// 差し替わるので、先に見ておくと古いVMへイベントを送ってしまう。
	s.mu.Lock()
	exec := s.active
	s.mu.Unlock()
	if exec == nil || exec.id != runID {
		s.execMu.Unlock()
		return EventStart{Error: "この画面は古い実行結果です。もう一度実行してください。"}
	}
	eventID, state := s.reserveRun(true)
	state.isEvent = true

	// execMu はイベントの実行が終わるまで握ったままにする。Goのミューテックスは
	// 所有者を見ないので、ロックした場所と解放する場所が違ってよい。
	go func() {
		defer s.execMu.Unlock()
		originalDir, _ := os.Getwd()
		if exec.workDir != "" {
			_ = os.Chdir(exec.workDir)
		}
		defer func() {
			if originalDir != "" {
				_ = os.Chdir(originalDir)
			}
		}()
		// ダイアログの宛先を今回のイベントに向ける。プログラム本体の実行は
		// すでに終わっており、そのままではもう誰も応答しない状態を待ってしまう。
		exec.host.dialog = state.dialogHandler(exec.host)

		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			streamPendingOutput(state, exec.host, exec.screen, done)
		}()
		err := exec.screen.DispatchEvent(handle, event, withDroppedFiles(event, payload))
		close(done)
		wg.Wait()

		result := RunResult{OK: err == nil, RunID: runID}
		if err != nil {
			result.Error = err.Error()
		}
		state.finish(result)
	}()
	return EventStart{RunID: eventID}
}
