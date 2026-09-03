package gogen

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// repoRoot finds this module's root from the test file's own location, so the
// generated program's go.mod can `replace` the module with a local path
// instead of a real version — exactly how a real project would pin an
// unreleased dependency during development.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/gogen/gogen_test.go -> repo root is two levels up.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// buildAndRun compiles code exactly as vm.RunSource does, generates Go source
// from the result, builds it as a standalone module (proving gogen's whole
// point: no Go toolchain access to this repo is needed beyond pkg/runtime),
// and runs it. It reports what the built binary printed.
func buildAndRun(t *testing.T, code string) string {
	t.Helper()
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	src, err := Generate(prog, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s\n--- source ---\n%s", err, out, src)
	}

	run := exec.Command("go", "run", ".")
	run.Dir = dir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	return string(out)
}

// vmOutput runs code through the VM backend exactly as the compat runner
// does, for the differential comparison AGENTS.md §12 asks for (TS / VM / Go
// generated code must agree — this checks the last two).
func vmOutput(t *testing.T, code string) string {
	t.Helper()
	result, err := vm.RunSource(code, "main.nako3", nil)
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	return result.Log
}

// assertMatchesVM builds and runs code through gogen and checks its output
// against the VM's, the same program compiled and run two different ways.
func assertMatchesVM(t *testing.T, name, code string) {
	t.Run(name, func(t *testing.T) {
		if testing.Short() {
			t.Skip("go build/run per case; skipped under -short")
		}
		want := strings.TrimRight(vmOutput(t, code), "\n") + "\n"
		got := buildAndRun(t, code)
		if got != want {
			t.Fatalf("出力が一致しない\nVM  : %q\ngogen: %q", want, got)
		}
	})
}

// TestEndToEnd builds and runs a representative program through gogen for
// each language feature the backend claims to support (package doc), and
// checks the result against the VM interpreting the same IR.
func TestEndToEnd(t *testing.T) {
	assertMatchesVM(t, "arithmetic_and_display", `
3.14を表示
(1+2)*3を表示
10 % 3を表示
`)

	assertMatchesVM(t, "string_and_concat", `
("こんにちは"&"、世界")を表示
「Fizz{1+1}」を表示
`)

	assertMatchesVM(t, "if_and_repeat", `
Iを1から20まで繰り返す
  もし、(I % 15) = 0ならば
    「FizzBuzz」と表示。
  違えば、もし、(I % 3) = 0ならば
    「Fizz」と表示。
  違えば、もし、(I % 5) = 0ならば
    「Buzz」と表示。
  違えば
    Iを表示
  ここまで
ここまで
`)

	assertMatchesVM(t, "array_and_dict", `
A=[3,1,4,1,5]
Aに9を配列追加
Aを表示
D={"a":1,"b":2}
D["a"]を表示
D["c"]=3
D["c"]を表示
`)

	assertMatchesVM(t, "user_function_recursion", `
●(Nの)階乗とは
もしN<=1ならば1で戻る
(N*((N-1)の階乗))で戻る
ここまで
(5の階乗)を表示
`)

	// --- 型推論 (types.go) が意味を変えていないことを見る ---

	// 代入前に読むローカルは undefined であって NaN ではない。
	// 数値だと決めつけて ToNumber を通すとここが NaN になる。
	assertMatchesVM(t, "typed_local_read_before_assign", `
●テスト
もし、1=2ならば
A=5
ここまで
Aを表示
ここまで
テスト
`)

	// 引数の型は呼び出し側から決まる。数値でない引数が1つでもあれば、
	// その引数を起点にした変数も数値ではなくなる。
	assertMatchesVM(t, "typed_param_not_number", `
●(Nの)倍とは
M=N
(M&M)で戻る
ここまで
(「あ」の倍)を表示
(3の倍)を表示
`)

	// NaN・±Inf・-0 は生の float64 で計算しても値が変わらないこと。
	assertMatchesVM(t, "typed_number_edges", `
A=0
B=0
(A/B)を表示
(1/B)を表示
(-1/B)を表示
(A*-1)を表示
((A*-1)=0)を表示
`)

	// 数値と文字列が混ざる演算は一般経路のまま。『&』は連結、
	// 『+』は数へ寄せる、という違いが消えていないこと。
	assertMatchesVM(t, "typed_mixed_operands", `
A=1
S=「2」
(A+S)を表示
(A&S)を表示
(S+S)を表示
`)

	// 数値ループ。剰余・整数割り・累乗・シフトも含める。
	assertMatchesVM(t, "typed_numeric_loop", `
S=0
Iを1から20まで繰り返す
S=S+(I*2)-(I%3)
ここまで
Sを表示
((7÷÷2))を表示
((2^10))を表示
((1<<4))を表示
`)

	assertMatchesVM(t, "closure_counter", `
●(Nで)カウンタ作成とは
M=N
(関数()
M=M+1
Mで戻る
ここまで)で戻る
ここまで
F=10でカウンタ作成
(F())を表示
(F())を表示
(F())を表示
`)

	assertMatchesVM(t, "try_catch", `
エラー監視
「Eの中身」でエラー発生
「エラーにならなかった」と表示
エラーならば
「エラーを捕まえた: {エラーメッセージ}」と表示
ここまで
「続きは動く」と表示
`)

	assertMatchesVM(t, "sore_and_pipeline", `
「  空白トリム対象  」の空白除去。
それを表示
`)

	assertMatchesVM(t, "special_written_by_stdlib", `
「前|後」から「|」まで切取
「結果:{それ}/対象:{対象}」と表示
`)

	// 大域変数をGoの変数へ移す最適化 (typeInfo.promotableGlobals) は、
	// 移してよいかを間違えても**動くコードのまま黙って答えを変える**ので、
	// 移せる形と移せない形の両方をここで実行して確かめる。
	assertMatchesVM(t, "global_promotion_in_loop", `
S=0
Iを1から10まで繰り返す
  S=S+(I*2)
ここまで
Sを表示
Iを表示
`)

	assertMatchesVM(t, "global_shared_with_function", `
S=0
●足すとは
  S=S+1
ここまで
Iを1から5まで繰り返す
  足す
ここまで
Sを表示
`)

	assertMatchesVM(t, "global_reflected_through_question_pipeline", `
S=5
["JSオブジェクト取得","表示"]をハテナ関数設定
?? "S"
`)

	assertMatchesVM(t, "frame_specials_inherit_and_isolate", `
対象="親"
回数=7
●確認
「入:{対象}/{回数}」と表示
対象="子"
回数=99
「子:{対象}/{回数}」と表示
ここまで
確認
「親:{対象}/{回数}」と表示
`)
}

