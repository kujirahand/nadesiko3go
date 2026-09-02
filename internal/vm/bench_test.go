// VMの速さを測るベンチマーク。速くしようとする前に、まずここを取る
// (AGENTS.md §6「VMの速さについて」)。
//
//	go test ./internal/vm/ -run XXX -bench . -benchtime 200x
//
// # これまでに取った値 (Apple M4 / darwin-arm64 / -benchtime 200x)
//
// 数字そのものは機械が変われば動く。意味があるのは変更前後の比と、
// 割り当て回数のほうで、そこが悪くなっていたら何かを踏んでいる。
//
//	                 Loop            Recursion         Calls
//	最適化前          37.2ms/3164     7.62ms/123758     36.9ms/503334
//	呼び出し改善後     36.9ms/3160     6.05ms/ 69030     30.7ms/303331
//	                   ±0             -21% / -44%       -17% / -40%
//
// 「呼び出し改善」= スタックの事前確保・セルのまとめ確保・定数判定マップの
// 廃止・引数の借用 (internal/vm/vm.go の callClosure)。呼び出し1回あたりの
// 割り当ては5個から3個になった。残る3個はフレーム・ローカルのスライス・
// セルのまとめ確保で、これ以上減らすにはフレームの使い回しが要る。
//
// Loop が動かないのは構造的な理由による。プロファイルでは命令ディスパッチ
// が約40%、オペランドスタックの push/pop が約23%を占めていて、ここを縮める
// にはスーパー命令 (LoadLocal+LoadLocal+Binary を1命令にまとめる等) が要る。
// 命令カウンタを外す案も測ったが2%程度だったので、暴走検知を優先して残した。
//
// なお Loop の実行そのものは既にアロケーションフリーで、上の 3160 個は
// ほぼ全部がコンパイル時のもの (BenchmarkCompile とほぼ同じ数)。
package vm_test

import (
	"io"

	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// benchHost throws away everything a benchmarked program prints, so the
// measurement is the language, not the terminal.
type benchHost struct{}

func (benchHost) Print(string)                       {}
func (benchHost) Write(string)                       {}
func (benchHost) ReadLine() (string, error)          { return "", io.EOF }
func (benchHost) Exit(int)                           {}
func (benchHost) Args() []string                     { return nil }
func (benchHost) ReadResource(string) ([]byte, bool) { return nil, false }
func (benchHost) Now() time.Time                     { return time.Unix(0, 0) }

// The two shapes that stress different parts of the interpreter: a loop is
// mostly instruction dispatch and operators, while recursion is mostly the
// per-call work in callClosure.
const (
	loopSource = `S=0
Iを1から200000まで繰り返す
  S=S+(I*2)
ここまで
`
	recursionSource = `●(Nの)フィボとは
もしN<=1ならばNで戻る
(((N-1)のフィボ)+((N-2)のフィボ))で戻る
ここまで
(20のフィボ)を表示
`
	callSource = `●(Aと)足すとは
(A+1)で戻る
ここまで
S=0
Iを1から100000まで繰り返す
  S=(Sと足す)
ここまで
`
)

func benchRun(b *testing.B, source string) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		if err := vm.RunWithHost(source, "bench.nako3", benchHost{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoop measures instruction dispatch and operators.
func BenchmarkLoop(b *testing.B) { benchRun(b, loopSource) }

// BenchmarkRecursion measures the per-call cost: frame, locals, cells.
func BenchmarkRecursion(b *testing.B) { benchRun(b, recursionSource) }

// BenchmarkCalls measures many shallow calls rather than deep ones.
func BenchmarkCalls(b *testing.B) { benchRun(b, callSource) }

// BenchmarkCompile measures parsing and compiling alone, so that the numbers
// above can be read as run time rather than start-up.
func BenchmarkCompile(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := vm.CompileProgram(loopSource, "bench.nako3"); err != nil {
			b.Fatal(err)
		}
	}
}
