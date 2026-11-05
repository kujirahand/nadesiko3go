package ir

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func testProgram() Program {
	return Program{
		Version: CurrentVersion,
		Funcs: []Func{{
			Name: "main",
			Code: []Inst{{Op: OpReturn, Pos: 0}},
		}},
		Main:      0,
		Sources:   []SourceFile{{Name: "main.nako3"}},
		Positions: []SourcePos{{Source: 0, Line: 0, Column: 0}},
	}
}

func TestProgramIsSerializableAndValid(t *testing.T) {
	p := testProgram()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Program
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestConstJSONKeepsSignedZero は #339: 数値定数の JSON 往復で +0 と -0 が
// ビット列まで一致すること。通常の omitempty では 0 が省略され、-0 が
// +0 に化けていた。
func TestConstJSONKeepsSignedZero(t *testing.T) {
	for _, n := range []float64{0, math.Copysign(0, -1), 1, -2.5} {
		data, err := json.Marshal(Const{Kind: ConstNumber, Num: n})
		if err != nil {
			t.Fatal(err)
		}
		var got Const
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		if math.Float64bits(got.Num) != math.Float64bits(n) {
			t.Errorf("%s: ビットが %x になった (want %x)", data, math.Float64bits(got.Num), math.Float64bits(n))
		}
	}
	// -0 は省略されずに書き出される
	data, err := json.Marshal(Const{Kind: ConstNumber, Num: math.Copysign(0, -1)})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"kind":3,"num":-0}` {
		t.Errorf("-0 のJSON = %s", data)
	}
}

// TestConstJSONLegacyWithoutNum は既存IRの互換性: num の無い数値定数は
// 従来どおり +0 として読めること。
func TestConstJSONLegacyWithoutNum(t *testing.T) {
	var got Const
	if err := json.Unmarshal([]byte(`{"kind":3}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != ConstNumber || math.Float64bits(got.Num) != 0 {
		t.Errorf("旧形式 = %+v (ビット %x)", got, math.Float64bits(got.Num))
	}
}

// TestConstJSONOtherKindsUnchanged は、数値以外の定数のJSON形が
// 従来と変わらないことを固定する。
func TestConstJSONOtherKindsUnchanged(t *testing.T) {
	cases := []struct {
		k    Const
		want string
	}{
		{Const{Kind: ConstUndefined}, `{"kind":0}`},
		{Const{Kind: ConstNull}, `{"kind":1}`},
		{Const{Kind: ConstBool, Bool: true}, `{"kind":2,"bool":true}`},
		{Const{Kind: ConstBool}, `{"kind":2}`},
		{Const{Kind: ConstString, Str: "あ"}, `{"kind":4,"str":"あ"}`},
		{Const{Kind: ConstString}, `{"kind":4}`},
	}
	for _, c := range cases {
		data, err := json.Marshal(c.k)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != c.want {
			t.Errorf("%+v のJSON = %s, want %s", c.k, data, c.want)
		}
		var back Const
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if back != c.k {
			t.Errorf("%s を読み戻すと %+v になった", data, back)
		}
	}
}

func TestProgramRejectsOtherVersion(t *testing.T) {
	p := testProgram()
	p.Version++
	if err := p.Validate(); err == nil {
		t.Fatal("未知のIRバージョンを受理しました")
	}
}

func TestPacked16BitIndexesRoundTrip(t *testing.T) {
	for _, index := range []int32{0, 32767, 32768, 65535} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			a := EncodeBinaryAtStoreLocal(BinAdd, SrcLocal, SrcConst, index)
			op, left, right, dst := DecodeBinaryAtStoreLocal(a)
			if op != BinAdd || left != SrcLocal || right != SrcConst || dst != index {
				t.Fatalf("BinaryAtStoreLocal round trip = (%v, %v, %v, %d), want (%v, %v, %v, %d)",
					op, left, right, dst, BinAdd, SrcLocal, SrcConst, index)
			}

			c := EncodeJumpBinaryAt(BinLtEq, SrcGlobal, SrcCapture, index)
			op, left, right, got := DecodeJumpBinaryAt(c)
			if op != BinLtEq || left != SrcGlobal || right != SrcCapture || got != index {
				t.Fatalf("JumpBinaryAt round trip = (%v, %v, %v, %d), want (%v, %v, %v, %d)",
					op, left, right, got, BinLtEq, SrcGlobal, SrcCapture, index)
			}
		})
	}
}