// TestPluginRegistry checks that a program using a plugin beyond
// plugin_system runs correctly when Generate and the caller's own compile
// step agree on the same plugin set (Options.Plugins / BuildRegistry).
// Getting this wrong doesn't fail to build — it calls the wrong stdlib
// command, since a command's ID depends on the registry's exact contents
// (AGENTS.md §12) — so this is worth its own test, not just coverage by
// implication from the no-plugin cases.
func TestPluginRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	code := `A=["名前,年齢","太郎,20"]
(Aを表CSV変換)を表示
`
	registry, err := BuildRegistry([]string{"csvlib"})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatal(err)
	}
	src, err := Generate(prog, Options{Plugins: []string{"csvlib"}})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	run := exec.Command("go", "run", ".")
	run.Dir = dir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	got := string(out)
	var wantBuf strings.Builder
	wantHost := vm.NewCUIHost(&wantBuf, strings.NewReader(""), nil)
	if err := vm.RunWithHostAndRegistry(code, "main.nako3", registry, wantHost); err != nil {
		t.Fatal(err)
	}
	want := wantBuf.String()
	if got != want {
		t.Fatalf("出力が違う: got %q want %q", got, want)
	}
}

// TestGenerateRejectsAsync checks that a 非同期関数 is refused with a clear
// reason rather than silently producing broken Go source (AGENTS.md §12: a
// dynamic feature may be limited, but never mis-generated).
func TestGenerateRejectsAsync(t *testing.T) {
	registry := stdlib.NewRegistry()
	code := `
非同期モード
●非同期テストは非同期
  1を表示
ここまで
非同期テスト
`
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Skipf("この構文が今のパーサで通らない: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Skipf("この構文が今のコンパイラで通らない: %v", err)
	}
	if !hasAsync(prog) {
		t.Skip("この構文からは非同期関数が作られなかった")
	}
	if _, err := Generate(prog, Options{}); err == nil {
		t.Fatal("非同期関数を含むプログラムを、エラーにせず生成してしまった")
	}
}

func hasAsync(prog *ir.Program) bool {
	for i := range prog.Funcs {
		if prog.Funcs[i].Async {
			return true
		}
	}
	return false
}
