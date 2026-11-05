package gogen

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestDynamicFuncCommandsResolve は、名前で関数を引く命令の一覧
// (dynamicFuncCommands / dynamicGlobalCommands) の綴りが実在の命令と
// 一致していることを固定する。一覧の名前がずれると Lookup が黙って
// 素通りし、#335 の誤数値化がその命令経由で再発する。
func TestDynamicFuncCommandsResolve(t *testing.T) {
	registry, err := BuildRegistry(PluginNames())
	if err != nil {
		t.Fatal(err)
	}
	for name := range dynamicFuncCommands {
		if _, ok := registry.Lookup(name); !ok {
			t.Errorf("dynamicFuncCommands の %q がレジストリにない", name)
		}
	}
	for _, name := range dynamicGlobalCommands {
		if _, ok := registry.Lookup(name); !ok {
			t.Errorf("dynamicGlobalCommands の %q がレジストリにない", name)
		}
	}
}

// compileForAnalyze は code をコンパイルし、gogen の環境解析までを返す。
func compileForAnalyze(t *testing.T, code string, plugins []string) (env analyzeEnv, info *typeInfo, prog *ir.Program, funcs []string) {
	t.Helper()
	registry, err := BuildRegistry(plugins)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err = compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	env, err = analyzeEnvFor(prog, plugins)
	if err != nil {
		t.Fatalf("analyzeEnvFor: %v", err)
	}
	info = analyze(prog, env)
	for i := range prog.Funcs {
		funcs = append(funcs, prog.Funcs[i].Name)
	}
	return env, info, prog, funcs
}

// funcIndexesBySuffix は、名前が "__"+suffix で終わる (または完全一致する)
// 関数のインデックスを全部返す。
func funcIndexesBySuffix(funcs []string, suffix string) []int {
	var out []int
	for i, name := range funcs {
		if name == suffix || strings.HasSuffix(name, "__"+suffix) {
			out = append(out, i)
		}
	}
	return out
}

// TestDynamicFuncsKeepsGlobalPromotion は、動的経路があっても主処理の
// 大域変数の昇格まで止めないことを固定する。主処理は FindFunc の対象外
// なので escapes に入れず、promotableGlobals が従来どおり働く。
// #432 以降、このプログラム (文字列リテラル指定) では全体抑制は立たず、
// 対象の『返却』だけが escapes になる。
func TestDynamicFuncsKeepsGlobalPromotion(t *testing.T) {
	code := "●(Aの)返却とは\n  Aで戻る\nここまで\n" +
		"S=0\nIを1から3まで繰り返す\n  S=S+I\nここまで\n" +
		"B=[「abc」]\nC=「返却」をBに配列マップ\nSを表示\n"
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	env, err := analyzeEnvFor(prog, nil)
	if err != nil {
		t.Fatalf("analyzeEnvFor: %v", err)
	}
	if env.dynamicFuncs {
		t.Fatal("文字列リテラル指定なのに全体抑制 (dynamicFuncs) が立っている")
	}
	var names []string
	for i := range prog.Funcs {
		names = append(names, prog.Funcs[i].Name)
	}
	targets := funcIndexesBySuffix(names, "返却")
	if len(targets) != 1 || !env.dynamicTargets[targets[0]] {
		t.Fatalf("dynamicTargets = %v, 『返却』だけが対象のはず", env.dynamicTargets)
	}
	if env.dynamicGlobals {
		t.Fatal("このプログラムでは dynamicGlobals は立たないはず")
	}
	info := analyze(prog, env)
	var got []string
	for slot := range info.promotedGlobals {
		got = append(got, prog.Globals[slot])
	}
	sort.Strings(got)
	want := []string{"main__I", "main__S"}
	if len(got) != len(want) {
		t.Fatalf("移した大域変数 = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("移した大域変数 = %v, want %v", got, want)
		}
	}
}

