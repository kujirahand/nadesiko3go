package gogen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// generateForPackage は code をコンパイルし、指定パッケージ名で
// 生成したGoソースを返す。
func generateForPackage(t *testing.T, code, pkg string) []byte {
	t.Helper()
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	src, err := Generate(prog, Options{Package: pkg})
	if err != nil {
		t.Fatalf("generate (package=%s): %v", pkg, err)
	}
	return src
}

// writeGoMod は生成コードを別モジュールとしてビルドするための
// go.mod を dir に書く (既存テストと同じく replace でこのリポジトリを指す)。
func writeGoMod(t *testing.T, dir string) {
	t.Helper()
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tidyModule(t *testing.T, dir string, src []byte) {
	t.Helper()
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s\n--- source ---\n%s", err, out, src)
	}
}

// TestGenerateMainPackageBuilds は、main パッケージで生成したコードが
// ビルドでき、実行すると「OK」と表示されることを確かめる (#332)。
func TestGenerateMainPackageBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	if got := buildAndRun(t, "「OK」を表示\n"); got != "OK\n" {
		t.Fatalf("main パッケージの出力が違う: got %q want %q", got, "OK\n")
	}
}

// TestGenerateLibraryPackageBuildsAndRuns は、main 以外のパッケージ
// (nakolib) で生成したコードがビルドでき、別パッケージから
// その Run を呼んで「OK」の出力を得られることを確かめる (#332)。
// 修正前は fmt と os が未使用 import になりビルドが失敗していた。
func TestGenerateLibraryPackageBuildsAndRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	src := generateForPackage(t, "「OK」を表示\n", "nakolib")

	// ライブラリ側には main 専用の import と func main が含まれない。
	for _, unwanted := range []string{"\t\"fmt\"\n", "\t\"os\"\n", "func main()"} {
		if strings.Contains(string(src), unwanted) {
			t.Fatalf("nakolib の生成コードに %q が含まれている\n--- source ---\n%s", unwanted, src)
		}
	}
	if !strings.Contains(string(src), "package nakolib\n") {
		t.Fatalf("パッケージ名が nakolib になっていない\n--- source ---\n%s", src)
	}

	dir := t.TempDir()
	libDir := filepath.Join(dir, "nakolib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "nako.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	// 別パッケージ (package main) から生成ライブラリを呼ぶ側。
	caller := `package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gogentest/nakolib"

	rt "github.com/kujirahand/nadesiko3go/pkg/runtime"
)

func main() {
	var buf bytes.Buffer
	host := rt.NewCUIHost(&buf, strings.NewReader(""), nil)
	if err := nakolib.Run(host); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(buf.String())
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, dir)
	tidyModule(t, dir, src)

	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./...: %v\n%s\n--- source ---\n%s", err, out, src)
	}

	run := exec.Command("go", "run", ".")
	run.Dir = dir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	if got := string(out); got != "OK\n" {
		t.Fatalf("生成ライブラリの出力が違う: got %q want %q", got, "OK\n")
	}
}
