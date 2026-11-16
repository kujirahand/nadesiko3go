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
