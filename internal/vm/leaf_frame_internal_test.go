package vm

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func TestLeafFrameReturnedAfterMonitoredError(t *testing.T) {
	const source = `●(Xで)失敗とは
「失敗」のエラー発生
ここまで
●呼ぶとは
エラー監視
  0で失敗
エラーならば
ここまで
ここまで
呼ぶ
`
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(source, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(prog, registry, &Collector{}, DefaultOptions())
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}

	// main -> 呼ぶ -> 失敗 are all leaf activations. The innermost frame
	// unwinds by panic and must be returned just like the two normal returns.
	if got, want := len(machine.leafFrames), 3; got != want {
		t.Fatalf("returned leaf frames = %d, want %d", got, want)
	}
}
