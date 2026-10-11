package vm_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// 値を返すもし式 (本家 #2591)
func TestIfExpression(t *testing.T) {
	tests := []struct{ code, want string }{
		{"家猫=1\n猫愛嬌は、もし家猫なら5、違えば2。\n猫愛嬌を表示", "5"},
		{"家猫=0\n猫愛嬌は、もし家猫なら5、違えば2。\n猫愛嬌を表示", "2"},
		{"家猫=0\nA=もし家猫なら\n  2に3を足したもの\n違えば\n  2\nここまで\nAを表示", "2"},
		{"家猫=1\nA=もし家猫なら\n  2に3を足したもの\n違えば\n  2\nここまで\nAを表示", "5"},
		{"((もし1なら5、違えば2)+10)を表示", "15"},
		{"家猫=1\nA=もし家猫なら:\n  5\n違えば:\n  2\nAを表示", "5"},
		{"A=もし1なら5\n違えば\n  2\nここまで\nAを表示\n「後続」を表示", "5\n後続"},
		{"A=もし0なら\n  5\n違えば2\nここまで\nAを表示\n「後続」を表示", "2\n後続"},
		{"A=もし1なら\n  もし0なら1、違えば7\n違えば3\nここまで\nAを表示", "7"},
		{"A=もし1なら\n  X=3\n  X*2\n違えば0\nここまで\nXを表示\nAを表示", "3\n6"},
		{"●F\n  A=もし1なら5、違えば2\n  Aを表示\nここまで\nFを実行", "5"},
		{"(もし0なら1、違えば2)と(もし1なら3、違えば4)を足して表示", "5"},
		{"もし1ならば\n  「文」を表示\nここまで\nA=もし0なら1、違えば2\nAを表示", "文\n2"},
		{"●F(Aを)\n  Aを戻す\nここまで\nA=F((もし1なら5、違えば2))\nAを表示", "5"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestIfExpressionOnlyRunsSelectedBranch(t *testing.T) {
	code := "A=もし1なら\n  「選択」を表示\n  1\n違えば\n  「未選択」を表示\n  2\nここまで\nAを表示"
	if got := run(t, code); got != "選択\n1" {
		t.Errorf("got %q", got)
	}
}

func TestIfExpressionSyntaxErrors(t *testing.T) {
	tests := []struct{ code, want string }{
		{"A=もし1なら5\nAを表示", "違えば"},
		{"A=もし1なら「x」を表示、違えば2", "末尾には値を返す式"},
		{"A=もし1なら\n  5\n違えば2\nAを表示", "ここまで"},
	}
	for _, tt := range tests {
		_, err := vm.RunSource(tt.code, "main.nako3", nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error = %v, want %q", tt.code, err, tt.want)
		}
	}
}