// TestDynamicFuncParams は、名前で取り出されうる関数の引数を数値だと
// 推論しないことを、生成バイナリを実行せず解析結果で直接固定する。
// 文字列リテラルで渡された場合は対象関数だけが escapes になり全体抑制
// (dynamicFuncs) は立たない (#432)。静的な数値呼び出しだけの関数は
// 従来どおり特殊化されることも見る。
func TestDynamicFuncParams(t *testing.T) {
	def := "●(Aの)返却とは\n  Aで戻る\nここまで\n"
	cases := []struct {
		name         string
		code         string
		funcName     string
		wantFallback bool
		wantEscapes  bool
		wantNumeric  bool
	}{
		{
			name:         "static_numeric_call_only",
			code:         "●(Nの)二倍とは\n  (N*2)で戻る\nここまで\n(21の二倍)を表示\n",
			funcName:     "二倍",
			wantFallback: false,
			wantEscapes:  false,
			wantNumeric:  true,
		},
		{
			name:         "js_object_get_and_call_value",
			code:         def + "F=「返却」のJSオブジェクト取得\n(F(「abc」))を表示\n",
			funcName:     "返却",
			wantFallback: false,
			wantEscapes:  true,
			wantNumeric:  false,
		},
		{
			name:         "array_map_by_name",
			code:         def + "A=「返却」を[「abc」]に配列マップ\nAを表示\n",
			funcName:     "返却",
			wantFallback: false,
			wantEscapes:  true,
			wantNumeric:  false,
		},
		{
			name:         "await_by_name",
			code:         def + "(「返却」を「xyz」でAWAIT実行)を表示\n",
			funcName:     "返却",
			wantFallback: false,
			wantEscapes:  true,
			wantNumeric:  false,
		},
		{
			name:         "exec_by_name",
			code:         def + "「返却」を実行して表示\n",
			funcName:     "返却",
			wantFallback: false,
			wantEscapes:  true,
			wantNumeric:  false,
		},
		{
			// 変数で名前が渡る場合は静的に特定できないので全体抑制
			name:         "array_map_by_variable_name",
			code:         def + "N=「返却」\nA=Nを[「abc」]に配列マップ\nAを表示\n",
			funcName:     "返却",
			wantFallback: true,
			wantEscapes:  true,
			wantNumeric:  false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			registry := stdlib.NewRegistry()
			tree, err := parser.ParseSource(c.code, "main.nako3", registry.FuncList())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			prog, err := compiler.Compile(tree, "main.nako3", registry)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			env, err := analyzeEnvFor(prog, nil)
			if err != nil {
				t.Fatalf("analyzeEnvFor: %v", err)
			}
			if env.dynamicFuncs != c.wantFallback {
				t.Fatalf("dynamicFuncs (全体抑制) = %v, want %v", env.dynamicFuncs, c.wantFallback)
			}
			info := analyze(prog, env)

			fi := -1
			for i := range prog.Funcs {
				if prog.Funcs[i].Name == c.funcName || strings.HasSuffix(prog.Funcs[i].Name, "__"+c.funcName) {
					fi = i
				}
			}
			if fi < 0 {
				t.Fatalf("関数 %q が見つからない: %v", c.funcName, prog.Funcs)
			}
			if len(prog.Funcs[fi].Params) != 1 {
				t.Fatalf("関数 %q の引数 = %v", c.funcName, prog.Funcs[fi].Params)
			}
			slot := prog.Funcs[fi].Params[0].Slot
			if got := info.localIsNumber(fi, slot); got != c.wantNumeric {
				t.Errorf("引数の数値推論 = %v, want %v", got, c.wantNumeric)
			}
			if info.escapes[fi] != c.wantEscapes {
				t.Errorf("escapes[%s] = %v, want %v", c.funcName, info.escapes[fi], c.wantEscapes)
			}
			// 主処理は FindFunc の対象外なので、全体抑制でも
			// escapes にしない (大域変数の昇格を止めないため)
			if info.escapes[prog.Main] {
				t.Errorf("主処理が escapes になっている")
			}
		})
	}
}

