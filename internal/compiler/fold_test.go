package compiler_test

// 定数畳み込みと定数伝播のテスト。同じ式が「何を出すか」(run) と
// 「どんなIRになるか」(mainOps) の両方を見る。前者だけだと、伝播が
// 効かなくなっても気づけない。

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func run(t *testing.T, code string) string {
	t.Helper()
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err != nil {
		t.Fatalf("%q: %v", code, err)
	}
	return r.Log
}

// mainOps counts how many times each opcode appears in main.
func mainOps(t *testing.T, code string) map[ir.Op]int {
	t.Helper()
	prog, err := vm.CompileProgram(code, "main.nako3")
	if err != nil {
		t.Fatalf("%q: %v", code, err)
	}
	got := map[ir.Op]int{}
	for _, inst := range prog.Funcs[prog.Main].Code {
		got[inst.Op]++
	}
	return got
}

// TestPropagateConst checks that a 『定数』 with a known value is substituted
// into later expressions, leaving no operator to run.
func TestPropagateConst(t *testing.T) {
	tests := []struct{ code, want string }{
		{"Nとは定数=10\n(N*N+1)を表示", "101"},
		{"Nとは定数=10\nMとは定数=N*2\nMを表示", "20"},
		{"Sとは定数=「あ」\n(S&S)を表示", "ああ"},
		{"Nとは定数=10\n(N>3)を表示", "true"},
		{"Nを10に定める\n(N-4)を表示", "6"},
		// 定数どうしなら、関数の中でも同じように潰れる
		{"●テスト\n\tKとは定数=3\n\t(K*K)を表示\nここまで\nテスト", "9"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
		ops := mainOps(t, tt.code)
		if n := ops[ir.OpBinary] + ops[ir.OpBinaryAt]; n != 0 {
			t.Errorf("%q: 演算が %d 個残っている。畳み込まれていない", tt.code, n)
		}
	}
}

// TestPropagateConstSkipsBranch keeps the fold away from a declaration that
// may never run. 『定数』 declared in a branch that is not taken leaves the
// name undefined, and substituting the value would change that.
func TestPropagateConstSkipsBranch(t *testing.T) {
	code := "もし、1=2ならば\n\tBとは定数=99\nここまで\nBを表示"
	if got, want := run(t, code), "undefined"; got != want {
		t.Errorf("%q = %q, want %q", code, got, want)
	}

	// 実行される枝の中の宣言も、伝播はしないが値は正しく読める
	code = "Nとは定数=2\nもし、1=1ならば\n\tCとは定数=N*2\nここまで\nCを表示"
	if got, want := run(t, code), "4"; got != want {
		t.Errorf("%q = %q, want %q", code, got, want)
	}
	// 『Cを表示』は変数を読む。値に置き換わっていたら OpLoadGlobal は消える
	if n := mainOps(t, code)[ir.OpLoadGlobal]; n == 0 {
		t.Error("枝の中の定数を伝播させてしまった (C の読み出しが消えている)")
	}
}

// TestPropagateConstNotAcrossFuncs guards the case a user function is called
// from a line above its definition: the module 『定数』 has not been assigned
// yet when the body runs, so the body must read the variable, not the value.
func TestPropagateConstNotAcrossFuncs(t *testing.T) {
	code := "テスト\nNとは定数=5\n●テスト\n\tNを表示\nここまで"
	if got, want := run(t, code), "undefined"; got != want {
		t.Errorf("%q = %q, want %q", code, got, want)
	}
}

// TestFoldKeepsRuntimeMeaning pins the values a fold must not change: NaN and
// ±Inf cannot go in the constant pool (IR は直列化可能であること・§6), and a
// very long string is cheaper to build at run time than to carry in the pool.
func TestFoldKeepsRuntimeMeaning(t *testing.T) {
	tests := []struct{ code, want string }{
		{"(0/0)を表示", "NaN"},
		{"(1/0)を表示", "Infinity"},
		{"(-1/0)を表示", "-Infinity"},
		{"(0*-1)を表示", "0"},
		{"(1&「あ」)を表示", "1あ"},
		{"(「1」+1)を表示", "2"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}

	// 畳み込むと大きくなる文字列は実行時に任せる。定数プールが膨らまないこと。
	long := "Sとは定数=「" + strings.Repeat("あ", 3000) + "」\n(S&S)を表示"
	ops := mainOps(t, long)
	if n := ops[ir.OpBinary] + ops[ir.OpBinaryAt]; n != 1 {
		t.Errorf("長い文字列を畳み込んでしまった (演算=%d, want 1)", n)
	}
}
