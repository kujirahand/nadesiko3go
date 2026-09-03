package compiler_test

// 覗き穴最適化のテスト。「命令が減ったこと」と「意味が変わらないこと」を
// 別々に見る。減ったかどうかだけを見ていると、消してはいけないものを消した
// ときに気づけない。

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func TestFuseBinary(t *testing.T) {
	// 変数どうし・変数と定数・グローバルと、取得元の組み合わせを変える
	tests := []struct{ code, want string }{
		{"A=3\nB=4\n(A+B)を表示", "7"},
		{"A=3\n(A*2)を表示", "6"},
		{"A=「あ」\n(A&A)を表示", "ああ"},
		{"●(Aの)倍とは\n(A*2)で戻る\nここまで\n(5の倍)を表示", "10"},
		// 捕捉した変数も取得元になる
		{"A=1\nF=関数(B)\n\t(A+B)で戻る\nここまで\nF(2)を表示", "3"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}

	ops := mainOps(t, "A=3\nB=4\n(A+B)を表示")
	if ops[ir.OpBinaryAt] != 1 {
		t.Errorf("BinaryAt にまとまっていない: %v", ops)
	}
	if ops[ir.OpBinary] != 0 {
		t.Errorf("Binary が残っている: %v", ops)
	}
}

func TestFuseBinaryAtStoreLocal(t *testing.T) {
	tests := []struct{ code, want string }{
		{"●テストとは\n\tA=1\n\tA=A+2\n\tAで戻る\nここまで\nテストを表示", "3"},
		{"●(Nの)ステップとは\n\tN=N*2\n\tNで戻る\nここまで\n(5のステップ)を表示", "10"},
		{"S=0\nIを1から5まで繰り返す\n\tS=S+I\nここまで\nSを表示", "15"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}

	prog, err := vm.CompileProgram("●テストとは\n\tA=1\n\tA=A+2\n\tAで戻る\nここまで", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, fn := range prog.Funcs {
		for _, inst := range fn.Code {
			if inst.Op == ir.OpBinaryAtStoreLocal {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("BinaryAtStoreLocal にまとまっていない")
	}
}

func TestFuseJumpBinaryAt(t *testing.T) {
	prog, err := vm.CompileProgram("A=1\nもしA=1ならば\n\t「はい」を表示\n違えば\n\t「いいえ」を表示\nここまで", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, inst := range prog.Funcs[prog.Main].Code {
		if inst.Op == ir.OpJumpIfNotBinaryAt || inst.Op == ir.OpJumpIfBinaryAt {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("JumpIf(Not)BinaryAt にまとまっていない: %v", prog.Funcs[prog.Main].Code)
	}
}

// TestFuseKeepsJumpTargets checks the rewrite around code something jumps to.
// 命令を消すと番号がずれるので、飛び先のつけ替えを間違えると、条件分岐が
// 別の場所へ飛ぶ。値が合っていれば、飛び先も合っている。
func TestFuseKeepsJumpTargets(t *testing.T) {
	tests := []struct{ code, want string }{
		{"S=0\nIを1から5まで繰り返す\n\tS=S+I\nここまで\nSを表示", "15"},
		{"A=1\nもしA=1ならば\n\t「はい」を表示\n違えば\n\t「いいえ」を表示\nここまで", "はい"},
		{"A=2\nもしA=1ならば\n\t「はい」を表示\n違えば\n\t「いいえ」を表示\nここまで", "いいえ"},
		{"I=0\nIが3未満の間繰り返す\n\tI=I+1\nここまで\nIを表示", "3"},
		{"エラー監視\n\t1のエラー発生\nエラーならば\n\t「捕まえた」を表示\nここまで", "捕まえた"},
		{"S=0\n10回\n\tもし回数=5ならば\n\t\t抜ける\n\tここまで\n\tS=S+1\nここまで\nSを表示", "4"},
		{"S=0\n10回\n\tもし回数=5ならば\n\t\t続ける\n\tここまで\n\tS=S+1\nここまで\nSを表示", "9"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestDropLoadPop checks that a value fetched and thrown away costs nothing.
// 戻り値のない命令は『LoadConst 未定義; Pop』で終わるので、文の数だけ効く。
func TestDropLoadPop(t *testing.T) {
	if got, want := run(t, "1を表示\n2を表示"), "1\n2"; got != want {
		t.Errorf("= %q, want %q", got, want)
	}
	ops := mainOps(t, "1を表示\n2を表示")
	// 残ってよい LoadConst は『それ』の初期化1つと、表示の引数2つ
	if n := ops[ir.OpLoadConst]; n != 3 {
		t.Errorf("捨てられる LoadConst が残っている: %d (want 3)", n)
	}
}

// TestPeepholeKeepsSore pins the value 『それ』 ends up with. 『Dup;
// StoreSpecial; Pop』 をまとめる規則は、捨てるのは結果だけで『それ』への
// 代入は残す、という約束の上に立っている。
func TestPeepholeKeepsSore(t *testing.T) {
	tests := []struct{ code, want string }{
		{"「あいう」の文字数\nそれを表示", "3"},
		{"1に2を足す\nそれを表示", "3"},
		// 戻り値のない命令は『それ』を変えない
		{"1に2を足す\n「x」を表示\nそれを表示", "x\n3"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}
