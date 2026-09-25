package stdlib_test

import (
	"math"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 本家 TypeScript 版 #2480（ゼロ埋・空白埋）と #2481（リフレイン）の修正に
// 合わせた回帰テストである。非有限値や極端に大きな桁数・回数で処理が
// 止まらないことと、文面が本家と同じことを確かめる。
//
// 期待値は core/test/plugin_system_test.mjs の msng リストと同じ入力。
func TestPadWidthAndRepeatCount(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	call := func(cmd string, args ...value.Value) (string, error) {
		e, ok := r.Lookup(cmd)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が登録されていない", cmd)
		}
		got, err := e.Fn(ctx, args)
		if err != nil {
			return "", err
		}
		return value.ToString(got), nil
	}

	num := func(f float64) value.Value { return value.Number(f) }

	tests := []struct {
		cmd  string
		args []value.Value
		want string
	}{
		// --- 通常入力は従来どおり ---
		{"ゼロ埋", []value.Value{value.Number(5), num(3)}, "005"},
		{"ゼロ埋", []value.Value{value.String("12345"), num(3)}, "12345"},
		{"ゼロ埋", []value.Value{value.String("𩸽"), num(4)}, "000𩸽"},  // サロゲートペアは1桁
		{"ゼロ埋", []value.Value{value.String("5"), num(-3)}, "5"},    // 負数は埋めない
		{"ゼロ埋", []value.Value{value.String("5"), num(-1e21)}, "5"}, // 巨大な負数でもint変換が壊れない
		{"空白埋", []value.Value{value.String("a"), num(3)}, "  a"},
		{"空白埋", []value.Value{value.String("12345"), num(3)}, "12345"},
		{"空白埋", []value.Value{value.String("𩸽"), num(4)}, "   𩸽"},
		{"リフレイン", []value.Value{value.String("ab"), num(3)}, "ababab"},
		{"リフレイン", []value.Value{value.String("x"), num(0)}, ""},
		// #2481 端数は切り捨て
		{"リフレイン", []value.Value{value.String("x"), num(0.5)}, ""},
		{"リフレイン", []value.Value{value.String("x"), num(2.5)}, "xx"},
		// #2481 上限ちょうどは許可
		{"リフレイン", []value.Value{value.String(""), num(1000000)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.cmd+"/通常", func(t *testing.T) {
			got, err := call(tt.cmd, tt.args...)
			if err != nil {
				t.Fatalf("%s%v: %v", tt.cmd, tt.args, err)
			}
			if got != tt.want {
				t.Errorf("%s%v = %q, want %q", tt.cmd, tt.args, got, tt.want)
			}
		})
	}

	errTests := []struct {
		name string
		cmd  string
		args []value.Value
		want string
	}{
		{"ゼロ埋/NaN", "ゼロ埋", []value.Value{value.String("5"), num(math.NaN())}, "『ゼロ埋』の桁数には有限の整数を指定してください。"},
		{"ゼロ埋/Infinity", "ゼロ埋", []value.Value{value.String("5"), num(math.Inf(1))}, "『ゼロ埋』の桁数には有限の整数を指定してください。"},
		{"ゼロ埋/上限超え", "ゼロ埋", []value.Value{value.String("5"), num(1000001)}, "『ゼロ埋』の桁数が大きすぎます。"},
		{"ゼロ埋/指数表記", "ゼロ埋", []value.Value{value.String("5"), num(1e21)}, "『ゼロ埋』の桁数が大きすぎます。"},
		{"空白埋/NaN", "空白埋", []value.Value{value.String("5"), num(math.NaN())}, "『空白埋』の桁数には有限の整数を指定してください。"},
		{"空白埋/Infinity", "空白埋", []value.Value{value.String("5"), num(math.Inf(1))}, "『空白埋』の桁数には有限の整数を指定してください。"},
		{"空白埋/上限超え", "空白埋", []value.Value{value.String("5"), num(1000001)}, "『空白埋』の桁数が大きすぎます。"},
		{"空白埋/指数表記", "空白埋", []value.Value{value.String("5"), num(1e21)}, "『空白埋』の桁数が大きすぎます。"},
		{"リフレイン/NaN", "リフレイン", []value.Value{value.String("x"), num(math.NaN())}, "『リフレイン』の回数には有限の整数を指定してください。"},
		{"リフレイン/Infinity", "リフレイン", []value.Value{value.String("x"), num(math.Inf(1))}, "『リフレイン』の回数には有限の整数を指定してください。"},
		{"リフレイン/負数", "リフレイン", []value.Value{value.String("x"), num(-1)}, "『リフレイン』の回数には0以上の整数を指定してください。"},
		{"リフレイン/負の端数", "リフレイン", []value.Value{value.String("x"), num(-0.5)}, "『リフレイン』の回数には0以上の整数を指定してください。"},
		{"リフレイン/上限超え", "リフレイン", []value.Value{value.String("x"), num(1000001)}, "『リフレイン』の回数が大きすぎます。"},
		{"リフレイン/指数表記", "リフレイン", []value.Value{value.String("x"), num(1e21)}, "『リフレイン』の回数が大きすぎます。"},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := call(tt.cmd, tt.args...)
			if err == nil {
				t.Fatalf("%s%v がエラーにならなかった", tt.cmd, tt.args)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("エラー文面 = %q, want %q", got, tt.want)
			}
		})
	}
}

// 結果が極端に大きくなるリフレインは、確保する前にエラーにする (#2481)。
func TestRefrainResultTooLarge(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()
	e, ok := r.Lookup("リフレイン")
	if !ok || e.Fn == nil {
		t.Fatal("リフレイン が登録されていない")
	}
	base := strings.Repeat("x", 700) // 本家のテストと同じ 700 文字
	if _, err := e.Fn(ctx, []value.Value{value.String(base), value.Number(1000000)}); err == nil {
		t.Fatal("結果が大きすぎるのに成功した")
	} else if want := "『リフレイン』の結果が大きすぎます。"; err.Error() != want {
		t.Errorf("エラー文面 = %q, want %q", err.Error(), want)
	}
}
