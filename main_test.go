package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	eval(t, "1+2", "3")
	eval(t, "1+2*3", "7")
	eval(t, "1に2を足して表示;表示ログ", "3")
	eval(t, "1に2を足して3を足して表示;表示ログ", "6")
}

func eval(t *testing.T, code, expected string) {
	Eval("表示ログ=「」")
	v, _ := Eval(code)
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main: %s != %s", rv, expected)
	}
}
