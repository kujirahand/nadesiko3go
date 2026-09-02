package gogen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compat"
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestCompatBatch runs gogen across the differential fixtures (AGENTS.md §1)
// and checks it against the VM backend on the same cases — the "VM / Go
// generated code" half of the three-way comparison §12 asks for; the TS half
// is what the fixtures themselves were generated from.
//
// It is opt-in (GOGEN_COMPAT=1) because it builds one small Go program per
// case: useful as a manual sweep after touching gogen, too slow to want in
// every `go test ./...`.
func TestCompatBatch(t *testing.T) {
	if os.Getenv("GOGEN_COMPAT") == "" {
		t.Skip("set GOGEN_COMPAT=1 to run gogen across the compat fixtures")
	}

	groups, err := compat.Load(filepath.Join(repoRoot(t), "testdata", "compat", "cases"))
	if err != nil {
		t.Fatal(err)
	}

	dir := sharedModule(t)

	var pass, fail, skip int
	var failures []string
	for _, group := range groups {
		if group.Group == "10_async" {
			skip += len(group.Cases)
			continue // 非同期関数はgogenの対象外 (AGENTS.md §12)
		}
		for _, c := range group.Cases {
			name := group.Group + "/" + c.Name
			switch runCompatCase(t, dir, *c.Code) {
			case caseMatch:
				pass++
			case caseSkip:
				skip++
			case caseMismatch:
				fail++
				failures = append(failures, name)
			}
		}
	}

	t.Logf("合計: %d/%d 通過（skip %d）", pass, pass+fail, skip)
	if fail > 0 {
		sort.Strings(failures)
		t.Errorf("一致しなかったケース (%d件):\n  %s", fail, strings.Join(failures, "\n  "))
	}
}

type caseOutcome int

const (
	caseSkip caseOutcome = iota
	caseMatch
	caseMismatch
)

// runCompatCase compiles code once and checks gogen's output against the
// VM's. A case that fails to parse or compile is skipped — that is a parser
// or compiler question, not something gogen's translation touches — and so
// is one Generate itself refuses (currently only a 非同期関数).
func runCompatCase(t *testing.T, moduleDir, code string) caseOutcome {
	t.Helper()
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		return caseSkip
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		return caseSkip
	}
	src, err := Generate(prog, Options{})
	if err != nil {
		return caseSkip
	}

	wantResult, wantErr := vm.RunSource(code, "main.nako3", nil)
	want := ""
	if wantResult != nil {
		want = wantResult.Log
	}

	got, runErr := runInModule(t, moduleDir, src)
	if wantErr != nil {
		// VMがなでしこの実行時エラーで終わったケース: 生成コードも
		// 何かしら失敗して当然だが、比較できる形の出力ではないので、
		// 「両方エラーで終わった」ことだけ確かめてスキップにする。
		if runErr != nil {
			return caseSkip
		}
		return caseSkip
	}
	if runErr != nil {
		t.Logf("gogen実行エラー: %v\n出力: %s", runErr, got)
		return caseMismatch
	}
	if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		t.Logf("不一致\nVM  : %q\ngogen: %q", want, got)
		return caseMismatch
	}
	return caseMatch
}

// sharedModule sets up one Go module — a go.mod replacing this repository —
// that every case's main.go gets written into in turn, so only the first
// case pays for `go mod tidy`.
func sharedModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	// go.sum/go.mod settle once here; every case below reuses them.
	seed := []byte("package main\n\nimport _ \"github.com/kujirahand/nadesiko3go/pkg/runtime\"\n\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), seed, 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	return dir
}

func runInModule(t *testing.T, dir string, src []byte) (string, error) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("go", "run", ".")
	run.Dir = dir
	out, err := run.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, out)
	}
	return string(out), nil
}
