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
