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

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

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
}

func newCommandBridge(windows guilib.WindowController, packed *bundle.Bundle, eval func(js string)) *commandBridge {
	return &commandBridge{windows: windows, packed: packed, eval: eval}
}

// prepare は常駐VMを最初の呼び出しのときに作る。エディタの起動を遅くしないため。
func (b *commandBridge) prepare() {
	screen := guilib.NewScreen()
	registry := stdlib.NewRegistry(guiPluginsWith(guilib.NewWithScreenAndWindow(screen, b.windows))...)
	host := newGUIHost(screen, false, nil, b.packed)
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

// call は命令を同期的に実行する。引数はJSON配列の文字列で受け取る。
func (b *commandBridge) call(name, argsJSON string) BridgeResult {
	b.mu.Lock()
	defer b.mu.Unlock()
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

// start は命令をバックグラウンドで実行し、呼び出しIDをすぐ返す。
func (b *commandBridge) start(name, argsJSON string) uint64 {
	id := b.nextID.Add(1)
	go func() {
		result := b.call(name, argsJSON)
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
