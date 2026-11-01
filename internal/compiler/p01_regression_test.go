package compiler_test

// P01: 長い文字列定数の連結をコンパイルすると、畳み込みの上限判定が連結の後
// だったため、上限を超えた部分木を祖先の数だけ組み直していた。連結の前に
// 上限を見積もり、断念した節点を覚えておくことで、確保が項数に対して
// ほぼ三乗に増えるのを止めた。出力の意味は変えない。

import (
	"runtime"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// p01Literal returns a string literal node of the given byte size.
func p01Literal(size int) *ast.Node {
	return &ast.Node{Type: ast.String, Value: strings.Repeat("a", size)}
}

// p01ConcatChain builds the left-associative 『&』 chain that the parser makes
// for `("…"&"…"&…)を表示`, without going through the parser.
func p01ConcatChain(items, size int) *ast.Node {
	node := p01Literal(size)
	for i := 1; i < items; i++ {
		node = &ast.Node{Type: ast.Op, Operator: "&", Blocks: []*ast.Node{node, p01Literal(size)}}
	}
	return node
}

// p01Program wraps expr in 『表示』.
func p01Program(expr *ast.Node) *ast.Node {
	return &ast.Node{Type: ast.Block, Blocks: []*ast.Node{{
		Type:   ast.Func,
		Name:   "表示",
		Blocks: []*ast.Node{expr},
	}}}
}

// p01Source writes the same program as source text, so the parser can be
// measured separately from the compiler.
func p01Source(items, size int) string {
	parts := make([]string, items)
	for i := range parts {
		parts[i] = `"` + strings.Repeat("a", size) + `"`
	}
	return "(" + strings.Join(parts, "&") + ")を表示\n"
}

// p01Compile compiles the tree with a fresh registry, as RunSource does.
func p01Compile(tb testing.TB, tree *ast.Node) (*ir.Program, *stdlib.Registry) {
	tb.Helper()
	registry := stdlib.NewRegistry()
	prog, err := compiler.Compile(tree, "p01.nako3", registry)
	if err != nil {
		tb.Fatalf("コンパイルに失敗した: %v", err)
	}
	return prog, registry
}

// p01Run compiles and runs the tree, returning the 『表示』 output and the IR.
func p01Run(t *testing.T, tree *ast.Node) (string, *ir.Program) {
	t.Helper()
	prog, registry := p01Compile(t, tree)
	out := &vm.Collector{}
	if err := vm.New(prog, registry, out, vm.DefaultOptions()).Run(); err != nil {
		t.Fatalf("実行に失敗した: %v", err)
	}
	return out.Log(), prog
}

// TestP01ConcatBudgetKeepsOutput holds the 4096-byte boundary: the fold may
// or may not happen, but the printed string must be the same either way.
func TestP01ConcatBudgetKeepsOutput(t *testing.T) {
	for _, total := range []int{4095, 4096, 4097, 10000} {
		left := strings.Repeat("a", total-1)
		tree := p01Program(&ast.Node{Type: ast.Op, Operator: "&", Blocks: []*ast.Node{
			{Type: ast.String, Value: left},
			{Type: ast.String, Value: "b"},
		}})
		got, prog := p01Run(t, tree)
		if want := left + "b"; got != want {
			t.Errorf("%d バイトの連結: 表示 = %d バイト, want %d バイト", total, len(got), len(want))
		}
		// 上限まで(4096 バイト)は定数プールに載り、超えたら実行時に連結する。
		binary := 0
		for _, inst := range prog.Funcs[prog.Main].Code {
			if inst.Op == ir.OpBinary || inst.Op == ir.OpBinaryAt {
				binary++
			}
		}
		wantBinary := 0
		if total > 4096 {
			wantBinary = 1
		}
		if binary != wantBinary {
			t.Errorf("%d バイトの連結: OpBinary = %d, want %d", total, binary, wantBinary)
		}
	}
}

// TestP01LongChainOutput pins the values of the reported sizes: each program
// must print exactly items*size bytes of 『a』.
func TestP01LongChainOutput(t *testing.T) {
	for _, items := range []int{100, 200, 400, 800} {
		got, _ := p01Run(t, p01Program(p01ConcatChain(items, 100)))
		if want := strings.Repeat("a", items*100); got != want {
			t.Fatalf("%d 項: 表示 = %d バイト, want %d バイト", items, len(got), len(want))
		}
	}
}

// TestP01SourceChainMatchesAST checks the real source path (parser included)
// produces the same program as the directly built AST.
func TestP01SourceChainMatchesAST(t *testing.T) {
	tree, err := vm.ParseProgram(p01Source(800, 100), "p01.nako3")
	if err != nil {
		t.Fatalf("構文解析に失敗した: %v", err)
	}
	want, _ := p01Run(t, p01Program(p01ConcatChain(800, 100)))
	got, _ := p01Run(t, tree)
	if got != want {
		t.Fatalf("実ソースの表示 = %d バイト, AST 直構築 = %d バイト", len(got), len(want))
	}
}

// TestP01ConcatAllocationStaysBounded is the measurement the issue asks for:
// doubling the items must not multiply the compile-time allocation.
func TestP01ConcatAllocationStaysBounded(t *testing.T) {
	alloc := func(items int) uint64 {
		tree := p01Program(p01ConcatChain(items, 100))
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		p01Compile(t, tree)
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	alloc(400) // 定数プールなどの初回分を温める
	small, large := alloc(400), alloc(800)
	if large > 3*small+(1<<20) {
		t.Errorf("400項 %d bytes に対して800項 %d bytes。項数に対して増えすぎている", small, large)
	}
	if large > 8<<20 {
		t.Errorf("800項のコンパイルで %d bytes を割り当てた (上限 8MiB)", large)
	}
}

// TestP01ShortCircuitAndPropagation keeps the meanings the fold must not
// change: 『かつ』『または』 still short-circuit, and a 『定数』 still folds.
func TestP01ShortCircuitAndPropagation(t *testing.T) {
	code := "●右辺\n" +
		"　「右辺を評価」と表示\n" +
		"　1で戻る\n" +
		"ここまで\n" +
		"(1=2)かつ右辺を表示\n" +
		"(1=1)または右辺を表示\n" +
		"Nとは定数=「あ」\n" +
		"(N&N&N)を表示\n"
	got, err := vm.RunSource(code, "p01.nako3", nil)
	if err != nil {
		t.Fatalf("実行に失敗した: %v", err)
	}
	if want := "false\ntrue\nあああ"; got.Log != want {
		t.Errorf("表示 = %q, want %q", got.Log, want)
	}
}

func benchmarkP01AST(b *testing.B, items int) {
	tree := p01Program(p01ConcatChain(items, 100))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p01Compile(b, tree)
	}
}

func BenchmarkP01AST100(b *testing.B) { benchmarkP01AST(b, 100) }
func BenchmarkP01AST200(b *testing.B) { benchmarkP01AST(b, 200) }
func BenchmarkP01AST400(b *testing.B) { benchmarkP01AST(b, 400) }
func BenchmarkP01AST800(b *testing.B) { benchmarkP01AST(b, 800) }

// BenchmarkP01Source800 compiles the real source form, parser excluded, so the
// number is comparable with BenchmarkP01AST800.
func BenchmarkP01Source800(b *testing.B) {
	tree, err := vm.ParseProgram(p01Source(800, 100), "p01.nako3")
	if err != nil {
		b.Fatalf("構文解析に失敗した: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p01Compile(b, tree)
	}
}

// BenchmarkP01Parse800 is the parser side of the real source path, which the
// AST figures deliberately leave out.
func BenchmarkP01Parse800(b *testing.B) {
	code := p01Source(800, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := vm.ParseProgram(code, "p01.nako3"); err != nil {
			b.Fatalf("構文解析に失敗した: %v", err)
		}
	}
}
