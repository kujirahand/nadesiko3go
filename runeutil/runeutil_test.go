package runeutil

import (
	"testing"
)

func Test1(t *testing.T) {
	eq(t, ToZenkakuKatakana("123"), "123")
	eq(t, ToZenkakuKatakana("ﾍﾟﾍﾟ"), "ペペ")
	eq(t, ToZenkakuKatakana("ﾍﾟｵ"), "ペオ")
	eq(t, ToHankakuKatakana("ペペ"), "ﾍﾟﾍﾟ")
	eq(t, ToHankakuKatakana("ペオ"), "ﾍﾟｵ")
}

func eq(t *testing.T, real, expected string) {
	if real != expected {
		t.Errorf("%s != %s", real, expected)
	}
}
