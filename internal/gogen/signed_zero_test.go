package gogen

import "testing"

// TestGeneratedSignedZero は #339 の受け入れ条件: 生成バイナリでも
// +0 と -0 が区別され、VM と結果が一致すること。生成コードが -0 を
// Go のリテラル "-0" で書くと、コンパイル時定数として +0 に
// 畳み込まれてしまうため、実際にビルドして実行して確かめる。
func TestGeneratedSignedZero(t *testing.T) {
	assertMatchesVM(t, "signed_zero_plus_first", `
A=0
B=-0
(1/A)を表示
(1/B)を表示
`)
	assertMatchesVM(t, "signed_zero_minus_first", `
A=-0
B=0
(1/A)を表示
(1/B)を表示
`)
	assertMatchesVM(t, "signed_zero_in_expression", `
(1/-0)を表示
`)
	// 融合命令の operandFloat 経路: 変数と -0 の二項演算でも符号が残ること。
	// X * -0 は -0 になるので、その逆数は -Infinity になる。
	assertMatchesVM(t, "signed_zero_fused_binary", `
X=5
Y=X*-0
(1/Y)を表示
`)
}
