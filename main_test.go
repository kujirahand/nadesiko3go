package main

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/eval"
)

// for test
func _e(t *testing.T, code, expected string) {
	sys := eval.InitSystem()
	sys.IsDebug = false
	// sys.IsOptimze = false
	v, err := eval.Eval2(code)
	if err != nil {
		t.Errorf("error: %s / code: %s", err.Error(), code)
		return
	}
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main@[%s] %s != %s", code, rv, expected)
	}
}

func TestExec(t *testing.T) {
	_e(t, "(1に2を足)に5を足す", "8")
	_e(t, "1に((1に2を足)に5を足す)を足す", "9")
	_e(t, "1に2を足して3を掛ける", "9")
}

func TestBasic1(t *testing.T) {
	_e(t, "1に2を足す", "3")
}

func TestBasic2(t *testing.T) {
	_e(t, "1+2", "3")
	_e(t, "1+2*3", "7")
	_e(t, "A=1;B=2;C=A+B;C", "3")
}

func TestBasic2a(t *testing.T) {
	_e(t, "1+2", "3")
	_e(t, "1+2*3", "7")
	_e(t, "A=1;B=2;C=A+B;C", "3")
	_e(t, "A=1+2*3;A", "7")
}

func TestLoopForeachArray(t *testing.T) {
	_e(t, "C=0;[1,2,3]を反復,C=C+それ;C", "6")
	_e(t, "C=0;[10,20,30,40]を反復,C=C+それ;C", "100")
	_e(t, "C='';['a','b','c']を反復,C=C+それ;C", "abc")
}
func TestLoopForeachHash(t *testing.T) {
	_e(t, "C=0;{'a':1,'b':2}を反復,C=C+それ;C", "3")
	_e(t, "C=0;{'a':1,'b':2, 'c':3}を反復,C=C+それ;C", "6")
}

func TestLoopKai(t *testing.T) {
	_e(t, "A=0;3回,A=A+5;A", "15")
	_e(t, "A=0;3回\nA=A+5\nここまで;A", "15")
}

func TestLoop(t *testing.T) {
	// 繰り返す
	_e(t, "C=0;Iを1から10まで繰り返す,C=C+I;C", "55")
	_e(t, "C=0;Iを1から10まで繰り返す\nJを1から2まで繰り返す\nC=C+I;ここまで;ここまで;C", "110")
	_e(t, "C=0;Iを1から10まで繰り返す,Jを1から2まで繰り返す,C=C+I;C", "110")
	// 間
	_e(t, "C=0;I=0;(I<=10)の間\nC=C+I;I=I+1;ここまで;C", "55")
	_e(t, "C=0;I=0;(I<=10)の間\nJ=0;(J<2)の間\nC=C+I;J=J+1;ここまで;I=I+1;ここまで;C", "110")
	// 反復
	_e(t, "C=0;[1,2,3]を反復,C=C+それ;C", "6")
	_e(t, "C=0;{'a':1,'b':2}を反復,C=C+それ;C", "3")
}

func TestIf(t *testing.T) {
	_e(t,
		"C=0;もしC=1ならば\n"+
			"　　C=30\n"+
			"違えば\n"+
			"　　C=40;\n"+
			"ここまで;C", "40")
	_e(t, "C=0;もしC=1ならば\nC=30\nここまで;C", "0")
	_e(t, "C=0;もしC=1ならば,C=30。違えば,C=50。C", "50")
}

func TestArray2(t *testing.T) {
	_e(t, "B=0;A=3;もし、A=3ならばB=1;違えばB=0;B", "1")
	// _e(t, "C=[1,2,3];C[1]", "[1,2,3]")
}

func TestArrayHoge(t *testing.T) {
	_e(t, "C=[1,2,3];C[1]", "2")
}
func TestArray(t *testing.T) {
	_e(t, "C=[1,2,3];C", "[1,2,3]")
	_e(t, "C=[1,2,3];C[1]", "2")
	_e(t, "C=[0,1,2];C[0]=3;C[0]", "3")
	_e(t, "C=[[0,1,2],[3,4,5],[6,7,8]];C[0][1]=8;C[0][1]", "8")
	_e(t, "C=[[0,1,2],[3,4,5],[6,7,8]];C[0][1]", "1")
	_e(t, "C={'a':1,'b':2};C['a']", "1")
	_e(t, "C=[1,2,3];Cの要素数", "3")
	_e(t, "C=[];Cの要素数", "0")
}

func TestFunc(t *testing.T) {
	_e(t, "1と2を足して表示;表示ログ", "3")
	_e(t, "C=足す(1,2);C", "3")
	_e(t, "1+2に3+4を足す", "10")
	_e(t, "足す(1,2)を表示;表示ログ", "3")
	_e(t, "C=1に2を足す;C", "3")
	_e(t, "(1に2を足)を表示;表示ログ", "3")
	_e(t, "(「abc」の「a」を「b」に置換)を表示;表示ログ", "bbc")
	_e(t, "([1,2,3]の要素数)を表示;表示ログ", "3")
}

func TestFuncRec(t *testing.T) {
	_e(t, "●FF(Nの)\n1で戻る\nここまで\n(30のFF)を表示;表示ログ", "1")
	_e(t, "●REC(Nの)\nもしN<1ならば、Nで戻る。\n(((N-1)のREC)+N)で戻る\nここまで\n(10のREC)を表示;表示ログ", "55")
	_e(t, "N=5;N-1を表示;表示ログ", "4")
	_e(t, "N=5;N-1と3を足して表示;表示ログ", "7")
}

func TestTemp(t *testing.T) {
	_e(t, "A=(今);もしA<>「」ならば「1」と表示。表示ログ", "1")
}
