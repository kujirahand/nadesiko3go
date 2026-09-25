package stdlib_test

import (
	"math"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 『配列連番作成』の範囲上限と端点処理の回帰テストである。
//
// 期待値は本家 TypeScript 版 core/test/array_test.mjs の #2472 と同じ入力。
// 修正前は整数化した端点まで無制限に生成していたため、巨大な範囲で
// メモリを使い尽くし、安全整数の外側では終了しなくなっていた。
func rangeSeq(t *testing.T, from, to value.Value) (value.Value, error) {
	t.Helper()
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("配列連番作成")
	if !ok || e.Fn == nil {
		t.Fatal("配列連番作成 が登録されていない")
	}
	return e.Fn(newContext(), []value.Value{from, to})
}

// 生成された配列を "1,2,3" の形に整える。要素が Number であることも確かめる。
func rangeSeqString(t *testing.T, from, to value.Value) (string, error) {
	t.Helper()
	got, err := rangeSeq(t, from, to)
	if err != nil {
		return "", err
	}
	arr, ok := got.Array()
	if !ok {
		t.Fatalf("結果が配列でない: %v", got.Kind())
	}
	parts := make([]string, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		item := arr.Get(i)
		if item.Kind() != value.KindNumber {
			t.Fatalf("%d番目の要素が数値でない: %v", i, item.Kind())
		}
		parts = append(parts, value.ToString(item))
	}
	return strings.Join(parts, ","), nil
}

func TestArrayRange(t *testing.T) {
	tests := []struct {
		name string
		from value.Value
		to   value.Value
		want string
	}{
		{"整数", value.Number(1), value.Number(3), "1,2,3"},
		{"開始が負", value.Number(0 - 2), value.Number(1), "-2,-1,0,1"},
		{"同じ値", value.Number(7), value.Number(7), "7"},
		// 順番が逆なら空配列（本家と同じ）
		{"逆順", value.Number(5), value.Number(1), ""},
		// #2472 数値文字列も数値として扱う
		{"数値文字列", value.String("1"), value.String("3"), "1,2,3"},
		{"数値文字列-小数", value.String("1.5"), value.String("2.5"), "1.5,2.5"},
		{"数値文字列-前後の空白", value.String(" 1 "), value.String("3"), "1,2,3"},
		{"数値文字列-指数表記", value.String("1e2"), value.String("102"), "100,101,102"},
		// #2472 小数の範囲は従来どおり生成できる
		{"小数の範囲", value.Number(1.5), value.Number(3.5), "1.5,2.5,3.5"},
		{"小数の範囲-1件", value.Number(0.5), value.Number(0.5), "0.5"},
		{"小数の範囲-端点がはみ出す", value.Number(1.5), value.Number(3), "1.5,2.5"},
		// #2472 安全整数の最大値ちょうどは許可される
		{"MAX_SAFE_INTEGER", value.Number(9007199254740991), value.Number(9007199254740991), "9007199254740991"},
		{"MAX_SAFE_INTEGER-負", value.Number(0 - 9007199254740991), value.Number(0 - 9007199254740991), "-9007199254740991"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rangeSeqString(t, tt.from, tt.to)
			if err != nil {
				t.Fatalf("%s がエラー: %v", tt.name, err)
			}
			if got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// 有限でない値・数値として読めない値は本家と同じ文面で止まる。
func TestArrayRangeRejectsNonFiniteEndpoint(t *testing.T) {
	const finite = "『配列連番作成』には有限の数値を指定してください。"
	// 空文字列や空白だけの文字列は Number() なら0になるが、本家は
	// 有限な数値として扱わない (#2472)
	tests := []struct {
		name string
		from value.Value
		to   value.Value
	}{
		{"空文字列", value.String(""), value.String("")},
		{"空白だけ", value.String("   "), value.Number(3)},
		{"数値でない文字列", value.String("あ"), value.Number(3)},
		{"NaN", value.Number(math.NaN()), value.Number(3)},
		{"正の無限大", value.Number(1), value.Number(math.Inf(1))},
		{"負の無限大", value.Number(math.Inf(-1)), value.Number(1)},
		{"未定義値", value.Undefined(), value.Number(1)},
		{"配列", value.ArrayValue(value.NewArray(value.Number(1))), value.Number(3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rangeSeq(t, tt.from, tt.to)
			if err == nil {
				t.Fatalf("%s がエラーにならなかった", tt.name)
			}
			if err.Error() != finite {
				t.Errorf("%s のエラー = %q, want %q", tt.name, err.Error(), finite)
			}
		})
	}
}

// 絶対値が2の53乗-1を超えると1ずつ加算しても値が変わらず、
// ループが進展しなくなるため先に弾く (#2472)。
func TestArrayRangeRejectsUnsafeInteger(t *testing.T) {
	const unsafe = "『配列連番作成』には絶対値が2の53乗-1(9007199254740991)以下の数値を指定してください。"
	tests := []struct {
		name string
		from value.Value
		to   value.Value
	}{
		{"2の53乗", value.Number(9007199254740992), value.Number(9007199254740992)},
		{"2の53乗+1", value.Number(9007199254740993), value.Number(9007199254740993)},
		{"負の2の53乗", value.Number(0 - 9007199254740992), value.Number(0 - 9007199254740992)},
		{"巨大な値", value.Number(1), value.Number(1e20)},
		{"巨大な負の値", value.Number(-1e300), value.Number(-1e300)},
		{"数値文字列の巨大値", value.Number(1), value.String("1e20")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rangeSeq(t, tt.from, tt.to)
			if err == nil {
				t.Fatalf("%s がエラーにならなかった", tt.name)
			}
			if err.Error() != unsafe {
				t.Errorf("%s のエラー = %q, want %q", tt.name, err.Error(), unsafe)
			}
		})
	}
}

// 要素数の上限は式ではなく生成しながら数えるので、端点の小数部がずれて
// いても過大評価せず、2の52乗付近の刻み幅のずれも見逃さない (#2472)。
func TestArrayRangeLimitsElementCount(t *testing.T) {
	const tooMany = "『配列連番作成』で生成される配列の要素数が多すぎます。"

	// 0.5から1000000まで(1ずつ加算)で要素数はちょうど100万。
	// to-from+1(=1000000.5)で判定すると誤って上限超過になる
	got, err := rangeSeq(t, value.Number(0.5), value.Number(1000000))
	if err != nil {
		t.Fatalf("0.5から1000000まで がエラー: %v", err)
	}
	arr, ok := got.Array()
	if !ok {
		t.Fatalf("結果が配列でない: %v", got.Kind())
	}
	if arr.Len() != 1000000 {
		t.Errorf("0.5から1000000まで の要素数 = %d, want %d", arr.Len(), 1000000)
	}

	// 上限を1件超える範囲はエラー
	if _, err := rangeSeq(t, value.Number(0), value.Number(1000000)); err == nil {
		t.Error("0から1000000まで がエラーにならなかった")
	} else if err.Error() != tooMany {
		t.Errorf("0から1000000まで のエラー = %q, want %q", err.Error(), tooMany)
	}

	// 2の52乗付近では加算の刻み幅が変わり、実際の反復回数が式と食い違う
	if _, err := rangeSeq(t, value.Number(4503599627370495.5), value.Number(4503599628370495)); err == nil {
		t.Error("2の52乗付近 がエラーにならなかった")
	} else if err.Error() != tooMany {
		t.Errorf("2の52乗付近 のエラー = %q, want %q", err.Error(), tooMany)
	}

	// 広すぎる範囲でも上限で打ち切られ、広い範囲を作り切る前に戻る
	if _, err := rangeSeq(t, value.Number(0), value.Number(9007199254740991)); err == nil {
		t.Error("0から安全整数の最大値まで がエラーにならなかった")
	} else if err.Error() != tooMany {
		t.Errorf("巨大な範囲 のエラー = %q, want %q", err.Error(), tooMany)
	}
}