// TestDynamicFuncGranularMixed は #432 の中心ケース: 動的呼び出しを
// 受ける関数と、静的数値呼び出しのみの関数が混在するプログラムで、
// 動的対象だけが抑制され、静的のみの関数は特殊化が維持されることを
// 固定する。細分化を全体抑制へ戻す変異ではこのテストが落ちる。
func TestDynamicFuncGranularMixed(t *testing.T) {
	code := "●(Aの)返却とは\n  Aで戻る\nここまで\n" +
		"●(Aの)表示用とは\n  Aで戻る\nここまで\n" +
		"●(Nの)二倍とは\n  (N*2)で戻る\nここまで\n" +
		"A=「返却」を[「abc」]に配列マップ\nAを表示\n" +
		"「表示用」を実行\n" +
		"(21の二倍)を表示\n"
	env, info, prog, funcs := compileForAnalyze(t, code, nil)
	mainIdx := prog.Main
	if env.dynamicFuncs {
		t.Fatal("リテラル指定のみなのに全体抑制が立っている")
	}
	for _, name := range []string{"返却", "表示用"} {
		idxs := funcIndexesBySuffix(funcs, name)
		if len(idxs) != 1 {
			t.Fatalf("関数 %q が %d 件ある", name, len(idxs))
		}
		if !env.dynamicTargets[idxs[0]] || !info.escapes[idxs[0]] {
			t.Errorf("動的対象 %q が escapes になっていない (targets=%v)", name, env.dynamicTargets)
		}
	}
	idxs := funcIndexesBySuffix(funcs, "二倍")
	if len(idxs) != 1 {
		t.Fatalf("関数 二倍 が %d 件ある", len(idxs))
	}
	fi := idxs[0]
	if env.dynamicTargets[fi] || info.escapes[fi] {
		t.Error("静的呼び出しのみの『二倍』まで抑制されている")
	}
	slot := prog.Funcs[fi].Params[0].Slot
	if !info.localIsNumber(fi, slot) {
		t.Error("静的呼び出しのみの『二倍』の引数の数値特殊化が維持されていない")
	}
	if info.escapes[mainIdx] {
		t.Error("主処理が escapes になっている")
	}
}

// TestDynamicFuncGranularTargets は、プラグイン経由を含む各種の動的命令で、
// 文字列リテラル指定なら対象だけが特定されることを固定する。
func TestDynamicFuncGranularTargets(t *testing.T) {
	def := "●(Aの)返却とは\n  Aで戻る\nここまで\n"
	cases := []struct {
		name     string
		code     string
		plugins  []string
		target   string // "" なら対象なしが期待値
		fallback bool
	}{
		{name: "timer_by_name", code: def + "「返却」を1秒後\n", target: "返却"},
		{name: "array_filter_by_name", code: def + "A=「返却」で[「abc」]を配列フィルタ\nAを表示\n", target: "返却"},
		{name: "array_custom_sort_by_name", code: def + "A=「返却」で[3,1]を配列カスタムソート\nAを表示\n", target: "返却"},
		{name: "measure_by_name", code: def + "(「返却」の実行時間計測)を表示\n", target: "返却"},
		{name: "nodelib_stdin", code: "●(Sで)取得CBとは\n  Sを表示\nここまで\n「取得CB」を標準入力取得時\n", plugins: []string{"nodelib"}, target: "取得CB"},
		{name: "nodelib_file_delete", code: "●完了とは\n  「完了」と表示\nここまで\n「完了」で「/tmp/x432」をファイル削除時\n", plugins: []string{"nodelib"}, target: "完了"},
		{name: "nodelib_ajax_onerror_store", code: "●(Eの)失敗処理とは\n  Eを表示\nここまで\n「失敗処理」のAJAX失敗時\n", plugins: []string{"nodelib"}, target: "失敗処理"},
		{name: "sqlite_exec_callback", code: "●後処理とは\n  「完了」と表示\nここまで\n「:memory:」をSQLITE3開く\n「後処理」に「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行時\n", plugins: []string{"sqlitelib"}, target: "後処理"},
		{name: "exec_in_loop", code: def + "Iを1から3まで繰り返す\n  「返却」を実行\nここまで\n", target: "返却"},
		{name: "exec_in_try", code: def + "エラー監視\n  「返却」を実行\nエラーならば\n  「失敗」と表示\nここまで\n", target: "返却"},
		// 関数値を直接渡す形は OpMakeFunc が既に escapes を立てるので、
		// 細分化側の対象にも全体抑制にもならない
		{name: "anon_func_value", code: "B=[1,2]\nA=(関数(V)\n(V*2)で戻る\nここまで)をBに配列マップ\nAを表示\n", target: ""},
		// 文字列でない定数は toFunc の対象にならないので対象なし
		{name: "number_as_callback", code: def + "A=123を[1]に配列マップ\nAを表示\n", target: ""},
		// 定数同士の連結はコンパイラが定数へ畳み込むため、結果として
		// 文字列リテラルと同じ扱いになる (畳み込みは compiler の実測)
		{name: "const_folded_concat_name", code: def + "A=(「返」&「却」)を[「abc」]に配列マップ\nAを表示\n", target: "返却"},
		// 名前がどの関数にも一致しなければ、動的呼び出し自体が起きない
		{name: "unknown_name", code: def + "A=「存在しない関数」を[「abc」]に配列マップ\nAを表示\n", target: ""},
		// ONERROR の消費専用命令だけでは対象は生まれない (保存側も直接
		// ストアも無いため)。URL 取得そのものは実行しない (解析のみ)
		{name: "onerror_consumer_only", code: "A=「http://example.invalid/」からAJAXテキスト取得\nAを表示\n", plugins: []string{"nodelib"}, target: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, _, _, funcs := compileForAnalyze(t, c.code, c.plugins)
			if env.dynamicFuncs != c.fallback {
				t.Fatalf("全体抑制 = %v, want %v", env.dynamicFuncs, c.fallback)
			}
			var got []string
			for fi := range env.dynamicTargets {
				got = append(got, funcs[fi])
			}
			if c.target == "" {
				if len(got) != 0 {
					t.Fatalf("対象 = %v, want なし", got)
				}
				return
			}
			idxs := funcIndexesBySuffix(funcs, c.target)
			if len(idxs) != 1 || len(got) != 1 || !env.dynamicTargets[idxs[0]] {
				t.Fatalf("対象 = %v, want %q だけ", got, c.target)
			}
		})
	}
}

