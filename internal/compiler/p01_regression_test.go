package compiler_test

// P01: 長い文字列定数の連結をコンパイルすると、畳み込みの上限判定が連結の後
// だったため、上限を超えた部分木を祖先の数だけ組み直していた。連結の前に
// 上限を見積もり、断念した節点を覚えておくことで、確保が項数に対して
// ほぼ三乗に増えるのを止めた。出力の意味は変えない。

import (
	"math"
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
// 400/800/1600項の3点で測り、項数に依らないベースライン（レジストリ生成
// など、実測で約330KB）は増分の差し引きで取り除いてから線形性を見る。
func TestP01ConcatAllocationStaysBounded(t *testing.T) {
	alloc := func(items int) int64 {
		tree := p01Program(p01ConcatChain(items, 100))
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		p01Compile(t, tree)
		runtime.ReadMemStats(&after)
		if after.TotalAlloc < before.TotalAlloc {
			t.Fatalf("割り当て量の計測値が逆転した: before=%d, after=%d", before.TotalAlloc, after.TotalAlloc)
		}
		return int64(after.TotalAlloc - before.TotalAlloc)
	}
	alloc(400) // 定数プールなどの初回分を温める
	small, medium, large := alloc(400), alloc(800), alloc(1600)
	firstIncrement := medium - small
	secondIncrement := large - medium

	if firstIncrement < 0 || secondIncrement < 0 {
		t.Errorf("400項 %d bytes、800項 %d bytes、1600項 %d bytes（増分: %d bytes, %d bytes）。項数の増加で割り当て量が減少した",
			small, medium, large, firstIncrement, secondIncrement)
		return
	}

	// 2026-11-05 の実測増分は 400→800 項で 131,104 bytes、
	// 800→1600 項で 273,232 bytes。後者は前者のおよそ 2 倍なので、
	// 3 倍 + 128KiB までを許容し、CI やアーキテクチャ差の揺れを吸収する。
	if secondIncrement > 3*firstIncrement+(128<<10) {
		t.Errorf("400項 %d bytes、800項 %d bytes、1600項 %d bytes（増分: %d bytes, %d bytes）。項数に対して増えすぎている",
			small, medium, large, firstIncrement, secondIncrement)
	}

	// 実測増分の約 4 倍を上限にする。一様にコストが増える退行も検出し、
	// 800項で約 2MiB になる退行は 400→800 項の増分上限を超える。
	if firstIncrement > 512<<10 || secondIncrement > 1<<20 {
		t.Errorf("400項 %d bytes、800項 %d bytes、1600項 %d bytes（増分: %d bytes, %d bytes）。増分が上限（512KiB, 1MiB）を超えた",
			small, medium, large, firstIncrement, secondIncrement)
	}
}

// p01Concat builds the 『&』 node the parser makes for `a&b`.
func p01Concat(a, b *ast.Node) *ast.Node {
	return &ast.Node{Type: ast.Op, Operator: "&", Blocks: []*ast.Node{a, b}}
}

// p01Rendered returns the string the runtime puts on the right of a 『&』 when
// its right operand is expr. The empty left operand isolates the rendering.
func p01Rendered(t *testing.T, expr *ast.Node) string {
	t.Helper()
	got, _ := p01Run(t, p01Program(p01Concat(&ast.Node{Type: ast.String, Value: ""}, expr)))
	return got
}

// p01AllocPerCompile compiles tree runs times with a fresh registry each time,
// as RunSource does, and returns the allocation per compile.
func p01AllocPerCompile(tb testing.TB, tree *ast.Node, runs int) uint64 {
	tb.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		p01Compile(tb, tree)
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(runs)
}

// p01BinaryCount returns how many binary ops the compiled main function still
// runs, i.e. how much the fold left to the runtime.
func p01BinaryCount(t *testing.T, tree *ast.Node) int {
	t.Helper()
	prog, _ := p01Compile(t, tree)
	n := 0
	for _, inst := range prog.Funcs[prog.Main].Code {
		if inst.Op == ir.OpBinary || inst.Op == ir.OpBinaryAt {
			n++
		}
	}
	return n
}

// TestP01ConcatBoundCountsNonStringOperands covers an operand that is not a
// string yet still pushes the concatenation past the limit. 『&』 renders both
// operands with the same stringification the runtime uses, so a number, a
// boolean or null contributes its rendered length. Counting that as 0 let the
// fold build a 4096+ byte string and only then have constantOf throw it away,
// which is exactly the allocation the pre-concat bound exists to avoid.
func TestP01ConcatBoundCountsNonStringOperands(t *testing.T) {
	const maxFolded = 4096 // compiler の maxFoldedString と同じ値 (非公開)
	number := func(f float64) *ast.Node { return &ast.Node{Type: ast.Number, Value: f} }
	divide := func(a, b float64) *ast.Node {
		return &ast.Node{Type: ast.Op, Operator: "/", Blocks: []*ast.Node{number(a), number(b)}}
	}
	cases := []struct {
		name    string
		operand *ast.Node
	}{
		{"最大の浮動小数", number(math.MaxFloat64)},
		{"NaN", divide(0, 0)},
		{"Infinity", divide(1, 0)},
		{"真偽値", &ast.Node{Type: ast.Bool, Value: true}},
		{"null", &ast.Node{Type: ast.Null}},
	}
	const runs = 64
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 実行時と同じ文字列化の長さ。ここが 0 と見積もられていた。
			rendered := p01Rendered(t, tc.operand)
			if len(rendered) == 0 {
				t.Fatal("文字列化した長さが 0 になった")
			}
			// 文字列化後がちょうど上限+1バイトになる左辺。
			big := strings.Repeat("a", maxFolded+1-len(rendered))
			withNonString := p01Program(p01Concat(&ast.Node{Type: ast.String, Value: big}, tc.operand))
			// 同じ長さの文字列オペランド。上限判定は文字列の長さを数えるので、
			// こちらは修正前から連結せずに断念する。確保量の基準にする。
			withString := p01Program(p01Concat(&ast.Node{Type: ast.String, Value: big},
				&ast.Node{Type: ast.String, Value: rendered}))

			// 出力の意味は変えない: 実行時に連結し、オペランドは実行時と同じ
			// 文字列になる。
			got, prog := p01Run(t, withNonString)
			if want := big + rendered; got != want {
				t.Fatalf("表示 = %d バイト, want %d バイト", len(got), len(want))
			}
			binary := 0
			for _, inst := range prog.Funcs[prog.Main].Code {
				if inst.Op == ir.OpBinary || inst.Op == ir.OpBinaryAt {
					binary++
				}
			}
			// オペランド自身が実行時に残す演算 (0/0 など) のぶんを足して、
			// 連結そのものが折り畳まれずに残っていることを見る。
			if want := p01BinaryCount(t, p01Program(tc.operand)) + 1; binary != want {
				t.Errorf("OpBinary = %d, want %d (連結は実行時に任せる)", binary, want)
			}

			// 連結を作らずに断念しているか。文字列オペランドとの差で見るので、
			// レジストリ生成など両方に共通の確保は打ち消える。
			stringAlloc := p01AllocPerCompile(t, withString, runs)
			nonStringAlloc := p01AllocPerCompile(t, withNonString, runs)
			t.Logf("1回あたりの確保: 文字列オペランド %d bytes, 非文字列オペランド %d bytes",
				stringAlloc, nonStringAlloc)
			// 非文字列側は文字列化のための小さな文字列だけを余計に確保する。
			// 連結を作っていれば 4097 バイトが上乗せされる。
			if nonStringAlloc > stringAlloc+1024 {
				t.Errorf("非文字列オペランド側が1回あたり %d bytes 余計に確保した"+
					" (文字列オペランド側 %d bytes)。連結を作ってから捨てている",
					nonStringAlloc, stringAlloc)
			}
		})
	}
	// 上限ちょうどは今までどおり畳み込む。見積もりを過大にして、実行時に
	// 任せれば済む式まで断念していないことを確かめる。
	t.Run("上限ちょうどは畳み込む", func(t *testing.T) {
		rendered := p01Rendered(t, number(math.MaxFloat64))
		big := strings.Repeat("a", maxFolded-len(rendered))
		tree := p01Program(p01Concat(&ast.Node{Type: ast.String, Value: big}, number(math.MaxFloat64)))
		got, _ := p01Run(t, tree)
		if want := big + rendered; got != want {
			t.Fatalf("表示 = %d バイト, want %d バイト", len(got), len(want))
		}
		if n := p01BinaryCount(t, tree); n != 0 {
			t.Errorf("OpBinary = %d, want 0 (上限ちょうどは畳み込む)", n)
		}
	})
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
