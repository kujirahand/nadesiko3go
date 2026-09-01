package compat

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// The expected JSON comes from SPEC.md's value representation and from the
// expected/*.json the TypeScript version generates.
func TestEncodeValue(t *testing.T) {
	arr := func(items ...value.Value) value.Value { return value.ArrayValue(value.NewArray(items...)) }
	d := value.NewDict()
	d.Set("x", value.Number(1))
	d.Set("y", value.Number(2))

	tests := []struct {
		name string
		in   value.Value
		want string
	}{
		{"undefined", value.Undefined(), `{"t":"undefined"}`},
		{"null", value.Null(), `{"t":"null"}`},
		{"bool", value.Bool(true), `{"t":"bool","v":true}`},
		{"整数", value.Number(3), `{"int":true,"t":"num","v":3}`},
		{"小数", value.Number(12.5), `{"int":false,"t":"num","v":12.5}`},
		{"NaN", value.Number(math.NaN()), `{"t":"num","v":"NaN"}`},
		{"Infinity", value.Number(math.Inf(1)), `{"t":"num","v":"Infinity"}`},
		{"-Infinity", value.Number(math.Inf(-1)), `{"t":"num","v":"-Infinity"}`},
		{"マイナスゼロ", value.Number(math.Copysign(0, -1)), `{"t":"num","v":"-0"}`},
		{"文字列", value.String("あ"), `{"len":1,"t":"str","v":"あ"}`},
		// len はコードポイント数。サロゲートペアは1文字。
		{"サロゲートペア", value.String("𩸽あ"), `{"len":2,"t":"str","v":"𩸽あ"}`},
		{"配列", arr(value.Number(1), value.Number(2)),
			`{"len":2,"t":"arr","v":[{"int":true,"t":"num","v":1},{"int":true,"t":"num","v":2}]}`},
		{"辞書", value.DictValue(d),
			`{"keys":["x","y"],"t":"obj","v":{"x":{"int":true,"t":"num","v":1},"y":{"int":true,"t":"num","v":2}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(encodeValue(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.want {
				t.Errorf("encodeValue = %s, want %s", b, tt.want)
			}
		})
	}
}

// TestEncodeCircular pins that a container holding itself is reported rather
// than recursed into forever.
func TestEncodeCircular(t *testing.T) {
	a := value.NewArray(value.Number(1))
	a.Set(1, value.ArrayValue(a))
	b, err := json.Marshal(encodeValue(value.ArrayValue(a)))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"len":2,"t":"arr","v":[{"int":true,"t":"num","v":1},{"t":"circular"}]}`
	if string(b) != want {
		t.Errorf("encodeValue = %s, want %s", b, want)
	}
}