// TestDynamicFuncGranularFallbacks は、静的に特定できない動的呼び出しが
// あれば全体抑制 (dynamicFuncs) へ倒れることを固定する。ここが緩むと
// #335 の誤数値化が再発する。
func TestDynamicFuncGranularFallbacks(t *testing.T) {
	def := "●(Aの)返却とは\n  Aで戻る\nここまで\n"
	cases := []struct {
		name    string
		code    string
		plugins []string
	}{
		{name: "global_variable_name", code: def + "N=「返却」\nA=Nを[「abc」]に配列マップ\nAを表示\n"},
		{name: "local_variable_name", code: "●テストとは\n  N=「返却」\n  A=Nを[「abc」]に配列マップ\n  Aを表示\nここまで\n" + def + "テスト\n"},
		{name: "concatenated_name", code: def + "X=「却」\nA=(「返」&X)を[「abc」]に配列マップ\nAを表示\n"},
		{name: "returned_name", code: def + "●名前取得とは\n  「返却」で戻る\nここまで\nA=(名前取得)を[「abc」]に配列マップ\nAを表示\n"},
		{name: "func_value_in_variable", code: "F=関数(V)\n(V*2)で戻る\nここまで\nB=[1,2,3]\nA=FをBに配列マップ\nAを表示\n"},
		{name: "hatena_pipeline", code: "[\"表示\"]をハテナ関数設定\n?? \"abc\"\n"},
		// 『JSオブジェクト取得』は同名の大域値・定数を先に引くので、
		// レジストリ定数と同名のリテラルは対象を特定したことにしない
		{name: "js_get_const_name_conflict", code: def + "F=「PI」のJSオブジェクト取得\nFを表示\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, _, _, _ := compileForAnalyze(t, c.code, c.plugins)
			if !env.dynamicFuncs {
				t.Fatal("静的に特定できないのに全体抑制へフォールバックしていない")
			}
		})
	}
}

