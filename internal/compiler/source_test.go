package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// #340: 取り込んだファイルは prog.Sources に1件だけ登録され、
// その命令位置は依存ファイルの ID を指す。
func TestCompileRegistersDepSources(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "dep.nako3")
	if err := os.WriteFile(dep, []byte("「a」を表示\n「b」を表示\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.nako3")
	prog, err := vm.CompileProgram("!「./dep.nako3」を取り込む\n「c」を表示\n", main)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Sources) != 2 || prog.Sources[0].Name != main || prog.Sources[1].Name != dep {
		t.Fatalf("Sources = %+v", prog.Sources)
	}
	found := false
	for _, p := range prog.Positions {
		if p.Source == 1 {
			found = true
		}
	}
	if !found {
		t.Error("依存ファイルの Source を持つ位置がない")
	}
	if err := prog.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
