package gogen

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// TestNonNumericLoopVarMatchesVM は、文字列の範囲を数えるループの変数が
// float64 に昇格しないことを、生成コードの実行結果で確かめる。
//
// 『Aを「あ」から「い」まで繰り返す』は、コンパイラの覗き穴最適化で
// 『それ』とループ変数への代入を OpStoreSoreAndLocal / OpStoreSoreAndGlobal
// に融合する。この命令はスタックを使わないため、ループの先頭では
// スタック深さが 0 になる。型推論がその深さで判定を打ち切ると、変数が
// 非数値のまま数値として扱われ、rt.ToNumber(「あ」) が NaN になって
// VM の出力と食い違う。
func TestNonNumericLoopVarMatchesVM(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{
			// 関数内のローカルループ (OpStoreSoreAndLocal)
			name: "local_in_function",
			code: "●テストとは\n" +
				"Aを「あ」から「い」まで繰り返す\n" +
				"  Aを表示\n" +
				"ここまで\n" +
				"ここまで\n" +
				"テスト\n",
		},
		{
			// トップレベルのグローバルループ (OpStoreSoreAndGlobal)
			name: "global_at_top_level",
			code: "Aを「あ」から「い」まで繰り返す\n" +
				"  Aを表示\n" +
				"ここまで\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("go build/run per case; skipped under -short")
			}
			const want = "あ"
			if got := strings.TrimRight(vmOutput(t, c.code), "\n"); got != want {
				t.Fatalf("VM の出力 = %q, want %q", got, want)
			}
			if got := strings.TrimRight(buildAndRun(t, c.code), "\n"); got != want {
				t.Fatalf("gogen の出力 = %q, want %q", got, want)
			}
		})
	}
}

// TestLoopVarTypeInference は、融合命令のソースと代入先の型推論を直接確かめる。
// 文字列ループの変数は数値ではなく、昇格もしない。数値ループの変数は
// これまでどおり float64 に昇格する (最適化を壊していない)。
func TestLoopVarTypeInference(t *testing.T) {
	cases := []struct {
		name        string
		code        string
		global      bool
		wantNumeric bool
	}{
		{
			name:        "local_string_loop",
			code:        "●テストとは\nAを「あ」から「い」まで繰り返す\n  Aを表示\nここまで\nここまで\nテスト\n",
			global:      false,
			wantNumeric: false,
		},
		{
			name:        "global_string_loop",
			code:        "Aを「あ」から「い」まで繰り返す\n  Aを表示\nここまで\n",
			global:      true,
			wantNumeric: false,
		},
		{
			name:        "local_numeric_loop",
			code:        "●テストとは\nAを1から3まで繰り返す\n  Aを表示\nここまで\nここまで\nテスト\n",
			global:      false,
			wantNumeric: true,
		},
		{
			name:        "global_numeric_loop",
			code:        "Aを1から3まで繰り返す\n  Aを表示\nここまで\n",
			global:      true,
			wantNumeric: true,
		},
		{
			name:        "local_numeric_arithmetic",
			code:        "●テストとは\nAを1から3まで繰り返す\n  (A+1)を表示\nここまで\nここまで\nテスト\n",
			wantNumeric: true,
		},
		{
			name:        "global_numeric_arithmetic",
			code:        "Aを1から3まで繰り返す\n  (A+1)を表示\nここまで\n",
			global:      true,
			wantNumeric: true,
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
			info := analyze(prog, env)

			found := false
			for fi := range prog.Funcs {
				for _, inst := range prog.Funcs[fi].Code {
					if c.global && inst.Op != ir.OpStoreSoreAndGlobal {
						continue
					}
					if !c.global && inst.Op != ir.OpStoreSoreAndLocal {
						continue
					}
					found = true
					slot := int(inst.B)
					var numeric, promoted bool
					if c.global {
						numeric = info.numericGlobals[slot]
						promoted = info.promotedGlobals[slot]
					} else {
						numeric = info.numericLocals[fi][slot]
						promoted = info.promoted[fi][slot]
					}
					if numeric != c.wantNumeric {
						t.Errorf("代入先スロット B=%d が数値と推論された = %v, want %v", slot, numeric, c.wantNumeric)
					}
					if promoted != c.wantNumeric {
						t.Errorf("代入先スロット B=%d が float64 に昇格した = %v, want %v", slot, promoted, c.wantNumeric)
					}
					// A は代入先がグローバルの場合もローカルスロットである。
					source := int(inst.A)
					if got := info.numericLocals[fi][source]; got != c.wantNumeric {
						t.Errorf("ソース側スロット A=%d が数値と推論された = %v, want %v", source, got, c.wantNumeric)
					}
					if got := info.promoted[fi][source]; got != c.wantNumeric {
						t.Errorf("ソース側スロット A=%d が float64 に昇格した = %v, want %v", source, got, c.wantNumeric)
					}
				}
			}
			if !found {
				t.Fatalf("融合命令 %s が見つからない", map[bool]string{true: "OpStoreSoreAndGlobal", false: "OpStoreSoreAndLocal"}[c.global])
			}
		})
	}
}

