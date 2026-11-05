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

// #340: 同一内容 (同じ行・列・オフセット) の2つの依存ファイルを取り込んでも、
// 位置は Source の異なる別エントリとして分離され、Sources へ初出順に
// 登録されること (Argonレビュー Medium-1)。
func TestCompileSeparatesSamePositionsAcrossDeps(t *testing.T) {
	dir := t.TempDir()
	body := "「a」を表示\n「b」を表示\n"
	depA := filepath.Join(dir, "depA.nako3")
	depB := filepath.Join(dir, "depB.nako3")
	for _, dep := range []string{depA, depB} {
		if err := os.WriteFile(dep, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(dir, "main.nako3")
	prog, err := vm.CompileProgram("!「./depA.nako3」を取り込む\n!「./depB.nako3」を取り込む\n", main)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Sources) != 3 || prog.Sources[0].Name != main || prog.Sources[1].Name != depA || prog.Sources[2].Name != depB {
		t.Fatalf("Sources = %+v", prog.Sources)
	}
	// 同じ Line/Column/Offset の組が Source 1 と 2 の両方に存在すること。
	type lco struct{ line, col, off int }
	seen := map[lco]map[int]bool{}
	for _, p := range prog.Positions {
		if p.Source != 1 && p.Source != 2 {
			continue
		}
		k := lco{p.Line, p.Column, p.Offset}
		if seen[k] == nil {
			seen[k] = map[int]bool{}
		}
		seen[k][p.Source] = true
	}
	shared := false
	for _, s := range seen {
		if s[1] && s[2] {
			shared = true
		}
	}
	if !shared {
		t.Errorf("同一位置が Source 1/2 の両方に分離されていない: %v", prog.Positions)
	}
	if err := prog.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
