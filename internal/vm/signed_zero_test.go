package vm_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestSignedZeroReciprocals は #339 の受け入れ条件: +0 と -0 の登場順に
// かかわらず、それぞれの逆数が Infinity / -Infinity になること。
// 以前は定数プールで先に現れたゼロへ統合され、両方とも同じ符号になった。
func TestSignedZeroReciprocals(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"プラスが先", "A=0\nB=-0\n(1/A)を表示\n(1/B)を表示", "Infinity\n-Infinity"},
		{"マイナスが先", "A=-0\nB=0\n(1/A)を表示\n(1/B)を表示", "-Infinity\nInfinity"},
		{"マイナスだけ", "B=-0\n(1/B)を表示", "-Infinity"},
		{"式の中のマイナスゼロ", "(1/-0)を表示", "-Infinity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestSignedZeroConstsStayDistinct は、定数プールに +0 と -0 が
// ビット列の違う別定数として残ることを固定する。
func TestSignedZeroConstsStayDistinct(t *testing.T) {
	for _, code := range []string{
		"A=0\nB=-0\n(1/A)を表示\n(1/B)を表示",
		"A=-0\nB=0\n(1/A)を表示\n(1/B)を表示",
	} {
		prog, err := vm.CompileProgram(code, "main.nako3")
		if err != nil {
			t.Fatalf("%q: %v", code, err)
		}
		bits := map[uint64]bool{}
		for _, k := range prog.Consts {
			if k.Kind == ir.ConstNumber {
				bits[math.Float64bits(k.Num)] = true
			}
		}
		if !bits[0] {
			t.Errorf("%q: 定数プールに +0 がない", code)
		}
		if !bits[math.Float64bits(math.Copysign(0, -1))] {
			t.Errorf("%q: 定数プールに -0 がない", code)
		}
	}
}
