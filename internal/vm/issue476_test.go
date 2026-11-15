package vm_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// runIssue476 は code を実行して、表示ログを比較する。
func runIssue476(t *testing.T, code, want string) {
	t.Helper()
	r, err := vm.RunSource(code, "main.nako3", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Log != want {
		t.Errorf("log = %q, want %q", r.Log, want)
	}
}

// 関数定義より後で代入したグローバル変数を、関数内から読み書きできる（#476）。
func TestIssue476LateAssign(t *testing.T) {
	code := "●後定義読取とは\n  Gを戻す\nここまで\n" +
		"●後定義書込とは\n  G=123\nここまで\n" +
		"G=99\nR=後定義読取\n「読取={R}」を表示\n後定義書込\n「書込後のG={G}」を表示\n"
	runIssue476(t, code, "読取=99\n書込後のG=123")
}

// 関数定義より後の『変数』宣言も同様。
func TestIssue476LateVarDecl(t *testing.T) {
	code := "●読む\n  Aを戻す\nここまで\n" +
		"●書く\n  A=7\nここまで\n" +
		"Aとは変数=1\n書く\n(読む)を表示\n"
	runIssue476(t, code, "7")
}

// 辞書を関数定義の後で定義しても、関数から要素を読める。
func TestIssue476LateDict(t *testing.T) {
	code := "●読む\n  D[\"a\"]を戻す\nここまで\n" +
		"D={\"a\":5}\n(読む)を表示\n"
	runIssue476(t, code, "5")
}

// 同名の明示ローカルがあれば、従来どおりローカルが優先される。
func TestIssue476ExplicitLocalWins(t *testing.T) {
	code := "●関数X\n  変数 A=50\n  Aを戻す\nここまで\n" +
		"A=1\n(関数X)を表示\nAを表示\n"
	runIssue476(t, code, "50\n1")
}

// 関数の中だけで暗黙に代入した変数はローカルのまま。
func TestIssue476ImplicitLocalStays(t *testing.T) {
	code := "●関数Y\n  Z=10\n  Zを戻す\nここまで\n" +
		"(関数Y)を表示\nZを表示\n"
	runIssue476(t, code, "10\nundefined")
}

// 名前付き関数をまたいだ無名関数は、外側の無名関数のローカルを捕捉しない。
func TestIssue476ScopeCutAtNamedFunc(t *testing.T) {
	code := "F=関数\n  変数 A=111\n" +
		"  ●内側とは\n    G=関数\n      Aを戻す\n    ここまで\n    Gを実行\n  ここまで\n" +
		"  内側を戻す\nここまで\n" +
		"A=222\nFを実行して表示\n"
	runIssue476(t, code, "222")
}

// 関数定義より後で宣言された定数へ、関数内から代入するとエラーになる。
func TestIssue476LateConstAssignError(t *testing.T) {
	code := "●Fとは\n  C=9\nここまで\n" +
		"Cとは定数=3\nF\n"
	if _, err := vm.RunSource(code, "main.nako3", nil); err == nil {
		t.Fatal("定数への代入がエラーになりませんでした")
	}
}

// 無名関数から後定義グローバルを参照できる。
func TestIssue476FuncObj(t *testing.T) {
	code := "F=関数\n  Gを戻す\nここまで\n" +
		"G=42\nFを実行して表示\n"
	runIssue476(t, code, "42")
}
