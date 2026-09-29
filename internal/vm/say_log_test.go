package vm_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestSayDoesNotPolluteDisplayLog は issue #224 の回帰テスト。『言』は
// stdout へ書き出すだけで表示ログには残さない。本家は
// sys.logger.send('stdout', ...) であり、表示ログは『表示』だけが更新する。
//
// 表示ログはプログラムが参照したときだけ大域変数として確保されるので、
// 末尾で代入の右辺に置いて参照だけ行う（『表示ログを表示』は表示した
// 内容をさらにログへ追記してしまうため使わない）。
func TestSayDoesNotPolluteDisplayLog(t *testing.T) {
	r, err := vm.RunSource(
		"表示ログクリア\n「X」と言\n「Y」と表示\nA=表示ログ",
		"main.nako3",
		[]string{"表示ログ"},
	)
	if err != nil {
		t.Fatal(err)
	}
	// 『言』のXは表示ログにも収集ログにも残らない
	if r.Log != "Y" {
		t.Errorf("出力ログ = %q, want %q", r.Log, "Y")
	}
	if got := value.ToString(r.Vars["表示ログ"]); got != "Y" {
		t.Errorf("表示ログ = %q, want %q", got, "Y")
	}
}
