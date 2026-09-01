package value_test

import (
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Expected values are what JavaScript's String(n) produces, since the compat
// fixtures pin the output of 表示.
// Goは定数式をコンパイル時に正確に畳み込むので、浮動小数点数の誤差を
// 見るケースは変数を経由させる。
var pointOne, pointTwo = 0.1, 0.2

func TestNumberToString(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"}, // JSは -0 も "0"
		{1, "1"},
		{-1, "-1"},
		{1.5, "1.5"},
		{0.1, "0.1"},
		{pointOne + pointTwo, "0.30000000000000004"},
		{100, "100"},
		{255, "255"},
		{1500, "1500"},
		{1234.5678, "1234.5678"},
		{-0.0001, "-0.0001"},
		{1.0 / 3.0, "0.3333333333333333"},
		// 指数表記に切り替わる境界
		{1e20, "100000000000000000000"},
		{1e21, "1e+21"},
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		{123456789012345678901234, "1.2345678901234569e+23"},
		{9007199254740993, "9007199254740992"}, // float64に丸められる
		{5e-324, "5e-324"},
		{math.MaxFloat64, "1.7976931348623157e+308"},
		{math.NaN(), "NaN"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
	}
	for _, tt := range tests {
		if got := value.NumberToString(tt.in); got != tt.want {
			t.Errorf("NumberToString(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
