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
	prog, err := CompileWithRegistry(code, filename, registry)
	if err != nil {
		return err
	}
	options := DefaultOptions()
	options.RealSleep = true
	return New(prog, registry, h, options).Run()
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
