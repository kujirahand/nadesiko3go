package stdlib_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #416 / #417: JSONエンコードを本家 JSON.stringify に合わせる。
// #416: 辞書の関数値のキーを省く (配列の中の関数は null のまま)。
// #417: 整数形式 (配列インデックス) のキーを数値昇順で先に並べ、
//       それ以外は挿入順にする。並び替えは JSON 出力時だけに適用し、
//       Dict 自身の保持順は変えない。

func encodeJSONForTest(t *testing.T, v value.Value) string {
	t.Helper()
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("JSONエンコード")
	if !ok {
		t.Fatal("JSONエンコード が見つかりません")
	}
	got, err := e.Fn(newContext(), []value.Value{v})
	if err != nil {
		t.Fatalf("JSONエンコードでエラー: %v", err)
	}
	s, _ := got.String()
	return s
}

func dictOf(pairs ...any) *value.Dict {
	d := value.NewDict()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i].(string), pairs[i+1].(value.Value))
	}
	return d
}

func num(n float64) value.Value { return value.Number(n) }

func TestJSONEncodeCompat416OmitsFuncValues(t *testing.T) {
	fn := value.FuncValue(&value.Func{ID: 1})
	tests := []struct {
		name string
		in   value.Value
		want string
	}{
		{"関数値だけの辞書は空になる", value.DictValue(dictOf("f", fn)), `{}`},
		{"関数値と通常値の混在", value.DictValue(dictOf("f", fn, "n", num(1))), `{"n":1}`},
		{"未定義と関数はともに省く", value.DictValue(dictOf("u", value.Undefined(), "f", fn, "ok", num(1))), `{"ok":1}`},
		{"NULL値は省かない", value.DictValue(dictOf("f", value.Null())), `{"f":null}`},
		{"入れ子辞書の関数値も省く",
			value.DictValue(dictOf("child", value.DictValue(dictOf("f", fn, "n", num(1))))),
			`{"child":{"n":1}}`},
		{"配列の中の辞書の関数値も省く",
			value.ArrayValue(value.NewArray(value.DictValue(dictOf("f", fn)))),
			`[{}]`},
		// 配列の中の関数は本家も null にするので現行どおり
		{"配列の中の関数はnull", value.ArrayValue(value.NewArray(fn, num(1))), `[null,1]`},
		{"配列の中の未定義はnull", value.ArrayValue(value.NewArray(value.Undefined())), `[null]`},
		// トップレベルの関数自体は現行どおり null (辞書のキー省略とは別)
		{"トップレベルの関数はnull", fn, `null`},
		// 省かれるキーが整数形式でも出力に残らない (#417 との組合せ)
		{"整数形式キーの関数値も省く", value.DictValue(dictOf("1", fn, "b", num(1))), `{"b":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeJSONForTest(t, tt.in); got != tt.want {
				t.Errorf("JSONエンコード = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONEncodeCompat417ArrayIndexOrder(t *testing.T) {
	tests := []struct {
		name string
		in   value.Value
		want string
	}{
		{"Issueの再現", value.DictValue(dictOf("b", num(1), "1", num(2))), `{"1":2,"b":1}`},
		{"数値昇順であり文字列順ではない", value.DictValue(dictOf("10", num(10), "2", num(2), "1", num(1))),
			`{"1":1,"2":2,"10":10}`},
		{"通常キーは挿入順のまま", value.DictValue(dictOf("10", num(10), "x", num(0), "2", num(2), "1", num(1), "y", num(0))),
			`{"1":1,"2":2,"10":10,"x":0,"y":0}`},
		{"0はインデックス", value.DictValue(dictOf("a", num(1), "0", num(0))), `{"0":0,"a":1}`},
		{"上限4294967294はインデックス", value.DictValue(dictOf("a", num(1), "4294967294", num(2))),
			`{"4294967294":2,"a":1}`},
		{"4294967295はインデックスではない", value.DictValue(dictOf("4294967295", num(1), "0", num(0))),
			`{"0":0,"4294967295":1}`},
		{"境界と非インデックスの混在",
			value.DictValue(dictOf(
				"4294967295", num(1), "4294967294", num(2), "01", num(3), "-1", num(4),
				"0", num(5), " 1", num(6), "1.0", num(7), "+1", num(8), "", num(9))),
			`{"0":5,"4294967294":2,"4294967295":1,"01":3,"-1":4," 1":6,"1.0":7,"+1":8,"":9}`},
		{"先行ゼロと負ゼロはインデックスではない",
			value.DictValue(dictOf("00", num(1), "-0", num(2), "0", num(0))),
			`{"0":0,"00":1,"-0":2}`},
		{"上限を超える大きな数はインデックスではない",
			value.DictValue(dictOf("9007199254740991", num(1), "4294967296", num(2), "3", num(3))),
			`{"3":3,"9007199254740991":1,"4294967296":2}`},
		{"桁数の多い数字だけのキーでも壊れない",
			value.DictValue(dictOf("99999999999999999999999999", num(1), "1", num(1))),
			`{"1":1,"99999999999999999999999999":1}`},
		{"入れ子辞書にも適用する",
			value.DictValue(dictOf("child", value.DictValue(dictOf("b", num(1), "2", num(2), "10", num(3))))),
			`{"child":{"2":2,"10":3,"b":1}}`},
		{"インデックスキーが無ければ挿入順のまま",
			value.DictValue(dictOf("b", num(1), "a", num(2))), `{"b":1,"a":2}`},
		// 省かれる未定義値のインデックスキーは順序計算に影響しない
		{"未定義の整数形式キーは省かれたまま並ぶ",
			value.DictValue(dictOf("2", value.Undefined(), "b", num(1), "1", num(1))),
			`{"1":1,"b":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeJSONForTest(t, tt.in); got != tt.want {
				t.Errorf("JSONエンコード = %q, want %q", got, tt.want)
			}
		})
	}
}

