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

// runMu は同時に複数のプログラムが走らないようにする。
// 出力の混線を避けるため、run の呼び出し順に1本ずつ実行する。
var runMu sync.Mutex

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
		resolve := p[0]
		go func() {
			resolve.Invoke(js.ValueOf(execute(code, options)))
		}()
		return nil
	})
	// Promiseのexecutorは生成時に同期で呼ばれるので、生成後に解放してよい
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
}

// execute はプログラムを実行し、JSへ渡す結果オブジェクトを作る。
func execute(code string, options js.Value) (result map[string]any) {
	runMu.Lock()
	defer runMu.Unlock()

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
