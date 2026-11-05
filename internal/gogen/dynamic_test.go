package gogen

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
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
	for _, name := range dynamicFuncCommands {
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

// TestDynamicFuncParams は、名前で取り出されうる関数の引数を数値だと
// 推論しないことを、生成バイナリを実行せず解析結果で直接固定する。
// 静的な数値呼び出しだけの関数は従来どおり特殊化されることも見る。
func TestDynamicFuncParams(t *testing.T) {
	def := "●(Aの)返却とは\n  Aで戻る\nここまで\n"
	cases := []struct {
		name        string
		code        string
		funcName    string
		wantDynamic bool
		wantNumeric bool
	}{
		{
			name:        "static_numeric_call_only",
			code:        "●(Nの)二倍とは\n  (N*2)で戻る\nここまで\n(21の二倍)を表示\n",
			funcName:    "二倍",
			wantDynamic: false,
			wantNumeric: true,
		},
		{
			name:        "js_object_get_and_call_value",
			code:        def + "F=「返却」のJSオブジェクト取得\n(F(「abc」))を表示\n",
			funcName:    "返却",
			wantDynamic: true,
			wantNumeric: false,
		},
		{
			name:        "array_map_by_name",
			code:        def + "A=「返却」を[「abc」]に配列マップ\nAを表示\n",
			funcName:    "返却",
			wantDynamic: true,
			wantNumeric: false,
		},
		{
			name:        "await_by_name",
			code:        def + "(「返却」を「xyz」でAWAIT実行)を表示\n",
			funcName:    "返却",
			wantDynamic: true,
			wantNumeric: false,
		},
		{
			name:        "exec_by_name",
			code:        def + "「返却」を実行して表示\n",
			funcName:    "返却",
			wantDynamic: true,
			wantNumeric: false,
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
			if env.dynamicFuncs != c.wantDynamic {
				t.Fatalf("dynamicFuncs = %v, want %v", env.dynamicFuncs, c.wantDynamic)
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
			if info.escapes[fi] != c.wantDynamic {
				t.Errorf("escapes[%s] = %v, want %v", c.funcName, info.escapes[fi], c.wantDynamic)
			}
			// 主処理は FindFunc の対象外なので、動的経路があっても
			// escapes にしない (大域変数の昇格を止めないため)
			if info.escapes[prog.Main] {
				t.Errorf("主処理が escapes になっている")
			}
		})
	}
}
