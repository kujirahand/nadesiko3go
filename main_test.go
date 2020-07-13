package main

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/eval"
)

func TestBasic1(t *testing.T) {
	_eval2(t, "1に2を足す", "3")
}

func TestBasic2a(t *testing.T) {
	_eval2(t, "1+2", "3")
	_eval2(t, "1+2*3", "7")
	_eval2(t, "A=1;B=2;C=A+B;C", "3")
	_eval2(t, "A=1+2*3;A", "7")
}

func TestLoop(t *testing.T) {
	// 繰り返す
	_eval2(t, "C=0;Iを1から10まで繰り返す,C=C+I;C", "55")
	_eval2(t, "C=0;Iを1から10まで繰り返す\nJを1から2まで繰り返す\nC=C+I;ここまで;ここまで;C", "110")
	_eval2(t, "C=0;Iを1から10まで繰り返す,Jを1から2まで繰り返す,C=C+I;C", "110")
	// 間
	_eval2(t, "C=0;I=0;(I<=10)の間\nC=C+I;I=I+1;ここまで;C", "55")
	_eval2(t, "C=0;I=0;(I<=10)の間\nJ=0;(J<2)の間\nC=C+I;J=J+1;ここまで;I=I+1;ここまで;C", "110")
	// 反復
	_eval2(t, "C=0;[1,2,3]を反復,C=C+それ;C", "6")
	_eval2(t, "C=0;{'a':1,'b':2}を反復,C=C+それ;C", "3")
}

func TestIf(t *testing.T) {
	_eval2(t,
		"C=0;もしC=1ならば\n"+
			"　　C=30\n"+
			"違えば\n"+
			"　　C=40;\n"+
			"ここまで;C", "40")
	_eval2(t, "C=0;もしC=1ならば\nC=30\nここまで;C", "0")
	_eval2(t, "C=0;もしC=1ならば,C=30。違えば,C=50。C", "50")
}
func TestArray(t *testing.T) {
	_eval2(t, "C=[1,2,3];C", "[1,2,3]")
	_eval2(t, "C=[1,2,3];C[1]", "2")
	_eval2(t, "C=[0,1,2];C[0]=3;C[0]", "3")
	_eval2(t, "C=[[0,1,2],[3,4,5],[6,7,8]];C[0][1]=8;C[0][1]", "8")
	_eval2(t, "C=[[0,1,2],[3,4,5],[6,7,8]];C[0][1]", "1")
	_eval2(t, "C={'a':1,'b':2};C['a']", "1")
	_eval2(t, "C=[1,2,3];Cの要素数", "3")
	_eval2(t, "C=[];Cの要素数", "0")
}

func TestFunc(t *testing.T) {
	_eval2(t, "1と2を足して表示;表示ログ", "3")
	_eval2(t, "C=足す(1,2);C", "3")
	_eval2(t, "1+2に3+4を足す", "10")
	_eval2(t, "足す(1,2)を表示;表示ログ", "3")
	_eval2(t, "C=1に2を足す;C", "3")
	_eval2(t, "(1に2を足)を表示;表示ログ", "3")
	_eval2(t, "(「abc」の「a」を「b」に置換)を表示;表示ログ", "bbc")
	_eval2(t, "([1,2,3]の要素数)を表示;表示ログ", "3")
}

func TestFuncRec(t *testing.T) {
	_eval2(t, "●FF(Nの)\n1で戻る\nここまで\n(30のFF)を表示;表示ログ", "1")
	_eval2(t, "●REC(Nの)\nもしN<1ならば、Nで戻る。\n(((N-1)のREC)+N)で戻る\nここまで\n(10のREC)を表示;表示ログ", "55")
	_eval2(t, "N=5;N-1を表示;表示ログ", "4")
	_eval2(t, "N=5;N-1と3を足して表示;表示ログ", "7")
}

func TestTemp(t *testing.T) {
	_eval2(t, "●FF(Nの)\n1で戻る\nここまで\n(30のFF)を表示;表示ログ", "1")
}

func _eval2(t *testing.T, code, expected string) {
	sys := eval.InitSystem()
	sys.IsDebug = true
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

/*
func TestMain(t *testing.T) {
	_eval(t, "1+2", "3")
	_eval(t, "1+2*3", "7")
	_eval(t, "1に2を足して表示;表示ログ", "3")
	_eval(t, "1に2を足して3を足して表示;表示ログ", "6")
}

func TestCallFunc(t *testing.T) {
	_eval(t, "N=(1と2を足す);N", "3")
	_eval(t, "A=3;B=5;N=((A-1)とBを足す);N", "7")
}

func TestDeffFunc(t *testing.T) {
	_eval(t, "●(AとBを)ADDとは\nそれはA+B\nここまで。\n1と2をADD", "3")
	_eval(t, "●(AとBを)ABCとは\nC=A*2;D=B*3;それはC+D\nここまで。\n1と2をABC", "8")
	_eval(t, "●(Aで)ABCとは\nA+1で戻る\nここまで。\n3でABC", "4")
	_eval(t, "●(Aで)ABCとは\nもしA<1ならばAで戻る。((A-1)でABC)+Aで戻る。\nここまで。\n10でABC", "55")
}

func TestDeffFunc2(t *testing.T) {
	_eval(t, "●(Aの)BBB\nそれはA*2\nここまで\n3のBBB;", "6")
}

func TestSyntax(t *testing.T) {
	_eval(t, "N=0;[1,2,3]を反復,N=N+対象。N", "6")
}

func TestJSON(t *testing.T) {
	_eval(t, "[1,2,3]", "[1,2,3]")
	_eval(t, "[1,[2,2,2],3]", "[1,[2,2,2],3]")
	_eval(t, "{'a':30}", "{\"a\":30}")
	_eval(t, "{'a':[1,2,3]}", "{\"a\":[1,2,3]}")
	_eval(t, "A={'a':3};A['a']", "3")
	_eval(t, "B=[1,2,3];B[1]", "2")
	_eval(t, "C=[[1,2,3],[11,22,33],[111,222,333]];C[1][2]", "33")
	_eval(t, "D=[1,2];D[1]=1;D", "[1,1]")
	_eval(t, "E={'a':30};E['a']=1;E", "{\"a\":1}")
}

func _evalDebug(t *testing.T, code, expected string) {
	sys := eval.InitSystem()
	sys.IsDebug = true
	eval.Eval("表示ログ=「」")
	v, err := eval.Eval(code)
	if err != nil {
		t.Errorf("error: %s / code: %s", err.Error(), code)
	}
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main: %s != %s", rv, expected)
	}
}

func _eval(t *testing.T, code, expected string) {
	eval.Eval("表示ログ=「」")
	v, err := eval.Eval(code)
	if err != nil {
		t.Errorf("error: %s / code: %s", err.Error(), code)
	}
	rv := v.ToString()
	if rv != expected {
		t.Errorf("main: %s != %s", rv, expected)
	}
}
*/
