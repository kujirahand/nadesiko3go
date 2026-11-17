package vm_test

import "testing"

// 辞書オブジェクトの省略記法『{A}』の回帰テスト。
// 回帰: 省略記法の値が getVarNameRef を通らず、グローバル変数 A が
// main__A に解決されずに undefined になっていた。
func TestDictShorthandVariableReference(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{
			"トップレベルのグローバル変数",
			"A = 10\nD = {A}\nD[\"A\"]を表示",
			"10",
		},
		{
			"関数内からのグローバル変数",
			"A = 10\n●hogeとは\n  D = {A}\n  D[\"A\"]を表示\nここまで\nhoge()",
			"10",
		},
		{
			"関数内のローカル変数",
			"●hogeとは\n  B = 20\n  E = {B}\n  E[\"B\"]を表示\nここまで\nhoge()",
			"20",
		},
		{
			"同名グローバル変数を隠すローカル変数",
			"A = 10\n●hogeとは\n  A = 20\n  D = {A}\n  D[\"A\"]を表示\nここまで\nhoge()",
			"20",
		},
		{
			"関数の引数",
			"●hoge(V)とは\n  D = {V}\n  D[\"V\"]を表示\nここまで\nhoge(30)",
			"30",
		},
		{
			"複数の省略キーと明示キーの混在",
			"A = 1\nB = 2\nD = {A, B, \"C\": 3}\nD[\"A\"]を表示\nD[\"B\"]を表示\nD[\"C\"]を表示",
			"1\n2\n3",
		},
		{
			"未定義変数は通常参照と同じ",
			"D = {Z}\nD[\"Z\"]を表示\nZを表示",
			"undefined\nundefined",
		},
		{
			"文字列リテラルの省略",
			"D = {\"A\"}\nD[\"A\"]を表示",
			"A",
		},
		{
			"数値リテラルの省略",
			"D = {1}\nD[1]を表示",
			"1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}
