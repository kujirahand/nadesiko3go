package stdlib

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// string_count_test.go が持つ表（NaN・±無限大・上限超え・負数・端数）を補う
// 回帰テストである。差し当たり次を確かめる (本家 #2480 / gonako #142)。
//   - 上限いっぱい(maxCount)でも必ず打ち切って結果を返すこと
//   - 桁数に数値でない値を渡したときも同じ文面で止まること
//   - int に収まらない巨大な負数が処理系依存の値にならないこと
func padCall(t *testing.T, cmd string, v, width value.Value) (string, error) {
	t.Helper()
	e, ok := NewRegistry().Lookup(cmd)
	if !ok || e.Fn == nil {
		t.Fatalf("命令 %s がレジストリにない", cmd)
	}
	// 実装は ctx を使わないので nil で構わない
	got, err := e.Fn(nil, []value.Value{v, width})
	if err != nil {
		return "", err
	}
	return value.ToString(got), nil
}

func TestPadLeftBoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		cmd   string
		v     value.Value
		width value.Value
		want  string
	}{
		{"ゼロ埋-桁数0", "ゼロ埋", value.String("5"), value.Number(0), "5"},
		{"ゼロ埋-負の端数", "ゼロ埋", value.String("5"), value.Number(-3.9), "5"},
		{"ゼロ埋-小数は切り捨て", "ゼロ埋", value.String("5"), value.Number(3.9), "005"},
		{"空白埋-小数は切り捨て", "空白埋", value.String("a"), value.Number(3.9), "  a"},
		{"空白埋-既にある空白は保つ", "空白埋", value.String("010"), value.Number(4), " 010"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := padCall(t, tt.cmd, tt.v, tt.width)
			if err != nil {
				t.Fatalf("%s がエラー: %v", tt.name, err)
			}
			if got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestPadLeftRejectsNonNumericWidth(t *testing.T) {
	finite := "の桁数には有限の整数を指定してください。"
	tests := []struct {
		name  string
		cmd   string
		width value.Value
	}{
		{"ゼロ埋-数値でない文字列", "ゼロ埋", value.String("あ")},
		{"ゼロ埋-未定義値", "ゼロ埋", value.Undefined()},
		{"空白埋-負の無限大", "空白埋", value.Number(math.Inf(-1))},
		{"空白埋-数値でない文字列", "空白埋", value.String("あ")},
		{"空白埋-未定義値", "空白埋", value.Undefined()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := padCall(t, tt.cmd, value.String("5"), tt.width)
			if err == nil {
				t.Fatalf("%s がエラーにならなかった", tt.name)
			}
			if want := "『" + tt.cmd + "』" + finite; err.Error() != want {
				t.Errorf("%s のエラー = %q, want %q", tt.name, err.Error(), want)
			}
		})
	}
}

// 上限いっぱいでも停止して結果を返すことと、int に収まらない負数が
// 処理系依存の巨大な値に化けないことを確かめる。
func TestPadLeftDoesNotOverflow(t *testing.T) {
	got, err := padCall(t, "ゼロ埋", value.String("5"), value.Number(maxCount))
	if err != nil {
		t.Fatalf("ゼロ埋(上限ちょうど) がエラー: %v", err)
	}
	if n := utf8.RuneCountInString(got); n != maxCount {
		t.Errorf("ゼロ埋(上限ちょうど) の桁数 = %d, want %d", n, maxCount)
	}
	if tail := strings.TrimLeft(got, "0"); tail != "5" {
		t.Errorf("ゼロ埋(上限ちょうど) の末尾 = %q, want %q", tail, "5")
	}

	for _, width := range []float64{-1e21, -1e300} {
		got, err := padCall(t, "空白埋", value.String("a"), value.Number(width))
		if err != nil {
			t.Fatalf("空白埋(%g) がエラー: %v", width, err)
		}
		if got != "a" {
			t.Errorf("空白埋(%g) = %q, want %q (int変換の桁溢れ)", width, got, "a")
		}
	}
}