// TestDynamicFuncDuplicateNames は、同名の関数が複数ある場合に候補を
// 全部対象にする (VM の FindFunc が返す最初の一致を含むスーパーセット)
// ことを固定する。どれか1つだけに絞ると、列挙順の解釈違いで実際に
// 呼ばれる関数を取りこぼしうる。
// 単一ソースでは同名関数は関数表で1件に潰れる (定義順で後勝ち、#433
// で実測) ため、複数候補が起こるのはモジュール (ファイル) をまたいで
// 接頭辞違いの同名関数が並ぶ場合だけである。ここでは IR を直接組んで
// その形を再現する。
func TestDynamicFuncDuplicateNames(t *testing.T) {
	registry := stdlib.NewRegistry()
	execEntry, ok := registry.Lookup("実行")
	if !ok {
		t.Fatal("『実行』がレジストリにない")
	}
	prog := &ir.Program{
		Version: ir.CurrentVersion,
		Consts:  []ir.Const{{Kind: ir.ConstString, Str: "挨拶"}},
		Funcs: []ir.Func{
			{Name: "a__挨拶", Code: []ir.Inst{{Op: ir.OpReturn}}},
			{Name: "b__挨拶", Code: []ir.Inst{{Op: ir.OpReturn}}},
			{Name: "main", Code: []ir.Inst{
				{Op: ir.OpLoadConst, A: 0},
				{Op: ir.OpCallStd, A: int32(execEntry.ID), B: 1},
				{Op: ir.OpPop},
				{Op: ir.OpReturn},
			}},
		},
		Main: 2,
	}
	env, err := analyzeEnvFor(prog, nil)
	if err != nil {
		t.Fatalf("analyzeEnvFor: %v", err)
	}
	if env.dynamicFuncs {
		t.Fatal("リテラル指定なのに全体抑制が立っている")
	}
	if !env.dynamicTargets[0] || !env.dynamicTargets[1] || len(env.dynamicTargets) != 2 {
		t.Fatalf("dynamicTargets = %v, want 同名候補の2件とも", env.dynamicTargets)
	}
	info := analyze(prog, env)
	if !info.escapes[0] || !info.escapes[1] {
		t.Errorf("escapes = %v, want 同名候補の2件とも", info.escapes)
	}
	if info.escapes[prog.Main] {
		t.Error("主処理が escapes になっている")
	}
}

// assertPluginMatchesVM はプラグイン込みでコンパイルしたプログラムを
// VM と gogen 生成バイナリの両方で実行し、出力を突き合わせる
// (TestPluginRegistry の汎用版)。stdin は両方へ同じ内容を渡す。
func assertPluginMatchesVM(t *testing.T, name, code string, plugins []string, stdin string) {
	t.Run(name, func(t *testing.T) {
		if testing.Short() {
			t.Skip("go build/run per case; skipped under -short")
		}
		registry, err := BuildRegistry(plugins)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		prog, err := compiler.Compile(tree, "main.nako3", registry)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		src, err := Generate(prog, Options{Plugins: plugins})
		if err != nil {
			t.Fatalf("generate: %v", err)
		}

		var wantBuf strings.Builder
		wantHost := vm.NewCUIHost(&wantBuf, strings.NewReader(stdin), nil)
		if err := vm.RunWithHostAndRegistry(code, "main.nako3", registry, wantHost); err != nil {
			t.Fatalf("vm: %v", err)
		}
		want := wantBuf.String()

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
			t.Fatal(err)
		}
		writeGoMod(t, dir)
		tidyModule(t, dir, src)
		run := exec.Command("go", "run", ".")
		run.Dir = dir
		run.Stdin = strings.NewReader(stdin)
		out, err := run.CombinedOutput()
		if err != nil {
			t.Fatalf("go run: %v\n%s\n--- source ---\n%s", err, out, src)
		}
		if got := string(out); got != want {
			t.Fatalf("出力が一致しない\nVM  : %q\ngogen: %q", want, got)
		}
	})
}

// TestDynamicFuncPluginEndToEnd は #432 のチェック項目: nodelib /
// sqlitelib 経由の動的命令でも数値化抑止が働くことの VM 突合。
// コールバック引数が文字列 (標準入力の行・SELECT の結果配列) なのに
// 数値だと推論されると、表示が NaN になったり添字アクセスが壊れたり
// して VM と一致しなくなる。混在させる静的数値関数の出力も見る。
func TestDynamicFuncPluginEndToEnd(t *testing.T) {
	assertPluginMatchesVM(t, "nodelib_stdin_callback_by_name", `
●(Nの)二倍とは
  (N*2)で戻る
ここまで
●(Sで)取得CBとは
  「取得:{S}」と表示
ここまで
「取得CB」を標準入力取得時
(21の二倍)を表示
`, []string{"nodelib"}, "入力行テスト\n")

	assertPluginMatchesVM(t, "sqlitelib_callbacks_by_name", `
●(Nの)二倍とは
  (N*2)で戻る
ここまで
「:memory:」をSQLITE3開く
●完了CBとは
  「作成完了」と表示
ここまで
「完了CB」に「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行時
「INSERT INTO t VALUES(?)」を[「値」]でSQLITE3実行
●(ROWSで)取得CBとは
  ROWS[0]["v"]を表示
ここまで
「取得CB」に「SELECT v FROM t」を[]でSQLITE3取得時
(21の二倍)を表示
SQLITE3閉じる
`, []string{"sqlitelib"}, "")

	// 変数で名前を渡す形 (全体抑制へフォールバックする経路) でも
	// 出力が VM と一致すること。
	assertPluginMatchesVM(t, "nodelib_stdin_callback_by_variable", `
●(Sで)取得CBとは
  「取得:{S}」と表示
ここまで
N=「取得CB」
Nを標準入力取得時
`, []string{"nodelib"}, "入力行テスト\n")
}

