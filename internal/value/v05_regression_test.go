package value_test

import (
	"math"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Number() は構文として正しい数値文字列の範囲超過を Infinity にする。
// 期待値は JavaScript の Number() を node で評価して得た。
func TestV05ToNumberDecimalOverflow(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"1e309", math.Inf(1)},
		{"-1e309", math.Inf(-1)},
		{"+1e309", math.Inf(1)},
		{"-1e-400", 0},
		{"1e-400", 0},
		{"1e308", 1e308},
		// 構文不正は NaN のまま
		{"12abc", math.NaN()},
		{"inf", math.NaN()},
	}
	for _, tt := range tests {
		if got := value.ToNumber(value.String(tt.in)); !sameNumber(got, tt.want) {
			t.Errorf("ToNumber(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// 0x/0o/0b 付きの数値文字列は 64bit に収まらなくても Number() の表現へ丸める。
func TestV05ToNumberNonDecimalOverflow(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"0x10000000000000000", 1 << 64},
		{"0xFFFFFFFFFFFFFFFFFF", 1 << 72},
		{"0Xffffffffffffffff", 1<<64 - 1},
		{"0o2000000000000000000000", 1 << 64},
		{"0b1" + strings.Repeat("0", 65), 1 << 65},
		// 接頭辞の後ろが空、桁として不正、符号付きは Number() でも NaN
		{"0x", math.NaN()},
		{"0xg", math.NaN()},
		{"0o8", math.NaN()},
		{"0x-1", math.NaN()},
		{"0x+1", math.NaN()},
		{"-0x10", math.NaN()},
		{"0x10", 16},
	}
	for _, tt := range tests {
		if got := value.ToNumber(value.String(tt.in)); !sameNumber(got, tt.want) {
			t.Errorf("ToNumber(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// parseFloat は先頭の数値だけを読み、範囲超過は Infinity として返す。
func TestV05ParseFloatOverflow(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"1e309", math.Inf(1)},
		{"-1e309", math.Inf(-1)},
		{"1e309abc", math.Inf(1)},
		{"-1e-400", 0},
		{"1e-400", 0},
		{"12abc", 12},
		{"0x10", 0},
	}
	for _, tt := range tests {
		if got := value.ParseFloat(value.String(tt.in)); !sameNumber(got, tt.want) {
			t.Errorf("ParseFloat(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// 変換の代表入力。性能比較は trunk と同じ入力で取る。範囲超過だけは
// big.Int を通るので、通常入力と分けて測る。
func BenchmarkV05ToNumber(b *testing.B) {
	cases := []struct {
		name string
		in   value.Value
	}{
		{"number", value.Number(1)},
		{"decimal", value.String(" 12.5 ")},
		{"hex", value.String("0x10")},
		{"decimal_overflow", value.String("1e309")},
		{"hex_over_64bit", value.String("0x10000000000000000")},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = value.ToNumber(c.in)
			}
		})
	}
}