// 並び替えは JSON 出力時だけであり、Dict の保持順は変えない (#417 の論点)。
func TestJSONEncodeCompat417DoesNotChangeDictOrder(t *testing.T) {
	d := dictOf("b", num(1), "1", num(2), "10", num(10))
	if got := encodeJSONForTest(t, value.DictValue(d)); got != `{"1":2,"10":10,"b":1}` {
		t.Fatalf("JSONエンコード = %q", got)
	}
	if got := strings.Join(d.Keys(), ","); got != "b,1,10" {
		t.Errorf("エンコード後の辞書キー順 = %q, want %q (保持順は変わらない)", got, "b,1,10")
	}
}

// 『JSONエンコード整形』も encodeJSON を通るので同じ規則になる。
func TestJSONEncodeCompatFormatted(t *testing.T) {
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("JSONエンコード整形")
	if !ok {
		t.Fatal("JSONエンコード整形 が見つかりません")
	}
	fn := value.FuncValue(&value.Func{ID: 1})
	d := dictOf("b", num(1), "1", num(2), "f", fn)
	got, err := e.Fn(newContext(), []value.Value{value.DictValue(d)})
	if err != nil {
		t.Fatalf("JSONエンコード整形でエラー: %v", err)
	}
	s, _ := got.String()
	want := "{\n  \"1\": 2,\n  \"b\": 1\n}"
	if s != want {
		t.Errorf("JSONエンコード整形 = %q, want %q", s, want)
	}
}

// 『一致』は両辺がオブジェクトのとき JSON 表現で比べるため、
// #416 / #417 の修正がそのまま結果に反映される (Issue 記載の逆転の解消)。
func TestJSONEncodeCompatMatched(t *testing.T) {
	fn := value.FuncValue(&value.Func{ID: 1})
	withFunc := value.DictValue(dictOf("f", fn))
	empty := value.DictValue(value.NewDict())
	withNull := value.DictValue(dictOf("f", value.Null()))
	if got := callCompare(t, "一致", withFunc, empty); !got {
		t.Errorf("関数値だけの辞書と空辞書の『一致』 = %v, want true (#416)", got)
	}
	if got := callCompare(t, "一致", withFunc, withNull); got {
		t.Errorf("関数値だけの辞書とNULL値の辞書の『一致』 = %v, want false (#416)", got)
	}
	if got := callCompare(t, "不一致", withFunc, empty); got {
		t.Errorf("関数値だけの辞書と空辞書の『不一致』 = %v, want false (#416)", got)
	}
	left := value.DictValue(dictOf("b", num(1), "1", num(2)))
	right := value.DictValue(dictOf("1", num(2), "b", num(1)))
	if got := callCompare(t, "一致", left, right); !got {
		t.Errorf("整数形式キーの順序だけが違う辞書の『一致』 = %v, want true (#417)", got)
	}
	// 関数を含む配列どうしは、配列要素の関数がどちらも null になるため一致する (本家と同じ)
	arrF := value.ArrayValue(value.NewArray(fn))
	arrNull := value.ArrayValue(value.NewArray(value.Null()))
	if got := callCompare(t, "一致", arrF, arrNull); !got {
		t.Errorf("[関数]と[NULL]の『一致』 = %v, want true", got)
	}
}
