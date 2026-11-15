package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// トップレベルで代入した後に定数として宣言し直すとエラーになる。
func TestIssue476TopLevelConstRedeclareError(t *testing.T) {
	if _, err := vm.RunSource("C=1\nCとは定数=3\n", "main.nako3", nil); err == nil {
		t.Fatal("定数の再宣言がエラーになりませんでした")
	}
}

// runIssue476Files は lib をファイルに書き、main から取り込んで実行する。
func runIssue476Files(t *testing.T, libName, libCode, mainCode, want string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, libName), []byte(libCode), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.nako3")
	prog, err := vm.CompileProgramFile(mainCode, "main.nako3", mainPath)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	if err := vm.RunCompiled(prog, host); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("log = %q, want %q", out.String(), want)
	}
}

// 自モジュールの後定義グローバルが、取り込み元の同名グローバルより優先される。
func TestIssue476ModuleOwnPriority(t *testing.T) {
	lib := "●LIB計算とは\n  結果=10\n  結果を戻す\nここまで\n結果=1\n"
	main := "!「./lib_own.nako3」を取り込む\n結果=99\nLIB計算を表示\n結果を表示\n"
	runIssue476Files(t, "lib_own.nako3", lib, main, "10\n99\n")
}

// 非公開の変数は、他のモジュールの関数から見えない。
func TestIssue476ModuleNonExportedHidden(t *testing.T) {
	lib := "秘とは変数{非公開}=5\n●LIB秘読取とは\n  秘を戻す\nここまで\n"
	main := "!「./lib_secret.nako3」を取り込む\n●MAIN読取とは\n  秘を戻す\nここまで\nMAIN読取を表示\nLIB秘読取を表示\n"
	runIssue476Files(t, "lib_secret.nako3", lib, main, "undefined\n5\n")
}

// 取り込んだモジュールの関数は、自モジュールの後定義グローバルを読める。
func TestIssue476ModuleLateGlobal(t *testing.T) {
	lib := "●LIB読取とは\n  共有値を戻す\nここまで\n共有値=7\n"
	main := "!「./lib_late.nako3」を取り込む\nLIB読取を表示\n"
	runIssue476Files(t, "lib_late.nako3", lib, main, "7\n")
}
