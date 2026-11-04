package vm_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Issue #346: 値の同一性比較。
// 『等』は参照の同一性、『一致』は内容の比較、『=』は両辺がオブジェクトの
// とき参照の同一性で決まることを、なでしこプログラムの入口から固定する。
// マニュアル側の DocTest は nadesiko3doc の 文法/値の同一性.txt
// (nadesiko3doc#115) に同じ内容を記述する。
func TestIdentity346FromSource(t *testing.T) {
	tests := []struct{ name, code, want string }{
		// Issue の再現そのもの
		{"別配列が等しいは偽", "A=[1];B=[1];(AがBと等しい)を表示", "false"},
		{"配列と辞書の=は偽", "A=[「[object Object]」];B={};(A=B)を表示", "false"},
		// 等と一致の分離
		{"別配列の等", "A=[1];B=[1];(AとBが等)を表示", "false"},
		{"別配列の一致", "A=[1];B=[1];(AとBが一致)を表示", "true"},
		{"別配列の等無", "A=[1];B=[1];(AとBが等無)を表示", "true"},
		{"別配列の不一致", "A=[1];B=[1];(AとBが不一致)を表示", "false"},
		{"同じ参照の等", "A=[1];B=A;(AとBが等)を表示", "true"},
		{"同じ参照の一致", "A=[1];B=A;(AとBが一致)を表示", "true"},
		{"同じ参照の=", "A=[1];B=A;(A=B)を表示", "true"},
		{"別配列の=", "A=[1];B=[1];(A=B)を表示", "false"},
		{"別配列の===", "A=[1];B=[1];(A===B)を表示", "false"},
		{"別辞書の等", "A={「x」:1};B={「x」:1};(AとBが等)を表示", "false"},
		{"別辞書の一致", "A={「x」:1};B={「x」:1};(AとBが一致)を表示", "true"},
		// 片辺だけがオブジェクトの『=』は変換して比べる (回帰防止)
		{"[1]=1", "([1]=1)を表示", "true"},
		{"[1]=「1」", "([1]=「1」)を表示", "true"},
		{"辞書=文字列", "A={};(A=「[object Object]」)を表示", "true"},
		{"NULL=未定義", "A=NULL;B=未定義;(A=B)を表示", "true"},
		// プリミティブ値の比較 (回帰防止)
		{"1=「1」", "(1=「1」)を表示", "true"},
		{"1と「1」の等", "(1と「1」が等)を表示", "false"},
		{"1と「1」の一致", "(1と「1」が一致)を表示", "false"},
		{"数値の等", "(1と1が等)を表示", "true"},
		{"NULLとNULLの等", "(NULLとNULLが等)を表示", "true"},
		{"NULLとNULLの一致", "(NULLとNULLが一致)を表示", "true"},
		{"NULLと0の等無", "(NULLと0が等無)を表示", "true"},
		{"NULLと辞書の一致", "A=NULL;B={};(AとBが一致)を表示", "false"},
		// 否定の演算子も両辺オブジェクトで参照判定になる
		{"別配列の≠", "A=[1];B=[1];(A≠B)を表示", "true"},
		{"別配列の!==", "A=[1];B=[1];(A!==B)を表示", "true"},
		{"同じ参照の≠", "A=[1];B=A;(A≠B)を表示", "false"},
		// 非数と未定義の『一致』 (本家と同じ結果になること)
		{"非数と非数の一致", "(非数と非数が一致)を表示", "false"},
		{"[非数]どうしの一致", "A=[非数];B=[非数];(AとBが一致)を表示", "true"},
		{"[非数]と[NULL]の一致", "A=[非数];B=[NULL];(AとBが一致)を表示", "true"},
		{"未定義どうしの一致", "(未定義と未定義が一致)を表示", "true"},
		// 片辺だけ循環していても、相手がプリミティブならエラーにならない
		{"片辺循環と数値の一致", "A={};A[「自己」]=A;(Aと1が一致)を表示", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// 循環参照どうしの『一致』は JSON 化でエラーになる (本家の
// JSON.stringify の例外相当)。黙って真偽に潰さない。
func TestIdentity346MatchedCyclicFromSource(t *testing.T) {
	code := "A={};A[「自己」]=A;B={};B[「自己」]=B;(AとBが一致)を表示"
	_, err := vm.RunSource(code, "main.nako3", nil)
	if err == nil {
		t.Fatal("循環辞書の『一致』はエラーになるべき")
	}
	if !strings.Contains(err.Error(), "循環") {
		t.Fatalf("エラーの内容が想定外: %v", err)
	}
}

// 片辺が循環参照でも、相手が NULL (typeof が "object") なら JSON 化に
// 入るため、やはりエラーになる。
func TestIdentity346MatchedCyclicVsNullFromSource(t *testing.T) {
	code := "A={};A[「自己」]=A;(AとNULLが一致)を表示"
	_, err := vm.RunSource(code, "main.nako3", nil)
	if err == nil {
		t.Fatal("循環辞書とNULLの『一致』はエラーになるべき")
	}
	if !strings.Contains(err.Error(), "循環") {
		t.Fatalf("エラーの内容が想定外: %v", err)
	}
}
