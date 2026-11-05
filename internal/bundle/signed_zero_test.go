package bundle_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestBuildOpenKeepsSignedZero は #339 の受け入れ条件: 梱包 (Build) して
// 開き直しても (Open)、-0 のビットが +0 に化けないこと。梱包は IR を
// JSON で保存するので、数値ゼロの省略が符号を落としていた。
func TestBuildOpenKeepsSignedZero(t *testing.T) {
	dir := t.TempDir()
	prog, err := vm.CompileProgram("A=0\nB=-0\n(1/A)を表示\n(1/B)を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "main.nako3", ""); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	defer packed.Close()

	bits := map[uint64]bool{}
	for _, k := range packed.Program.Consts {
		if k.Kind == ir.ConstNumber {
			bits[math.Float64bits(k.Num)] = true
		}
	}
	if !bits[0] {
		t.Error("梱包後の定数に +0 がない")
	}
	if !bits[math.Float64bits(math.Copysign(0, -1))] {
		t.Error("梱包で -0 の符号が失われた")
	}
}
