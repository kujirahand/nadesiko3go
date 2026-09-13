//go:build js && wasm

// gonako-wasm は、なでしこ3のコア機能をブラウザで動かすWebAssembly版です。
//
// ビルド: GOOS=js GOARCH=wasm go build -o gonako.wasm ./cmd/gonako-wasm
// （just wasm で wasm_exec.js とサンプルページも一緒に bin/wasm へ出力します）
//
// 読み込むと、JavaScriptのグローバルに gonako オブジェクトを公開します。
//
//	gonako.version                      バージョン文字列
//	gonako.run(code, options) → Promise 実行結果 {ok, output, error}
//
// options は省略できます。
//
//	filename  エラー表示に使うファイル名（既定: main.nako3）
//	args      プログラムに渡す引数（文字列の配列）
//	onPrint   『表示』のたびに1行を受け取る関数
//	onWrite   改行しない出力を受け取る関数
//	onDialog  (kind, message) を受け取る関数。kind は alert/confirm/prompt。
//	          省略時はブラウザの alert/confirm/prompt を使い、無ければ『表示』に切り替える
//
// 準備ができると globalThis.gonakoReady が関数なら gonako を引数に呼びます。
package main

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/version"
	"github.com/kujirahand/nadesiko3go/internal/wasmrt"
)

// job は gonako.run の1回分の呼び出しを表す。
type job struct {
	code    string
	options js.Value
	resolve js.Value
}

// queue と workerRunning は、run の呼び出し順に1本ずつ実行するためのFIFOキュー。
//
// 以前は呼び出しごとに goroutine を作って mutex を取り合わせていたが、Goの
// スケジューラは新しい goroutine を先に走らせることがある（`go`文で作った
// goroutineは実行キューの `runnext` に入り、さらに次の goroutine が来ると
// そちらに追い出される）ため、mutexの取得順は呼び出し順と一致しなかった。
// ここでは、JSからの呼び出し自体は必ず1本ずつ（JSはシングルスレッド）である
// ことを使い、Promiseのexecutor内（goroutineを作る前の同期区間）でキューに
// 積むことで、追加順=呼び出し順を保証する。
var (
	queueMu       sync.Mutex
	queue         []job
	workerRunning bool
)

// submit はジョブをキューの末尾に積む。呼び出し自体はブロックしない。
// キューが空だった（＝ワーカーが止まっていた）ときだけワーカーを起こす。
func submit(j job) {
	queueMu.Lock()
	queue = append(queue, j)
	start := !workerRunning
	if start {
		workerRunning = true
	}
	queueMu.Unlock()
	if start {
		go worker()
	}
}

// worker はキューが空になるまで、先頭から1本ずつ実行する。
func worker() {
	for {
		queueMu.Lock()
		if len(queue) == 0 {
			workerRunning = false
			queueMu.Unlock()
			return
		}
		j := queue[0]
		queue = queue[1:]
		queueMu.Unlock()

		j.resolve.Invoke(js.ValueOf(execute(j.code, j.options)))
	}
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("version", version.Version)
	api.Set("run", js.FuncOf(run))
	js.Global().Set("gonako", api)

	if ready := js.Global().Get("gonakoReady"); ready.Type() == js.TypeFunction {
		ready.Invoke(api)
	}
	// JSから呼ばれ続けるので、main は終わらせない
	select {}
}

// run は gonako.run の本体。『秒待機』で実時間を待てるように、
// 実行はgoroutineで行い、結果はPromiseで返す。
func run(_ js.Value, args []js.Value) any {
	code := ""
	if len(args) > 0 {
		code = args[0].String()
	}
	options := js.Undefined()
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		options = args[1]
	}

	executor := js.FuncOf(func(_ js.Value, p []js.Value) any {
		// ここは同期で呼ばれる区間なので、積む順序がJSの呼び出し順と一致する
		submit(job{code: code, options: options, resolve: p[0]})
		return nil
	})
	// Promiseのexecutorは生成時に同期で呼ばれるので、生成後に解放してよい
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
}

// execute はプログラムを実行し、JSへ渡す結果オブジェクトを作る。
// worker が1本ずつ呼ぶので、複数の実行が同時に動くことはない。
func execute(code string, options js.Value) (result map[string]any) {
	h := &wasmrt.Host{Dialog: browserDialog}
	filename := wasmrt.DefaultFilename
	if options.Type() == js.TypeObject {
		if v := options.Get("filename"); v.Type() == js.TypeString {
			filename = v.String()
		}
		if v := options.Get("args"); v.Type() == js.TypeObject {
			for i := 0; i < v.Length(); i++ {
				h.CmdArgs = append(h.CmdArgs, v.Index(i).String())
			}
		}
		if fn := options.Get("onPrint"); fn.Type() == js.TypeFunction {
			h.OnPrint = func(s string) { fn.Invoke(s) }
		}
		if fn := options.Get("onWrite"); fn.Type() == js.TypeFunction {
			h.OnWrite = func(s string) { fn.Invoke(s) }
		}
		if fn := options.Get("onDialog"); fn.Type() == js.TypeFunction {
			h.Dialog = func(kind, message string) (string, bool, bool, error) {
				return dialogResult(kind, fn.Invoke(kind, message))
			}
		}
	}

	// Go側のpanicでwasm全体が止まらないように、エラーとして返す
	defer func() {
		if r := recover(); r != nil {
			result = makeResult(h, fmt.Errorf("内部エラー: %v", r))
		}
	}()
	return makeResult(h, wasmrt.Run(code, filename, h))
}

func makeResult(h *wasmrt.Host, err error) map[string]any {
	result := map[string]any{
		"ok":     err == nil,
		"output": h.Output(),
		"error":  nil,
	}
	if err == nil {
		return result
	}
	info := map[string]any{"message": err.Error(), "kind": "", "file": "", "line": 0}
	var ne *errs.NakoError
	if errors.As(err, &ne) {
		info["kind"] = ne.Kind.String()
		info["file"] = ne.File
		info["line"] = ne.Line + 1
	}
	result["error"] = info
	return result
}

// browserDialog はブラウザの alert/confirm/prompt を使う。
// Node.jsなどで関数が無ければ未対応として返し、命令側に『表示』へ切り替えさせる。
func browserDialog(kind, message string) (string, bool, bool, error) {
	fn := js.Global().Get(kind)
	if fn.Type() != js.TypeFunction {
		return "", false, false, nil
	}
	return dialogResult(kind, fn.Invoke(message))
}

// dialogResult はダイアログの戻り値を vm の形に直す。
// prompt はキャンセルで null、confirm は真偽値を返す。
func dialogResult(kind string, v js.Value) (string, bool, bool, error) {
	switch v.Type() {
	case js.TypeNull, js.TypeUndefined:
		return "", kind == "alert", true, nil
	case js.TypeBoolean:
		return "", v.Bool(), true, nil
	default:
		return v.String(), true, true, nil
	}
}
