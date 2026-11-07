package vm

// ここにはレジストリを呼び出し側が渡す実行経路を置く。
// 既定プラグインに依存しないので、js/wasm 向けのビルドでも使える (internal/wasmrt)。

import (
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// RunWithHostAndRegistry compiles and runs a program with a custom registry.
func RunWithHostAndRegistry(code, filename string, registry *stdlib.Registry, h Host) error {
	options := DefaultOptions()
	options.RealSleep = true
	return RunWithHostAndRegistryOptions(code, filename, registry, h, options)
}

// RunWithHostAndRegistryOptions は RunWithHostAndRegistry と同じ実行を、
// 指定したオプションで行う。
func RunWithHostAndRegistryOptions(code, filename string, registry *stdlib.Registry, h Host, options Options) error {
	prog, err := CompileWithRegistry(code, filename, registry)
	if err != nil {
		return err
	}
	machine := New(prog, registry, h, options)
	runErr := machine.Run()
	// 一回限りの実行なので、終了時に環境を退役させ、閉じ忘れた接続などを
	// 解放する（#363）。プラグインの状態（開いた接続など）を実行をまたいで
	// 保持したい場合は、この入口ではなく VM を保持する実行形態を使うこと。
	if closeErr := machine.Close(); runErr == nil {
		runErr = closeErr
	}
	return runErr
}

// CompileWithRegistry compiles source with an explicitly supplied command
// registry. Interactive GUI sessions use it to retain the resulting VM after
// main has returned, so browser events can call registered closures later.
func CompileWithRegistry(code, filename string, registry *stdlib.Registry) (*ir.Program, error) {
	tree, err := parser.ParseSource(code, filename, registry.FuncList())
	if err != nil {
		return nil, err
	}
	return compiler.Compile(tree, filename, registry)
}
