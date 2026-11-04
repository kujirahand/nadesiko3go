package stdlib_test

import (
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 『参照』の範囲辞書 ({先頭, 末尾}) の回帰テストである (#342)。
//
// 期待値は本家 TypeScript 版の実測値。文字列は String.prototype.substring、
// 配列は Array.prototype.slice の規約に従い、末尾は含む指定になる。
// 修正前は負の末尾で slice bounds out of range の panic になった。

func refRange(t *testing.T, container value.Value, first, last float64) value.Value {
	t.Helper()
	e, ok := stdlib.NewRegistry().Lookup("参照")
	if !ok || e.Fn == nil {
		t.Fatal("参照 が登録されていない")
	}
	span := value.NewDict()
	span.Set("先頭", value.Number(first))
	span.Set("末尾", value.Number(last))
	got, err := e.Fn(newContext(), []value.Value{container, value.DictValue(span)})
	if err != nil {
		t.Fatalf("参照 がエラー: %v", err)
	}
	return got
}

func numArray(nums ...float64) value.Value {
	items := make([]value.Value, len(nums))
	for i, n := range nums {
		items[i] = value.Number(n)
	}
	return value.ArrayValue(value.NewArray(items...))
}

// 配列を "0,1,2" の形に整える。
func joinNums(t *testing.T, v value.Value) string {
	t.Helper()
	arr, ok := v.Array()
	if !ok {
		t.Fatalf("結果が配列でない: %v", v.Kind())
	}
	parts := make([]string, arr.Len())
	for i := range parts {
		parts[i] = strconv.FormatFloat(value.ToNumber(arr.Get(i)), 'f', -1, 64)
	}
	return strings.Join(parts, ",")
}

func TestReferenceRangeJSSemantics(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name        string
		first, last float64
		str, arr    string
	}{
		{"0と-2", 0, -2, "", "0,1,2"},
		{"0と2", 0, 2, "abc", "0,1,2"},
		{"-1と1", -1, 1, "ab", ""},
		{"逆転2と0", 2, 0, "b", ""},
		{"逆転1と0", 1, 0, "", ""},
		{"0と0", 0, 0, "a", "0"},
		{"広すぎる範囲", -99, 99, "abc", "0,1,2,3"},
		{"範囲外", 99, 100, "", ""},
		{"先頭NaN", nan, 1, "ab", "0,1"},
		{"末尾NaN", 0, nan, "", ""},
		{"末尾+Inf", 0, inf, "abc", "0,1,2,3"},
		{"両方+Inf", inf, inf, "", ""},
		{"先頭-Inf", math.Inf(-1), 1, "ab", "0,1"},
		{"小数", 1.7, 2.9, "bc", "1,2"},
		{"負の範囲", -3, -1, "", ""},
		{"巨大値", 0, 1e19, "abc", "0,1,2,3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := value.ToString(refRange(t, value.String("abc"), tt.first, tt.last)); got != tt.str {
				t.Errorf("文字列 = %q, want %q", got, tt.str)
			}
			if got := joinNums(t, refRange(t, numArray(0, 1, 2, 3), tt.first, tt.last)); got != tt.arr {
				t.Errorf("配列 = %q, want %q", got, tt.arr)
			}
		})
	}
}

func TestReferenceRangeEmptyInput(t *testing.T) {
	if got := value.ToString(refRange(t, value.String(""), 0, 10)); got != "" {
		t.Errorf("空文字列 = %q, want 空", got)
	}
	if got := joinNums(t, refRange(t, numArray(), 0, 10)); got != "" {
		t.Errorf("空配列 = %q, want 空", got)
	}
}

func TestReferenceRangeJapaneseRunes(t *testing.T) {
	if got := value.ToString(refRange(t, value.String("あいうえお"), 0, 2)); got != "あいう" {
		t.Errorf("= %q, want あいう", got)
	}
}

// 巨大な末尾を指定しても、実配列長に見合う量しか確保しないことを確かめる。
// 修正前は {0, 4000000} で約128MBを確保していた。
func TestReferenceRangeHugeEndAllocation(t *testing.T) {
	arr := numArray(1)
	if got := joinNums(t, refRange(t, arr, 0, 4000000)); got != "1" {
		t.Fatalf("結果 = %q, want 1", got)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := refRange(t, arr, 0, 4000000)
	runtime.ReadMemStats(&after)
	if s := joinNums(t, got); s != "1" {
		t.Errorf("結果 = %q, want 1", s)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 4<<20 {
		t.Errorf("確保量が大きすぎる: %d バイト", grew)
	}
}

// 『配列切取』の範囲辞書も JS の splice 規約に揃える。
func TestArrayCutRangeJSSemantics(t *testing.T) {
	e, ok := stdlib.NewRegistry().Lookup("配列切取")
	if !ok || e.Fn == nil {
		t.Skip("配列切取 が登録されていない")
	}
	tests := []struct {
		name        string
		first, last float64
		cut, rest   string
	}{
		{"末尾+Inf", 0, math.Inf(1), "0,1,2,3", ""},
		{"負の先頭", -1, 1, "3", "0,1,2"},
		{"通常", 1, 2, "1,2", "0,3"},
		{"逆転", 2, 0, "", "0,1,2,3"},
		{"末尾NaN", 0, math.NaN(), "", "0,1,2,3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := numArray(0, 1, 2, 3)
			span := value.NewDict()
			span.Set("先頭", value.Number(tt.first))
			span.Set("末尾", value.Number(tt.last))
			got, err := e.Fn(newContext(), []value.Value{arr, value.DictValue(span)})
			if err != nil {
				t.Fatal(err)
			}
			if s := joinNums(t, got); s != tt.cut {
				t.Errorf("切取結果 = %q, want %q", s, tt.cut)
			}
			if s := joinNums(t, arr); s != tt.rest {
				t.Errorf("残り = %q, want %q", s, tt.rest)
			}
		})
	}
}
