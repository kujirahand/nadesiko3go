package gogen

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// buildAndRunWithNodelib は nodelib を含めて生成・ビルドし、標準出力と終了コードを返す。
func buildAndRunWithNodelib(t *testing.T, code string) (stdout string, exitCode int) {
	t.Helper()
	registry, err := BuildRegistry([]string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	src, err := Generate(prog, Options{Plugins: []string{"nodelib"}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	binary := filepath.Join(dir, "exit-test")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	var out bytes.Buffer
	run := exec.Command(binary)
	run.Stdout = &out
	err = run.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		exitCode = 0
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		t.Fatalf("生成コードの実行: %v", err)
	}
	return out.String(), exitCode
}

// 『終了』『強制終了』が、VM版と生成コード版で同じ出力・同じ終了コードになる (#337)。
func TestGeneratedExitMatchesVM(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"1_エラー監視内の終了", "エラー監視\n  終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"2_ユーザー定義関数内の終了", "●止める処理とは\n  終了\nここまで\nエラー監視\n  止める処理を実行\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"3_入れ子のエラー監視内の終了", "エラー監視\n  エラー監視\n    終了\n  エラーならば\n    「内側」を表示\n  ここまで\nエラーならば\n  「外側」を表示\nここまで\n「後続」を表示\n"},
		{"4_エラー監視内の強制終了", "エラー監視\n  5で強制終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"8_対照_通常のエラーは捕捉される", "エラー監視\n  「E」でエラー発生\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"関数内にエラー監視を持つ", "●Fとは\n  エラー監視\n    「前」を表示\n    終了\n  エラーならば\n    「捕捉された」を表示\n  ここまで\n  「関数の後続」を表示\nここまで\nFを実行\n「後続」を表示\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("go build/run per case; skipped under -short")
			}
			var want strings.Builder
			host := vm.NewCUIHost(&want, strings.NewReader(""), nil)
			if err := vm.RunProgram(tc.code, "main.nako3", host); err != nil {
				t.Fatalf("vm: %v", err)
			}
			got, gotCode := buildAndRunWithNodelib(t, tc.code)
			if got != want.String() {
				t.Errorf("出力が一致しない\nVM  : %q\ngogen: %q", want.String(), got)
			}
			if gotCode != host.ExitCode {
				t.Errorf("終了コードが一致しない VM=%d gogen=%d", host.ExitCode, gotCode)
			}
		})
	}
}
