package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 『数列判定』の回帰テストである。
//
// 期待値は本家 TypeScript 版 core/test/plugin_system_test.mjs の
// 「数列か判定」と同じ入力（#1423・#2143・#2483 と小数部省略の指数表記）。
// 判定の正規表現は core/src/plugin_system_string.mts の checkerRE に合わせてある。
func TestIsNumberSequence(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()
	e, ok := r.Lookup("数列判定")
	if !ok || e.Fn == nil {
		t.Fatal("数列判定 が登録されていない")
	}

	isNumSeq := func(s string) string {
		t.Helper()
		got, err := e.Fn(ctx, []value.Value{value.String(s)})
		if err != nil {
			t.Fatalf("数列判定(%q): %v", s, err)
		}
		return value.ToString(got)
	}

	tests := []struct {
		in   string
		want string
	}{
		{"12345", "true"},
		{"あいうえお", "false"},
		// #1423 符号・小数点・指数表記
		{"-12345", "true"},
		{"123-45", "false"},
		{"12.345", "true"},
		{"1.23.45", "false"},
		{"1.234E-5", "true"},
		// #2143 空文字列はfalse
		{"", "false"},
		// #2483 符号のみはfalse。整数仮数・小数仮数の指数表記はtrue
		{"+", "false"},
		{"-", "false"},
		{"＋", "false"},
		{"－", "false"},
		{".", "false"},
		{"1e3", "true"},
		{"1.0e3", "true"},
		{"+1.5e+10", "true"},
		{"-1.5e-10", "true"},
		// 小数部を省略した指数表記（本家 d2dac011）
		{"123.e1", "true"},
		{"1.e1", "true"},
		// 小数部の省略自体も受理する（本家と同じ。"1." は数値として読める）
		{"123.", "true"},
		{"1.", "true"},
		{"-.", "false"},
		{"+.", "false"},
		// 指数部が空のものはfalse
		{"1.5e", "false"},
		{"123.e", "false"},
		{"1e", "false"},
		{"e1", "false"},
		// 符号は1つだけ
		{"--1", "false"},
		{"+-1", "false"},
		// 先頭ドットの小数は従来どおり受理する
		{".5", "true"},
		{"-.5", "true"},
		// 全角数字・全角記号
		{"１２３", "true"},
		{"１.５", "true"},
		{"＋１", "true"},
		{"１e３", "true"},
		// 16進数表記は数列ではない
		{"0x1F", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isNumSeq(tt.in); got != tt.want {
				t.Errorf("数列判定(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// 数列判定に渡した値は本家と同じく文字列化して判定する。
func TestIsNumberSequenceNonString(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()
	e, ok := r.Lookup("数列判定")
	if !ok || e.Fn == nil {
		t.Fatal("数列判定 が登録されていない")
	}

	got, err := e.Fn(ctx, []value.Value{value.Number(123)})
	if err != nil {
		t.Fatal(err)
	}
	if want := "true"; value.ToString(got) != want {
		t.Errorf("数列判定(123:number) = %s, want %s", value.ToString(got), want)
	}
}
