package stdlib_test

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue V09: JSONエンコード整形 は整形済み文字列を汎用 map へ再パースしていた。
// そのためキーがソートされ、<, >, & が Unicode エスケープされていた。
// 整形が変えてよいのは空白と改行だけで、値の表現は JSONエンコード と同じ。
func TestV09IndentKeepsKeyOrderAndCharacters(t *testing.T) {
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("JSONエンコード整形")
	if !ok {
		t.Fatal("JSONエンコード整形が見つからない")
	}
	tests := []struct {
		name  string
		build func() value.Value
		want  string
	}{
		{
			name: "z、aのキー順と<>の文字表現",
			build: func() value.Value {
				d := value.NewDict()
				d.Set("z", value.String("<"))
				d.Set("a", value.String(">"))
				return value.DictValue(d)
			},
			want: "{\n  \"z\": \"<\",\n  \"a\": \">\"\n}",
		},
		{
			name: "&の文字表現",
			build: func() value.Value {
				d := value.NewDict()
				d.Set("b", value.String("R&D"))
				d.Set("a", value.String("1<2>3"))
				return value.DictValue(d)
			},
			want: "{\n  \"b\": \"R&D\",\n  \"a\": \"1<2>3\"\n}",
		},
		{
			name: "入れ子のキー順",
			build: func() value.Value {
				inner := value.NewDict()
				inner.Set("b", value.Number(1))
				inner.Set("a", value.ArrayValue(value.NewArray(value.Number(1), value.Number(2))))
				d := value.NewDict()
				d.Set("z", value.DictValue(inner))
				d.Set("a", value.String("&"))
				return value.DictValue(d)
			},
			want: "{\n  \"z\": {\n    \"b\": 1,\n    \"a\": [\n      1,\n      2\n    ]\n  },\n  \"a\": \"&\"\n}",
		},
		{
			name: "配列を含む整形",
			build: func() value.Value {
				first := value.NewDict()
				first.Set("z", value.Number(1))
				first.Set("a", value.Number(2))
				second := value.NewDict()
				second.Set("b", value.Number(3))
				return value.ArrayValue(value.NewArray(value.DictValue(first), value.DictValue(second)))
			},
			want: "[\n  {\n    \"z\": 1,\n    \"a\": 2\n  },\n  {\n    \"b\": 3\n  }\n]",
		},
		{
			name: "非有限数はnullのまま",
			build: func() value.Value {
				d := value.NewDict()
				d.Set("n", value.Number(math.Inf(1)))
				d.Set("m", value.Number(math.NaN()))
				return value.DictValue(d)
			},
			want: "{\n  \"n\": null,\n  \"m\": null\n}",
		},
		{
			name: "共有だが非循環の値",
			build: func() value.Value {
				shared := value.NewDict()
				shared.Set("v", value.Number(1))
				d := value.NewDict()
				d.Set("a", value.DictValue(shared))
				d.Set("b", value.DictValue(shared))
				return value.DictValue(d)
			},
			want: "{\n  \"a\": {\n    \"v\": 1\n  },\n  \"b\": {\n    \"v\": 1\n  }\n}",
		},
		{
			name: "空の辞書",
			build: func() value.Value {
				return value.DictValue(value.NewDict())
			},
			want: "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Fn(newContext(), []value.Value{tt.build()})
			if err != nil {
				t.Fatalf("JSONエンコード整形: %v", err)
			}
			s, _ := got.String()
			if s != tt.want {
				t.Fatalf("got=%q, want=%q", s, tt.want)
			}
		})
	}
}

// 空白をすべて詰めた整形結果は通常の JSONエンコード と一致する。
// 整形が値の表現を変えていないことを、出力全体で確かめる。
func TestV09IndentOnlyChangesWhitespace(t *testing.T) {
	r := stdlib.NewRegistry()
	indentCmd, _ := r.Lookup("JSONエンコード整形")
	plainCmd, _ := r.Lookup("JSONエンコード")
	inner := value.NewDict()
	inner.Set("b", value.String("<&>"))
	inner.Set("a", value.ArrayValue(value.NewArray(value.Number(1), value.Number(2))))
	d := value.NewDict()
	d.Set("z", value.DictValue(inner))
	d.Set("a", value.String("日本語"))
	v := value.DictValue(d)

	indented, err := indentCmd.Fn(newContext(), []value.Value{v})
	if err != nil {
		t.Fatalf("JSONエンコード整形: %v", err)
	}
	plain, err := plainCmd.Fn(newContext(), []value.Value{v})
	if err != nil {
		t.Fatalf("JSONエンコード: %v", err)
	}
	indentedStr, _ := indented.String()
	plainStr, _ := plain.String()

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(indentedStr)); err != nil {
		t.Fatalf("整形結果がJSONとして読めない: %v", err)
	}
	if compact.String() != plainStr {
		t.Fatalf("空白除去後=%q, JSONエンコード=%q", compact.String(), plainStr)
	}
}

// 循環参照は JSONエンコード と同じくエラーにする（整形経路でもプロセスを落とさない）。
func TestV09IndentCyclicError(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("JSONエンコード整形")
	d := value.NewDict()
	d.Set("self", value.DictValue(d))
	if _, err := e.Fn(newContext(), []value.Value{value.DictValue(d)}); err == nil {
		t.Fatal("循環する辞書はエラーになるべき")
	}
}
