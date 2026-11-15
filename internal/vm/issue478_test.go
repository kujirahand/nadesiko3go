package vm_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// C言語風関数呼び出しの直後に演算子が続く式 (#478 / 本家 #2671)
func TestIssue478CFuncOperator(t *testing.T) {
	curry := "curry_add = 関数(あ)\n  inner = 関数(い)\n    それはあ + い\n  ここまで\n  innerを戻す\nここまで\n"
	tests := []struct{ code, want string }{
		// 再現
		{"INT(3.7)-1を表示", "2"},
		{"MAX(1,5)-1を表示", "4"}, // ABS(-5)-1 も実バイナリでは 4 を確認済み (ABS は mathlib のため RunSource の標準構成では使えない)
		{"INT(8)/2を表示", "4"},
		{"INT(3.7)+1を表示", "4"},
		// 演算子各種
		{"INT(3.7)*2を表示", "6"},
		{"INT(3.7)%2を表示", "1"},
		{"INT(3.7)^2を表示", "9"},
		{"INT(3.7)==3を表示", "true"},
		{"INT(3.7)-1*2を表示", "1"},
		// 連続呼び出し
		{"INT(1)+INT(2)を表示", "3"},
		// 既存引数あり
		{"10にINT(3.7)+1を足して表示", "14"},
		{"AにINT(3.7)+1を代入;Aを表示", "4"},
		{"10とINT(3.7)+1の最大値を表示", "10"},
		// 計算結果の後ろに別の引数が続く語順 (レビュー中-1)
		{"INT(3.7)+1をAに代入;Aを表示", "4"},
		{"INT(3.7)-1と5の最小値を表示", "2"},
		{"INT(3.7)-1回「a」を表示", "a\na"},
		// 関数内
		{"●三\n　INT(3.7)-1を戻す\nここまで\n三を表示", "2"},
		// ユーザー定義のC風関数
		{"●三倍(A)とは\n　それはA*3\nここまで\n三倍(2)-1を表示", "5"},
		// チェーン呼び出し
		{curry + "curry_add(1)(2)-1を表示", "2"},
		{curry + "curry_add(1)(2)+1を表示", "4"},
		// 回帰
		{"INT(3.7)を表示", "3"},
		{"A=INT(3.7)-1;Aを表示", "2"},
		{"(INT(3.7)-1)を表示", "2"},
		{"もしINT(3.7)-1=2ならば「OK」を表示", "OK"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// 消費されない計算式だけの文は、通常の計算文 (例: 3-1) と同じく文法エラーになる
func TestIssue478BareCalcIsError(t *testing.T) {
	for _, code := range []string{"INT(3.7)-1", "INT(3.7)-を表示"} {
		if _, err := vm.RunSource(code, "main.nako3", nil); err == nil {
			t.Errorf("%q は文法エラーになるべき", code)
		}
	}
}