// assertNonEmptyMatchesVM は出力があるケースで、VMとの一致に加えて空出力を検出する。
func assertNonEmptyMatchesVM(t *testing.T, name, code string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if testing.Short() {
			t.Skip("go build/run per case; skipped under -short")
		}
		if strings.TrimRight(vmOutput(t, code), "\n") == "" {
			t.Fatal("VM の出力が空")
		}
		assertMatchesVM(t, "matches_vm", code)
	})
}

// TestNumericLoopVarMatchesVM は数値ループの実行結果を VM と比較する。
func TestNumericLoopVarMatchesVM(t *testing.T) {
	assertNonEmptyMatchesVM(t, "local_in_function", "●テストとは\nAを1から3まで繰り返す\n  Aを表示\nここまで\nここまで\nテスト\n")
	assertNonEmptyMatchesVM(t, "global_at_top_level", "Aを1から3まで繰り返す\n  Aを表示\nここまで\n")
}

// TestNestedLoopMatchesVM は異なる型のループ変数が互いの動作に影響しないことを確かめる。
func TestNestedLoopMatchesVM(t *testing.T) {
	body := "Aを1から2まで繰り返す\n  Bを「あ」から「い」まで繰り返す\n    Aを表示\n    Bを表示\n  ここまで\nここまで\n"
	assertNonEmptyMatchesVM(t, "local_in_function", "●テストとは\n"+body+"ここまで\nテスト\n")
	assertNonEmptyMatchesVM(t, "global_at_top_level", body)
}

// TestMixedLoopBoundsMatchesVM は開始値と終了値の型が異なる場合の動作を VM と比較する。
func TestMixedLoopBoundsMatchesVM(t *testing.T) {
	for _, c := range []struct{ name, bounds string }{
		{"numeric_start", "1から「い」"},
		{"string_start", "「あ」から3"},
	} {
		body := "Aを" + c.bounds + "まで繰り返す\n  Aを表示\nここまで\n「終了」を表示\n"
		assertNonEmptyMatchesVM(t, c.name+"_local", "●テストとは\n"+body+"ここまで\nテスト\n")
		assertNonEmptyMatchesVM(t, c.name+"_global", body)
	}
}

// TestLoopVarArithmeticMatchesVM は昇格したループ変数の算術結果を VM と比較する。
func TestLoopVarArithmeticMatchesVM(t *testing.T) {
	body := "Aを1から3まで繰り返す\n  (A+1)を表示\nここまで\n"
	assertNonEmptyMatchesVM(t, "local_in_function", "●テストとは\n"+body+"ここまで\nテスト\n")
	assertNonEmptyMatchesVM(t, "global_at_top_level", body)
}