// handProg は、名前 name の関数を2つ (a__/b__ 接頭辞) と、main の命令列
// からなる IR を直接組む。ソースからは作りにくい形 (合流・未到達・
// 引数なし呼び出し等) の解析テスト用。
func handProg(t *testing.T, consts []ir.Const, globals []string, mainCode []ir.Inst) *ir.Program {
	t.Helper()
	return &ir.Program{
		Version: ir.CurrentVersion,
		Consts:  consts,
		Globals: globals,
		Funcs: []ir.Func{
			{Name: "a__対象", Code: []ir.Inst{{Op: ir.OpReturn}}},
			{Name: "b__対象", Code: []ir.Inst{{Op: ir.OpReturn}}},
			{Name: "main", Code: mainCode},
		},
		Main: 2,
	}
}

func callStdInst(t *testing.T, registry *stdlib.Registry, name string, argc int32) ir.Inst {
	t.Helper()
	e, ok := registry.Lookup(name)
	if !ok {
		t.Fatalf("『%s』がレジストリにない", name)
	}
	return ir.Inst{Op: ir.OpCallStd, A: int32(e.ID), B: argc}
}

// TestDynamicFuncStackOriginsIR は stackOrigins の合流・複製・未到達・
// 引数なしの扱いを、IR を直接組んで固定する。
func TestDynamicFuncStackOriginsIR(t *testing.T) {
	registry := stdlib.NewRegistry()
	strA := ir.Const{Kind: ir.ConstString, Str: "対象"}
	strB := ir.Const{Kind: ir.ConstString, Str: "別人"}
	num := ir.Const{Kind: ir.ConstNumber, Num: 1}

	t.Run("merge_same_const", func(t *testing.T) {
		// 2つの経路が同じ文字列定数を積んで合流する → 対象が特定できる
		prog := handProg(t, []ir.Const{num, strA}, nil, []ir.Inst{
			{Op: ir.OpLoadConst, A: 0},        // 条件値
			{Op: ir.OpJumpIfFalse, A: 5},      // false なら pc5
			{Op: ir.OpLoadConst, A: 1},        // 「対象」
			{Op: ir.OpJump, A: 6},             // 合流へ
			{Op: ir.OpReturn},                 // 未到達 (詰め物)
			{Op: ir.OpLoadConst, A: 1},        // 「対象」
			callStdInst(t, registry, "実行", 1), // pc6
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if env.dynamicFuncs {
			t.Fatal("同じ定数同士の合流なのに全体抑制になっている")
		}
		if !env.dynamicTargets[0] || !env.dynamicTargets[1] || len(env.dynamicTargets) != 2 {
			t.Fatalf("dynamicTargets = %v, want 対象の2件", env.dynamicTargets)
		}
	})

	t.Run("merge_different_consts", func(t *testing.T) {
		// 別の文字列定数同士が合流する → 生成元は不明になり全体抑制
		prog := handProg(t, []ir.Const{num, strA, strB}, nil, []ir.Inst{
			{Op: ir.OpLoadConst, A: 0},
			{Op: ir.OpJumpIfFalse, A: 5},
			{Op: ir.OpLoadConst, A: 1},
			{Op: ir.OpJump, A: 6},
			{Op: ir.OpReturn},
			{Op: ir.OpLoadConst, A: 2},
			callStdInst(t, registry, "実行", 1),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !env.dynamicFuncs {
			t.Fatal("別の定数が合流するのに全体抑制へフォールバックしていない")
		}
	})

	t.Run("dup_keeps_origin", func(t *testing.T) {
		// Dup で複製して片方を捨てても、残った段の生成元は保たれる
		prog := handProg(t, []ir.Const{strA}, nil, []ir.Inst{
			{Op: ir.OpLoadConst, A: 0},
			{Op: ir.OpDup},
			{Op: ir.OpPop},
			callStdInst(t, registry, "実行", 1),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if env.dynamicFuncs || len(env.dynamicTargets) != 2 {
			t.Fatalf("dynamicFuncs=%v targets=%v, want 対象2件のみ特定", env.dynamicFuncs, env.dynamicTargets)
		}
	})

	t.Run("unreachable_call_ignored", func(t *testing.T) {
		// Return の後ろ (未到達) の動的呼び出しは実行されないので無視する
		prog := handProg(t, []ir.Const{strA}, nil, []ir.Inst{
			{Op: ir.OpReturn},
			{Op: ir.OpLoadConst, A: 0},
			callStdInst(t, registry, "実行", 1),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if env.dynamicFuncs || len(env.dynamicTargets) != 0 {
			t.Fatalf("未到達コードに反応している: dynamicFuncs=%v targets=%v", env.dynamicFuncs, env.dynamicTargets)
		}
	})

	t.Run("no_callback_arg", func(t *testing.T) {
		// 引数なしで動的命令を呼ぶ形は想定外なので全体抑制 (fail-closed)
		prog := handProg(t, nil, nil, []ir.Inst{
			callStdInst(t, registry, "実行", 0),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !env.dynamicFuncs {
			t.Fatal("引数なし呼び出しなのに全体抑制になっていない")
		}
	})

	t.Run("js_get_global_suffix_conflict", func(t *testing.T) {
		// 『JSオブジェクト取得』は同名 (接尾辞一致) の大域変数を先に
		// 引くため、リテラルでも対象を特定したことにしない
		prog := handProg(t, []ir.Const{strA}, []string{"main__対象"}, []ir.Inst{
			{Op: ir.OpLoadConst, A: 0},
			callStdInst(t, registry, "JSオブジェクト取得", 1),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !env.dynamicFuncs {
			t.Fatal("同名大域変数があるのに全体抑制になっていない")
		}
	})

	t.Run("js_get_number_const", func(t *testing.T) {
		// 『JSオブジェクト取得』は引数を無条件に ToString するので、
		// 数値定数でも全体抑制 (他の命令の数値定数は対象なしでよい
		// ことは TestDynamicFuncGranularTargets/number_as_callback が固定)
		prog := handProg(t, []ir.Const{num}, nil, []ir.Inst{
			{Op: ir.OpLoadConst, A: 0},
			callStdInst(t, registry, "JSオブジェクト取得", 1),
			{Op: ir.OpPop},
			{Op: ir.OpReturn},
		})
		env, err := analyzeEnvFor(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !env.dynamicFuncs {
			t.Fatal("JSオブジェクト取得へ数値定数を渡すのに全体抑制になっていない")
		}
	})
}

// TestDynamicFuncOnErrorDirectStore は、AJAX:ONERROR 消費命令がある
// プログラムで、その大域への直接ストアがあると全体抑制になることを
// 固定する。現行の文法では『AJAX:ONERROR』を
// 変数として書けないため、IR を直接組んで確認する。
func TestDynamicFuncOnErrorDirectStore(t *testing.T) {
	registry, err := BuildRegistry([]string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	getInst := callStdInst(t, registry, "AJAXテキスト取得", 1)
	urlConst := []ir.Const{{Kind: ir.ConstString, Str: "http://example.invalid/"}}

	// ストアなし: 消費命令だけでは全体抑制にならない
	prog := handProg(t, urlConst, nil, []ir.Inst{
		{Op: ir.OpLoadConst, A: 0},
		getInst,
		{Op: ir.OpPop},
		{Op: ir.OpReturn},
	})
	env, err := analyzeEnvFor(prog, []string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	if env.dynamicFuncs {
		t.Fatal("消費命令だけなのに全体抑制になっている")
	}

	// 直接ストアあり: 保存元を静的に追えないので全体抑制
	prog2 := handProg(t, urlConst, []string{"main__AJAX:ONERROR"}, []ir.Inst{
		{Op: ir.OpLoadConst, A: 0},
		{Op: ir.OpStoreGlobal, A: 0},
		{Op: ir.OpLoadConst, A: 0},
		getInst,
		{Op: ir.OpPop},
		{Op: ir.OpReturn},
	})
	env2, err := analyzeEnvFor(prog2, []string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	if !env2.dynamicFuncs {
		t.Fatal("AJAX:ONERROR への直接ストアがあるのに全体抑制になっていない")
	}
}
