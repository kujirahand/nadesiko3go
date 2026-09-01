package vm_test

import (
	"errors"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Expected values come from running the same source through the TypeScript
// implementation. The compat fixtures pin the same behaviour case by case;
// these tests keep the failure readable when something breaks.
func run(t *testing.T, code string) string {
	t.Helper()
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err != nil {
		t.Fatalf("%q: %v", code, err)
	}
	return r.Log
}

func TestLiterals(t *testing.T) {
	tests := []struct{ code, want string }{
		{"3を表示", "3"},
		{"3.10を表示", "3.1"},
		{"-5を表示", "-5"},
		{"0xFFを表示", "255"},
		{"1_000を表示", "1000"},
		{"「abc」を表示", "abc"},
		{"『abc{30}abc』を表示", "abc{30}abc"}, // 二重鍵括弧は展開しない
		{"A=30\n「abc{A}abc」を表示", "abc30abc"},
		{"A=30\n「{A+1}」を表示", "31"},
		{"オンを表示", "true"},
		{"オフを表示", "false"},
		{"A=空\nAを表示", ""},
		{"A=未定義\nAを表示", "undefined"},
		{"A=[1,2,3]\nAを表示", "1,2,3"},
		{"A=[[1,2],[3,4]]\nAを表示", "1,2,3,4"},
		{`A={"x":1,"y":2}` + "\nAを表示", "[object Object]"},
		{"A=1;B=2;(A+B)を表示", "3"},
		{"1を表示\n2を表示\n3を表示", "1\n2\n3"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestOperators(t *testing.T) {
	tests := []struct{ code, want string }{
		{"(1+2)を表示", "3"},
		{"(7/2)を表示", "3.5"},
		{"(7%3)を表示", "1"},
		{"(-7%3)を表示", "-1"},
		{"(2^10)を表示", "1024"},
		{"(1+2*3)を表示", "7"},
		{"((1+2)*3)を表示", "9"},
		{"A=5\n(-A)を表示", "-5"},
		{`("a"&"b")を表示`, "ab"},
		{`("a"&1)を表示`, "a1"},
		{"(1=1)を表示", "true"},
		{"(1≠2)を表示", "true"},
		{"(2≦2)を表示", "true"},
		{"(オン&&オン)を表示", "true"},
		{"(オン||オフ)を表示", "true"},
		{"(12と10のAND)を表示", "8"},
		{"(1 << 4)を表示", "16"},
		{"1に2を足して3を掛けて表示", "9"}, // 連文
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestTypeConversion pins the coercions that make nadesiko's 『+』 different
// from JavaScript's.
func TestTypeConversion(t *testing.T) {
	tests := []struct{ code, want string }{
		{`(1+"2")を表示`, "3"},
		{`("1"+"2")を表示`, "3"}, // 文字列連結にはならない
		{`(1+"あ")を表示`, "NaN"},
		{`("5"-"2")を表示`, "3"},
		{`(0="0")を表示`, "true"},
		{"(オン+オン)を表示", "NaN"},
		{"A=1/0\nAを表示", "Infinity"},
		{"A=0/0\nAを表示", "NaN"},
		{"A=0.1+0.2\nAを表示", "0.30000000000000004"},
		{"A=9007199254740993\nAを表示", "9007199254740992"},
		{`A="12.7"を整数変換` + "\nAを表示", "12"},
		{`A=[1,2]を文字列変換` + "\nAを表示", "1,2"},
		{`A=""を真偽判定` + "\nAを表示", "偽"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestControlFlow(t *testing.T) {
	tests := []struct {
		name, code, want string
	}{
		{"もし-真", "もし1=1ならば\n「T」と表示\n違えば\n「F」と表示\nここまで", "T"},
		{"もし-偽", "もし1=2ならば\n「T」と表示\n違えば\n「F」と表示\nここまで", "F"},
		{"もし-一行", "もし1=1ならば「T」と表示", "T"},
		{"もし-入れ子", "A=5\nもしA>3ならば\nもしA>4ならば\n「大」と表示\nここまで\nここまで", "大"},
		{"回", "3回\n「x」と表示\nここまで", "x\nx\nx"},
		{"回-回数変数", "3回\n回数を表示\nここまで", "1\n2\n3"},
		{"繰り返す", "Nを1から3まで繰り返す\nNを表示\nここまで", "1\n2\n3"},
		// ループ変数を書かない『繰り返す』は『対象』を設定しない
		{"繰り返す-対象", "1から3まで繰り返す\n対象を表示\nここまで", ""},
		{"反復-配列", "[10,20]を反復\n対象を表示\nここまで", "10\n20"},
		{"反復-対象キー", `{"x":1,"y":2}を反復` + "\n対象キーを表示\nここまで", "x\ny"},
		{"間", "A=0\nA<3の間\nA=A+1\nAを表示\nここまで", "1\n2\n3"},
		{"抜ける", "5回\nもし回数=3ならば抜ける\n回数を表示\nここまで", "1\n2"},
		{"続ける", "3回\nもし回数=2ならば続ける\n回数を表示\nここまで", "1\n3"},
		{"条件分岐", "A=2\nAで条件分岐\n1ならば\n「一」と表示\nここまで\n2ならば\n「二」と表示\nここまで\n違えば\n「他」と表示\nここまで\nここまで", "二"},
		{"条件分岐-違えば", "A=9\nAで条件分岐\n1ならば\n「一」と表示\nここまで\n違えば\n「他」と表示\nここまで\nここまで", "他"},
		{"ループ-入れ子", "2回\nI=回数\n2回\n「{I}-{回数}」と表示\nここまで\nここまで", "1-1\n1-2\n2-1\n2-2"},
		{"戻る-ループ内", "●テストとは\n3回\nもし回数=2ならば「{回数}」で戻る\nここまで\nここまで\nテストを表示", "2"},
		{"インデント構文", "!インデント構文\nもし1=1ならば\n    「T」と表示\n", "T"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestRuntimeError(t *testing.T) {
	_, err := vm.RunSource("「わざとエラー」でエラー発生", "main.nako3", nil)
	if err == nil {
		t.Fatal("エラーになるはずが成功した")
	}
	var nakoErr *errs.NakoError
	if !errors.As(err, &nakoErr) {
		t.Fatalf("エラー型 = %T, want *errs.NakoError", err)
	}
	if nakoErr.Kind != errs.Runtime {
		t.Errorf("Kind = %v, want errs.Runtime", nakoErr.Kind)
	}
	want := "[実行時エラー]main.nako3(1行目): わざとエラー"
	if got := nakoErr.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestVars checks the variable read-back the compat runner uses.
func TestVars(t *testing.T) {
	r, err := vm.RunSource("A=[1,2]\nB=3", "main.nako3", []string{"A", "B", "C"})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.ToString(r.Vars["A"]); got != "1,2" {
		t.Errorf("A = %q, want \"1,2\"", got)
	}
	if got := value.ToString(r.Vars["B"]); got != "3" {
		t.Errorf("B = %q, want \"3\"", got)
	}
	// 存在しない変数は undefined
	if r.Vars["C"].Kind() != value.KindUndefined {
		t.Errorf("C = %v, want undefined", r.Vars["C"].Kind())
	}
}

// TestLogTrimsTrailingWhitespace pins that printing empty strings leaves an
// empty log, because 『表示ログ』 is read back with its tail trimmed.
func TestLogTrimsTrailingWhitespace(t *testing.T) {
	if got := run(t, "「」を表示\n「」を表示"); got != "" {
		t.Errorf("log = %q, want \"\"", got)
	}
	if got := run(t, "「a」を表示\n「」を表示"); got != "a" {
		t.Errorf("log = %q, want \"a\"", got)
	}
}
