package main

import (
	"strings"
	"testing"
)

func TestCheckNakoSyntaxOK(t *testing.T) {
	res := checkNakoSyntax("「こんにちは」と表示。", "")
	if !res.OK {
		t.Errorf("OK = false, error = %q", res.Error)
	}
	if res.Error != "" {
		t.Errorf("Error = %q, want empty", res.Error)
	}
}

func TestCheckNakoSyntaxError(t *testing.T) {
	res := checkNakoSyntax("もし「A」ならば\n「B」と表示。", "test.nako3")
	if res.OK {
		t.Fatal("文法エラーがあるのにOK = trueでした")
	}
	if res.Error == "" {
		t.Error("Error が空でした")
	}
}

func TestFormatNakoCodeChanged(t *testing.T) {
	code := "3回\n「A」と表示。\nここまで\n"
	res := formatNakoCode(code, "test.nako3")
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if !res.Changed {
		t.Error("Changed = false, want true")
	}
	if !strings.Contains(res.Formatted, "    「A」と表示。") {
		t.Errorf("インデントが整形されませんでした: %q", res.Formatted)
	}
}

func TestFormatNakoCodeUnchanged(t *testing.T) {
	code := "3回\n    「A」と表示。\nここまで\n"
	res := formatNakoCode(code, "test.nako3")
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if res.Changed {
		t.Error("Changed = true, want false")
	}
}

func TestFormatNakoCodeRefusesStructureChange(t *testing.T) {
	code := "3回:\n    「A」と表示。\n「終」と表示。\n"
	res := formatNakoCode(code, "colon.nako3")
	if res.OK {
		t.Fatal("構造が変わるのにOK = trueでした")
	}
	if !strings.Contains(res.Error, "構文構造が変わってしまうため中止") {
		t.Errorf("Error = %q", res.Error)
	}
}

func TestFormatNakoCodeSyntaxError(t *testing.T) {
	res := formatNakoCode("もし「A」ならば\n「B」と表示。", "test.nako3")
	if res.OK {
		t.Fatal("文法エラーがあるのにOK = trueでした")
	}
	if res.Error == "" {
		t.Error("Error が空でした")
	}
}
