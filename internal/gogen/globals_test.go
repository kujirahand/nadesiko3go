package gogen

import (
	"sort"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// TestPromotableGlobals pins down when a module variable may leave its cell
// for an ordinary Go variable. Getting this wrong does not fail to build: the
// generated code would just keep writing a copy nobody else can see, so the
// rules are worth checking directly rather than only through the end-to-end
// cases (which is also why every "must not promote" case is here).
func TestPromotableGlobals(t *testing.T) {
	cases := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "loop_counter_and_accumulator",
			code: "S=0\nIを1から3まで繰り返す\n  S=S+I\nここまで\nSを表示\n",
			want: []string{"main__I", "main__S"},
		},
		{
			// 関数から読まれる変数は、関数側からGoの変数が見えない
			name: "read_by_a_function",
			code: "S=0\n●足すとは\n  S=S+1\nここまで\n足す\nSを表示\n",
			want: nil,
		},
		{
			// システム変数は stdlib の命令が名前で読み書きする
			name: "system_variable",
			code: "表示ログ=1\n表示ログを表示\n",
			want: nil,
		},
		{
			// 名前を実行時に決めて大域変数を引く命令があると、どれも移せない
			name: "dynamic_lookup",
			code: "S=5\n(「S」のJSオブジェクト取得)を表示\n",
			want: nil,
		},
		{
			// ?? のパイプラインは命令名を実行時に解決するので、そこから
			// JSオブジェクト取得を呼べる場合もセルを残す必要がある
			name: "dynamic_lookup_through_question_pipeline",
			code: "S=5\n[\"JSオブジェクト取得\",\"表示\"]をハテナ関数設定\n?? \"S\"\n",
			want: nil,
		},
		{
			// 数値以外が入る変数は、そもそも float64 にならない
			name: "not_a_number",
			code: "S=\"あ\"\nSを表示\n",
			want: nil,
		},
		{
			// 代入より先に読まれる変数は undefined を返すことがある
			name: "read_before_assigned",
			code: "もしS==0ならば\n  Sを表示\nここまで\nS=1\n",
			want: nil,
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

			var got []string
			for slot := range info.promotedGlobals {
				got = append(got, prog.Globals[slot])
			}
			sort.Strings(got)
			if len(got) != len(c.want) {
				t.Fatalf("移した大域変数 = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("移した大域変数 = %v, want %v", got, c.want)
				}
			}
		})
	}
}
