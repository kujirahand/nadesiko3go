package vm_test

import (
	"sync/atomic"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// countingCloserPlugin は Close の呼び出し回数を数えるテスト用プラグイン。
// finalizer に頼らず、VM → Registry → Plugin の退役配線そのものを
// 決定的に検証するために使う（#363）。
type countingCloserPlugin struct{ closes atomic.Int32 }

func (p *countingCloserPlugin) FuncList() lexer.FuncList { return lexer.FuncList{} }
func (p *countingCloserPlugin) Impls() map[string]stdlib.Impl {
	return map[string]stdlib.Impl{}
}
func (p *countingCloserPlugin) Close() error {
	p.closes.Add(1)
	return nil
}

func TestVMCloseClosesRegistryPluginsExactlyOnce(t *testing.T) {
	plugin := &countingCloserPlugin{}
	registry := stdlib.NewRegistry(plugin)
	prog, err := vm.CompileWithRegistry("「1」と表示", "main.nako3", registry)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(prog, registry, &vm.Collector{}, vm.DefaultOptions())
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if got := plugin.closes.Load(); got != 0 {
		t.Fatalf("Run の時点では閉じないはず: closes=%d", got)
	}
	if err := machine.Close(); err != nil {
		t.Fatal(err)
	}
	if err := machine.Close(); err != nil {
		t.Fatal(err)
	}
	if got := plugin.closes.Load(); got != 1 {
		t.Fatalf("VM.Close はプラグインをちょうど1回閉じるはず: closes=%d", got)
	}
}

func TestRunWithHostAndRegistryClosesPlugins(t *testing.T) {
	plugin := &countingCloserPlugin{}
	registry := stdlib.NewRegistry(plugin)
	if err := vm.RunWithHostAndRegistry("「1」と表示", "main.nako3", registry, &vm.Collector{}); err != nil {
		t.Fatal(err)
	}
	if got := plugin.closes.Load(); got != 1 {
		t.Fatalf("一回限りの入口は実行後にプラグインを閉じるはず: closes=%d", got)
	}
	// 同じレジストリでもう一度実行しても、その実行の分が解放される
	// （Registry.Close は一度きりではない）。
	if err := vm.RunWithHostAndRegistry("「2」と表示", "main.nako3", registry, &vm.Collector{}); err != nil {
		t.Fatal(err)
	}
	if got := plugin.closes.Load(); got != 2 {
		t.Fatalf("レジストリ再利用時も実行ごとに閉じるはず: closes=%d", got)
	}
}
