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
		{"整数", value.Number(3), `{"t":"num","v":3,"int":true}`},
		{"小数", value.Number(12.5), `{"t":"num","v":12.5,"int":false}`},
		{"NaN", value.Number(math.NaN()), `{"t":"num","v":"NaN"}`},
		{"Infinity", value.Number(math.Inf(1)), `{"t":"num","v":"Infinity"}`},
		{"-Infinity", value.Number(math.Inf(-1)), `{"t":"num","v":"-Infinity"}`},
		{"マイナスゼロ", value.Number(math.Copysign(0, -1)), `{"t":"num","v":"-0"}`},
		{"文字列", value.String("あ"), `{"t":"str","v":"あ","len":1}`},
		// len はコードポイント数。サロゲートペアは1文字。
		{"サロゲートペア", value.String("𩸽あ"), `{"t":"str","v":"𩸽あ","len":2}`},
		{"配列", arr(value.Number(1), value.Number(2)),
			`{"t":"arr","len":2,"v":[{"t":"num","v":1,"int":true},{"t":"num","v":2,"int":true}]}`},
		{"辞書", value.DictValue(d),
			`{"t":"obj","keys":["x","y"],"v":{"x":{"t":"num","v":1,"int":true},"y":{"t":"num","v":2,"int":true}}}`},
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
	want := `{"t":"arr","len":2,"v":[{"t":"num","v":1,"int":true},{"t":"circular"}]}`
	if string(b) != want {
		t.Errorf("encodeValue = %s, want %s", b, want)
	}
}

// TestEncodeValueKeepsInsertionOrder pins the bug testdata/compat/expected's
// 06_dict.json「辞書-キーの並び順」exposes: encoding a dict as a Go
// map[string]any lets encoding/json sort its keys alphabetically, which
// silently fails check_compat.mjs's raw JSON.stringify comparison for any
// dict not already in alphabetical order. Both the field order (t, keys, v)
// and the "v" object's own key order must follow insertion, matching "keys".
func TestEncodeValueKeepsInsertionOrder(t *testing.T) {
	d := value.NewDict()
	d.Set("z", value.Number(1))
	d.Set("a", value.Number(2))
	d.Set("m", value.Number(3))

	b, err := json.Marshal(encodeValue(value.DictValue(d)))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"t":"obj","keys":["z","a","m"],"v":{"z":{"t":"num","v":1,"int":true},"a":{"t":"num","v":2,"int":true},"m":{"t":"num","v":3,"int":true}}}`
	if string(b) != want {
		t.Errorf("encodeValue = %s, want %s", b, want)
	}
}

// TestResultVarsKeepsRequestOrder pins the analogous bug in how runCase
// builds Result.Vars: it too must not be a plain Go map, or a multi-variable
// case whose vars are requested out of alphabetical order would fail the
// same string comparison.
func TestResultVarsKeepsRequestOrder(t *testing.T) {
	vars := ordered()
	vars.set("Z", encodeValue(value.Number(1)))
	vars.set("A", encodeValue(value.Number(2)))
	b, err := json.Marshal(vars)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Z":{"t":"num","v":1,"int":true},"A":{"t":"num","v":2,"int":true}}`
	if string(b) != want {
		t.Errorf("vars = %s, want %s", b, want)
	}
}
