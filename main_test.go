package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	eval(t, "1+2", "3")
}

func eval(t *testing.T, code, expected string) {
	v, _ := Eval(code)
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main: %s != %s", rv, expected)
	}
}
