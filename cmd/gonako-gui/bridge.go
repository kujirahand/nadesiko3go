package main

// wnako3（ブラウザ版なでしこ）からGo側の命令を呼ぶブリッジ（#63）。
//
// JavaScriptは startNakoCommand(命令名, 引数JSON) で呼び出しIDを受け取り、
// 結果はGoが window.__gonakoCommandDone(ID, 結果JSON) を Eval して返す。
// WebViewのBindはUIスレッドで動くため、命令そのものはgoroutineで実行し、
// 時間のかかる命令でも画面を止めない。
//
// 命令は1つの常駐VMで実行する。SQLiteのハンドルなど命令の状態が
// 呼び出しをまたいで保たれるようにするため。
//
// 『言』『尋』『文字尋』『二択』はダイアログを出すためctx.ShowDialogを呼ぶ。
// これに応えるため、host.dialogをshowDialogに向けている。macOSのWKWebView
// はJavaScriptのalert/confirm/promptを実装しておらず（WKUIDelegateが
// runJavaScript*Panelメソッドを持たない）、wnako3自身の同名命令
// （window.alert等を直接呼ぶ）は何も表示せず素通りしてしまう（#63関連）。
// このブリッジ経由の実装をwnako3側から上書きすることで、これらの命令を
// 動くようにする（gonako-loader.jsのpluginGonakoを参照）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// BridgeResult は1回の命令呼び出しの結果。
type BridgeResult struct {
	OK     bool            `json:"ok"`
	Value  json.RawMessage `json:"value,omitempty"`
	Output string          `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type commandBridge struct {
	mu      sync.Mutex
	once    sync.Once
	machine *vm.VM
	host    *guiHost
	initErr error

	windows guilib.WindowController
	packed  *bundle.Bundle
	eval    func(js string)
	nextID  atomic.Uint64

	dialogMu      sync.Mutex
	nextDialogID  uint64
	dialogAnswers map[uint64]chan dialogAnswer

	// dialogEpoch は cancelPendingDialogs のたびに進む世代番号。
	//
	// call は実行を始めた時点の世代を callEpoch に控え、showDialog は
	// 登録の直前に現在の世代と比べる。打ち切りの走査が終わった直後に
	// 登録されたダイアログでも、世代が変わっていれば応答は来ないと
	// 分かるので、待ちに入らずエラーで b.mu を解放できる。
	// どちらも dialogMu の下で読み書きする。
	dialogEpoch uint64
	callEpoch   uint64
}

func newCommandBridge(windows guilib.WindowController, packed *bundle.Bundle, eval func(js string)) *commandBridge {
	return &commandBridge{windows: windows, packed: packed, eval: eval}
}

// prepare は常駐VMを最初の呼び出しのときに作る。エディタの起動を遅くしないため。
func (b *commandBridge) prepare() {
	screen := guilib.NewScreen()
	registry := stdlib.NewRegistry(guiPluginsWith(guilib.NewWithScreenAndWindow(screen, b.windows))...)
	host := newGUIHost(screen, false, nil, b.packed)
	host.dialog = b.showDialog
	prog, err := vm.CompileWithRegistry("", "gonako-bridge.nako3", registry)
	if err != nil {
		b.initErr = err
		return
	}
	opts := vm.DefaultOptions()
	opts.RealSleep = true
	machine := vm.New(prog, registry, host, opts)
	if err := machine.Run(); err != nil {
		b.initErr = err
		return
	}
	b.machine, b.host = machine, host
}

// close は常駐VMを退役させ、命令呼び出しをまたいで開かれたままの接続
// （SQLiteなど）を解放する（#363）。ウィンドウを閉じるときに呼ぶ。
// 常駐VMがまだ作られていなければ何もしない。
func (b *commandBridge) close() {
	// 実行中の命令（ダイアログ待ちなど）が b.mu を握っている間は待たない。
	// 終了経路で固まらないよう、短い猶予で取れなければ退役を諦める。
	// その場合はプロセスの終了時にOSが接続ごと回収する。
	deadline := time.Now().Add(dispatchLockGrace)
	for !b.mu.TryLock() {
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	defer b.mu.Unlock()
	if b.machine != nil {
		_ = b.machine.Close()
	}
}

// call は命令を同期的に実行する。引数はJSON配列の文字列で受け取る。
func (b *commandBridge) call(name, argsJSON string) BridgeResult {
	// この呼び出しが始まった時点の世代を控える。b.muを待っている間に
	// ページが遷移しても、この呼び出しは前のページのものなので、
	// ダイアログは応答が来ないものとして打ち切る（→ showDialog）。
	// ここより後にcancelPendingDialogsが走れば世代が変わる。
	b.dialogMu.Lock()
	epoch := b.dialogEpoch
	b.dialogMu.Unlock()
	return b.callAtEpoch(name, argsJSON, epoch)
}

// callAtEpoch は、呼び出しを始めた時点の世代 epoch で命令を実行する。
// start が起動時に控えた世代を渡すためにも使う。
func (b *commandBridge) callAtEpoch(name, argsJSON string, epoch uint64) BridgeResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dialogMu.Lock()
	b.callEpoch = epoch
	b.dialogMu.Unlock()
	b.once.Do(b.prepare)
	if b.initErr != nil {
		return BridgeResult{Error: b.initErr.Error()}
	}
	args, err := decodeBridgeArgs(argsJSON)
	if err != nil {
		return BridgeResult{Error: err.Error()}
	}
	v, err := b.machine.InvokeCommand(name, args)
	output := b.host.drainOutput()
	if err != nil {
		return BridgeResult{Output: output, Error: err.Error()}
	}
	text, err := stdlib.EncodeJSON(v)
	if err != nil {
		return BridgeResult{Output: output, Error: fmt.Sprintf("命令『%s』の戻り値をJSONにできません: %v", name, err)}
	}
	return BridgeResult{OK: true, Value: json.RawMessage(text), Output: output}
}

// bridgeDialogTimeout はshowDialogが応答を待つ上限。ページの移動や
// リロードでJavaScript側のコンテキストが消えると、resolveGonakoDialogは
// 二度と呼ばれない。応答がここまで来なければ諦めて呼び出し元へエラーを返し、
// b.mu（常駐VM全体のロック）を解放する。solveDialogがresolveGonakoDialog
// より先にこのタイムアウトで諦めても、後から届いた応答はresolveDialogが
// 「該当なし」としてfalseを返すだけで安全に無視される。
// リロード時はcancelPendingDialogs（→ gonako-loader.jsのunloadフック）が
// 先に待機を打ち切るので、通常はここまで待たない。
// テストが短縮できるよう変数にしてある。
var bridgeDialogTimeout = 30 * time.Minute

// showDialog は『言』『尋』『文字尋』『二択』が呼ぶ。JavaScript側に
// __gonakoBridgeDialog(id, kind, message) をEvalで届け、そこでダイアログを
// 表示させる。応答はresolveDialogがresolveGonakoDialog（Bind）経由で運ぶ。
// 打ち切りはcancelPendingDialogsが運ぶ。
// bridge.callはこの呼び出しの間ずっとb.muを握ったままになる（guiSessionの
// execMuと同じ考え方）ので、常駐VMは1度に1つのダイアログしか出さない。
//
// 登録と世代の確認はdialogMuの下で行う。cancelPendingDialogsは同じロックで
// 走査するので、打ち切りと登録が競合しても取りこぼさない（→ callEpoch）。
// 打ち切りは利用者の「キャンセル」ではないので、必ず非nilのエラーを返す。
func (b *commandBridge) showDialog(kind, message string) (string, bool, error) {
	b.dialogMu.Lock()
	if b.dialogEpoch != b.callEpoch {
		// この呼び出しが始まった後にページが遷移した。応答は来ない。
		b.dialogMu.Unlock()
		return "", false, errDialogCanceled
	}
	b.nextDialogID++
	id := b.nextDialogID
	answer := make(chan dialogAnswer, 1)
	if b.dialogAnswers == nil {
		b.dialogAnswers = map[uint64]chan dialogAnswer{}
	}
	b.dialogAnswers[id] = answer
	b.dialogMu.Unlock()

	kindJSON, _ := json.Marshal(kind)
	messageJSON, _ := json.Marshal(message)
	b.eval(fmt.Sprintf("window.__gonakoBridgeDialog && window.__gonakoBridgeDialog(%d, %s, %s)", id, kindJSON, messageJSON))

	timer := time.NewTimer(bridgeDialogTimeout)
	defer timer.Stop()
	select {
	case result := <-answer:
		return dialogAnswerOutcome(result)
	case <-timer.C:
		if result, ok := b.abandonDialog(id, answer); ok {
			// タイムアウトと同時に応答・打ち切りが決まっていた。決まった側を優先する。
			return dialogAnswerOutcome(result)
		}
		return "", false, fmt.Errorf("ダイアログの応答がありません（%v以内に応答がなく、ページの移動などで打ち切られた可能性があります）", bridgeDialogTimeout)
	}
}

// dialogAnswerOutcome は dialogAnswer を showDialog の戻り値へ変換する。
// 打ち切りのエラーを、利用者のキャンセル（accepted=false, err=nil）と
// 混同しないよう、必ずerrを優先して返す。
func dialogAnswerOutcome(result dialogAnswer) (string, bool, error) {
	if result.err != nil {
		return "", false, result.err
	}
	return result.text, result.accepted, nil
}

// resolveDialog はresolveGonakoDialog（Bind）から呼ばれ、showDialogの
// 待ちを解く。該当する呼び出しがなければ（二重応答など）falseを返す。
//
// 登録の削除と応答の送信はdialogMuを握ったまま行う。こうするとabandonDialog
// やcancelPendingDialogsから見て「登録が残っている＝未応答」「登録が消えている
// ＝決着が送信済み」のどちらかに必ず定まり、タイムアウト・打ち切りと応答が
// 競合しても応答が失われない。
// チャネルは容量1で、登録を消した者だけが送るので、送信がブロックすることはない。
func (b *commandBridge) resolveDialog(id uint64, text string, accepted bool) bool {
	b.dialogMu.Lock()
	defer b.dialogMu.Unlock()
	answer := b.dialogAnswers[id]
	if answer == nil {
		return false
	}
	delete(b.dialogAnswers, id)
	answer <- dialogAnswer{text: text, accepted: accepted}
	return true
}

// abandonDialog はタイムアウトしたshowDialogが呼ぶ。登録がまだ残っていれば
// 削除して諦める（falseを返す）。既にresolveDialogかcancelPendingDialogsが
// 登録を取っていれば、決着はチャネルへ送信済みなので、それを受け取って返す
// （trueを返す）。打ち切りの場合はdialogAnswer.errにエラーが入っている。
func (b *commandBridge) abandonDialog(id uint64, answer chan dialogAnswer) (dialogAnswer, bool) {
	b.dialogMu.Lock()
	defer b.dialogMu.Unlock()
	if b.dialogAnswers[id] == answer {
		delete(b.dialogAnswers, id)
		return dialogAnswer{}, false
	}
	return <-answer, true
}

// cancelPendingDialogs は、応答を待っているダイアログをすべて打ち切る。
//
// ページのリロード・移動（→ beforeunload / pagehide）やウィンドウ終了で
// 呼ぶ。wnako3の命令ブリッジはcallの間ずっとb.muを握ったままダイアログの
// 応答を待つので、打ち切らないと新ページからの命令呼び出しがb.muに阻まれ、
// bridgeDialogTimeout（30分）まで止まる（#183の外部URL非公開は変えない）。
//
// 登録の削除とエラーの送信はdialogMuを握ったまま行う。こうすると
// resolveDialog・タイムアウト（abandonDialog）・ここが同時に来ても、
// 「登録が残っている＝まだ誰も決着させていない」「登録が消えている＝
// 決着が送信済み」のどちらかに必ず定まり、パニック・ブロック・取りこぼしが
// 起きない。チャネルは容量1で、登録を消した者だけが送るので送信は
// ブロックしない。
//
// dialogEpochを進めるのは、この走査の直後に登録されたダイアログも確実に
// 打ち切るため。登録側（showDialog）は同じdialogMuを取って世代を確かめる
// ので、打ち切りと登録が競合しても取りこぼさない。待機が無いときの呼び出しも
// 世代は進む（戻り値は 0）。2回目が既に打ち切ったダイアログを二重に
// エラーにはしない。beforeunload と pagehide の二重発火は、JS側の
// unloadDialogsCanceled で離脱1回につき1回にまとめる。
//
// 戻り値は打ち切ったダイアログの数。テストとログのために返している。
func (b *commandBridge) cancelPendingDialogs() int {
	b.dialogMu.Lock()
	defer b.dialogMu.Unlock()
	b.dialogEpoch++
	canceled := 0
	for id, answer := range b.dialogAnswers {
		delete(b.dialogAnswers, id)
		answer <- dialogAnswer{err: errDialogCanceled}
		canceled++
	}
	return canceled
}

// start は命令をバックグラウンドで実行し、呼び出しIDをすぐ返す。
func (b *commandBridge) start(name, argsJSON string) uint64 {
	id := b.nextID.Add(1)
	// 起動した時点の世代を控える。goroutineが走り出す前にページが
	// 遷移しても、この命令は前のページのものとしてダイアログを打ち切る。
	b.dialogMu.Lock()
	epoch := b.dialogEpoch
	b.dialogMu.Unlock()
	go func() {
		result := b.callAtEpoch(name, argsJSON, epoch)
		b.eval(bridgeDoneScript(id, result))
	}()
	return id
}

// bridgeDoneScript は結果をJavaScriptへ返すEval用の文を作る。
// 結果JSONは文字列リテラルとして埋め込む（encoding/jsonはU+2028/2029もエスケープする）。
func bridgeDoneScript(id uint64, result BridgeResult) string {
	data, _ := json.Marshal(result)
	literal, _ := json.Marshal(string(data))
	return fmt.Sprintf("window.__gonakoCommandDone && window.__gonakoCommandDone(%d, %s)", id, literal)
}

// decodeBridgeArgs はJSON配列をなでしこの値の並びにする。空文字は引数なし。
func decodeBridgeArgs(argsJSON string) ([]value.Value, error) {
	if argsJSON == "" {
		return nil, nil
	}
	v, err := stdlib.DecodeJSON(argsJSON)
	if err != nil {
		return nil, errors.New("命令の引数をJSONとして読めません")
	}
	arr, ok := v.Array()
	if !ok || arr == nil {
		return nil, errors.New("命令の引数はJSON配列で指定してください")
	}
	args := make([]value.Value, arr.Len())
	for i := range args {
		args[i] = arr.Get(i)
	}
	return args, nil
}
