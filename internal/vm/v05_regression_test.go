package vm_test

import "testing"

// 実際の入口（CLI と同じ VM → stdlib 経路）で範囲超過の数値文字列を通す。
// 期待値は JavaScript で同じ式を評価した結果。
func TestV05ArithmeticOverflow(t *testing.T) {
	tests := []struct{ code, want string }{
		{"「1e309」に0を足して表示", "Infinity"},
		{"「-1e309」に0を足して表示", "-Infinity"},
		{"「1e309」から1を引いて表示", "Infinity"},
		{"「1e309」を実数変換して表示", "Infinity"},
		{"「-1e309」を実数変換して表示", "-Infinity"},
		{"「1e-400」を実数変換して表示", "0"},
		{"「0x10000000000000000」から1を引いて表示", "18446744073709552000"},
		{"「0x10」から1を引いて表示", "15"},
		{"「12abc」に0を足して表示", "12"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}
