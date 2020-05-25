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

func TestJSON(t *testing.T) {
	eval(t, "[1,2,3]", "[1,2,3]")
	eval(t, "[1,[2,2,2],3]", "[1,[2,2,2],3]")
	eval(t, "{'a':30}", "{\"a\":30}")
	eval(t, "{'a':[1,2,3]}", "{\"a\":[1,2,3]}")
	eval(t, "A={'a':3};A['a']", "3")
	eval(t, "B=[1,2,3];B[1]", "2")
	eval(t, "C=[[1,2,3],[11,22,33],[111,222,333]];C[1][2]", "33")
	eval(t, "D=[1,2];D[1]=1;D", "[1,1]")
	eval(t, "E={'a':30};E['a']=1;E", "{\"a\":1}")
}

func eval(t *testing.T, code, expected string) {
	Eval("表示ログ=「」")
	v, _ := Eval(code)
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main: %s != %s", rv, expected)
	}
}
