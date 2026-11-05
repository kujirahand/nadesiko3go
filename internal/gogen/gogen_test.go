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
	writeGoMod(t, dir)
	tidyModule(t, dir, src)

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

	// #336: 添字付き増減は対象と添字を一度だけ評価する (VMと同じ結果)
	assertMatchesVM(t, "indexed_inc_single_eval", `
件数=0
●次番とは
件数=件数+1
件数-1で戻る
ここまで
A=[10,20,30]
A[次番()]を1だけ増やす
AをJSONエンコードして表示
件数を表示
A[次番()]を1だけ減らす
AをJSONエンコードして表示
件数を表示
`)

	assertMatchesVM(t, "indexed_inc_multi_index", `
件数=0
●外番とは
件数=件数+1
0で戻る
ここまで
●内番とは
件数=件数+1
1で戻る
ここまで
A=[[10,20],[30,40]]
A[外番()][内番()]を1だけ増やす
AをJSONエンコードして表示
件数を表示
`)

	assertMatchesVM(t, "multiple_declaration_with_non_array", `
変数[A,B]=1
Aを表示
Bを表示
変数[C,D]=「abc」
Cを表示
Dを表示
変数[E,F]={「x」:2}
E[「x」]を表示
Fを表示
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

	// #420: 名前付きの入れ子関数が外側の局所変数を捕捉する。定義側で
	// 作られたクロージャを値として呼ぶ形が、生成コードでも同じ結果になる。
	assertMatchesVM(t, "nested_named_closure", `
●処理とは
件数=0
●次番とは
件数=件数+1
件数-1で戻る
ここまで
次番を表示
次番を表示
件数を表示
ここまで
処理
`)

	// #420: 兄弟の入れ子関数呼び出し。クロージャは定義側 (親) のフレームで
	// 作られているので、呼び出し側が違っても捕捉セルが正しく届く。
	assertMatchesVM(t, "nested_named_closure_sibling", `
●処理とは
件数=10
●上乗せとは
件数=件数+5
件数で戻る
ここまで
●二回とは
上乗せ
(上乗せ)で戻る
ここまで
二回を表示
ここまで
処理
`)

	// #420: 引数つき・自己再帰・前方参照・2段入れ子の形も生成コードで
	// 同じ結果になること。束縛の捕捉が生成側のセル化と噛み合うかを見る。
	assertMatchesVM(t, "nested_named_closure_args", `
●処理とは
件数=0
●(Nだけ)進めるとは
件数=件数+N
件数で戻る
ここまで
(3だけ進める)を表示
(4だけ進める)を表示
ここまで
処理
`)

	assertMatchesVM(t, "nested_named_closure_recursion", `
●処理とは
残り=3
総和=0
●数えるとは
もし残り>0ならば
総和=総和+残り
残り=残り-1
数える
ここまで
ここまで
数える
総和を表示
ここまで
処理
`)

	assertMatchesVM(t, "nested_named_closure_forward_ref", `
●処理とは
件数=41
次番を表示
●次番とは
件数=件数+1
件数で戻る
ここまで
ここまで
処理
`)

	assertMatchesVM(t, "nested_named_closure_two_levels", `
●外とは
件数=100
●中とは
●内とは
件数=件数+1
件数で戻る
ここまで
内を表示
ここまで
中
ここまで
外
`)

	// #433: 入れ子関数の相互再帰。甲 (+10) と乙 (+1) が外側の局所変数を
	// 捕捉して交互に呼び合うため、合計 33 はどちらか一方の再帰では出ず、
	// 順序コード 121212 が訪問順（甲乙甲乙甲乙）まで固定する。
	assertMatchesVM(t, "nested_named_closure_mutual_recursion", `
●処理とは
残り=6
総和=0
順序=0
●甲とは
もし残り>0ならば
総和=総和+10
順序=順序*10+1
残り=残り-1
乙
ここまで
ここまで
●乙とは
もし残り>0ならば
総和=総和+1
順序=順序*10+2
残り=残り-1
甲
ここまで
ここまで
甲
総和を表示
順序を表示
残りを表示
ここまで
処理
`)

	// #433: トップレベル関数と同名の入れ子関数。関数名は全体で一意という
	// 現行の意味論では「後に定義された方に解決される」ことを、捕捉あり
	// （処理内からの呼び出し）、捕捉なし（トップレベルからの呼び出しでも
	// 入れ子側に解決される）、定義順を逆にした形（トップレベル側に解決
	// される）の3形で固定する。
	assertMatchesVM(t, "nested_named_shadow_top_level_closure", `
●挨拶とは
「外側」で戻る
ここまで
●処理とは
件数=100
●挨拶とは
件数=件数+1
「内側{件数}」で戻る
ここまで
挨拶を表示
挨拶を表示
件数を表示
ここまで
処理
`)

	assertMatchesVM(t, "nested_named_shadow_top_level_from_top_level", `
●挨拶とは
「外側」で戻る
ここまで
●処理とは
●挨拶とは
「内側」で戻る
ここまで
挨拶を表示
ここまで
処理
挨拶を表示
`)

	assertMatchesVM(t, "nested_named_shadow_top_level_defined_first", `
●処理とは
●挨拶とは
「内側」で戻る
ここまで
挨拶を表示
ここまで
●挨拶とは
「外側」で戻る
ここまで
処理
挨拶を表示
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

	// #335: 名前で動的に取り出した関数は、静的呼び出しが見えないので
	// 引数を数値だと推論してはいけない。動的経路ごとに VM と比べる。
	assertMatchesVM(t, "dynamic_func_via_js_object_get", `
●(Aの)返却とは
  Aで戻る
ここまで
F=「返却」のJSオブジェクト取得
(F(「abc」))を表示
`)

	assertMatchesVM(t, "dynamic_func_via_await_by_name", `
●(Aの)返却とは
  Aで戻る
ここまで
(「返却」を「xyz」でAWAIT実行)を表示
`)

	// 引数なしで名前実行すると、引数は undefined であって NaN ではない。
	assertMatchesVM(t, "dynamic_func_via_exec_by_name_no_args", `
●(Aの)返却とは
  Aで戻る
ここまで
「返却」を実行して表示
`)

	assertMatchesVM(t, "dynamic_func_via_array_map_strings", `
●(Aの)返却とは
  Aで戻る
ここまで
B=[「abc」,「def」]
A=「返却」をBに配列マップ
Aを表示
`)

	assertMatchesVM(t, "dynamic_func_via_array_map_array_and_dict", `
●(Aの)返却とは
  Aで戻る
ここまで
B=[[1,2],{"k":"v"}]
A=「返却」をBに配列マップ
(AをJSONエンコード)を表示
`)

	// #432: 動的対象 (文字列名で配列マップ) と静的数値呼び出しのみの
	// 関数が混在しても出力が一致する。細分化で対象だけが抑制され、
	// 静的関数の特殊化が維持される側は解析テストで固定している。
	assertMatchesVM(t, "dynamic_func_granular_mixed", `
●(Aの)返却とは
  Aで戻る
ここまで
●(Nの)二倍とは
  (N*2)で戻る
ここまで
B=[「abc」,「def」]
A=「返却」をBに配列マップ
Aを表示
(21の二倍)を表示
`)

	// #432: 同じ関数が静的に数値で呼ばれ、かつ
	// 動的に文字列で呼ばれるケース。静的呼び出しだけを見て引数を
	// 数値だと決めると、動的経由の文字列が NaN になる (#335 の形)。
	assertMatchesVM(t, "dynamic_func_same_func_static_and_dynamic", `
●(Aの)返却とは
  Aで戻る
ここまで
(21の返却)を表示
B=[「abc」,「def」]
A=「返却」をBに配列マップ
Aを表示
`)

	// #432: 名前が変数で渡る場合は全体抑制へフォールバックするが、
	// 出力は従来どおり VM と一致する。
	assertMatchesVM(t, "dynamic_func_via_variable_name", `
●(Aの)返却とは
  Aで戻る
ここまで
N=「返却」
B=[「abc」,「def」]
A=NをBに配列マップ
Aを表示
`)

	// 動的経路のない静的な数値呼び出しは、従来どおりの出力のまま。
	assertMatchesVM(t, "static_numeric_func_call", `
●(Nの)二倍とは
  (N*2)で戻る
ここまで
(21の二倍)を表示
S=0
Iを1から10まで繰り返す
  S=S+(Iの二倍)
ここまで
Sを表示
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
