package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot finds this module's root from the test file's own location, the
// way internal/gogen's tests do — this repository is itself a valid
// nadesiko3go checkout, so pointing sourceDirForGoBuild at it proves the
// whole pipeline without touching the network.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cmd/gonako-gui/gobuild_test.go -> repo root is two levels up.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func TestIsValidSourceCheckout(t *testing.T) {
	if !isValidSourceCheckout(repoRoot(t)) {
		t.Fatal("このリポジトリ自身が有効なチェックアウトとして認識されない")
	}
	if isValidSourceCheckout(t.TempDir()) {
		t.Fatal("空のフォルダを有効と判定してしまった")
	}
}

// TestBuildWithGoUsesExistingCheckout proves the pipeline end-to-end without
// any network access: pointing sourceDirForGoBuild at this repository (a
// valid checkout already) means buildWithGo must never call
// downloadSourceFunc, and the built binary must actually run and print what
// the source says.
func TestBuildWithGoUsesExistingCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("go mod tidy/go build; skipped under -short")
	}
	root := repoRoot(t)
	restore := swapSourceDir(t, func() (string, error) { return root, nil })
	defer restore()

	called := false
	origDownload := downloadSourceFunc
	downloadSourceFunc = func(string) error { called = true; return nil }
	defer func() { downloadSourceFunc = origDownload }()

	dir := t.TempDir()
	src := filepath.Join(dir, "hello.nako3")
	if err := os.WriteFile(src, []byte("「こんにちは、Goビルド！」と表示。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "hello")

	res := buildWithGo(src, out)
	if called {
		t.Fatal("既存のチェックアウトがあるのにダウンロード関数が呼ばれた")
	}
	if !res.OK {
		t.Fatalf("ビルドに失敗した: %s", res.Error)
	}
	if res.Downloaded {
		t.Fatal("ダウンロードしていないのに Downloaded=true になっている")
	}

	cmd := exec.Command(res.Path)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ビルドした実行ファイルが動かない: %v\n%s", err, outBytes)
	}
	got := string(outBytes)
	want := "こんにちは、Goビルド！\n"
	if got != want {
		t.Fatalf("出力が違う: got %q want %q", got, want)
	}
}

// swapSourceDir replaces sourceDirForGoBuild for the duration of a test.
func swapSourceDir(t *testing.T, fn func() (string, error)) func() {
	t.Helper()
	orig := sourceDirForGoBuild
	sourceDirForGoBuild = fn
	return func() { sourceDirForGoBuild = orig }
}

// TestDownloadSourceExtractsAndStripsTopDir checks the zip-handling logic
// against a small in-memory zip shaped like a GitHub codeload archive
// (everything under one "<repo>-<branch>/" folder) — no real network call.
func TestDownloadSourceExtractsAndStripsTopDir(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "nadesiko3go-master/go.mod", "module github.com/kujirahand/nadesiko3go\n")
	writeZipFile(t, zw, "nadesiko3go-master/pkg/runtime/runtime.go", "package runtime\n")
	writeZipFile(t, zw, "nadesiko3go-master/README.md", "# なでしこ3\n")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	restore := swapZipURL(t, server.URL)
	defer restore()

	dest := t.TempDir()
	dest = filepath.Join(dest, "nadesiko3go") // まだ存在しないフォルダへ展開する
	if err := downloadSource(dest); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"go.mod", "pkg/runtime/runtime.go", "README.md"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Fatalf("%s が展開されていない: %v", rel, err)
		}
	}
	if !isValidSourceCheckout(dest) {
		t.Fatal("展開結果が有効なチェックアウトと認識されない")
	}
}

// TestBuildWithGoDownloadsRealSource is the real thing: an empty folder, no
// checkout anywhere nearby, so buildWithGo must reach the actual GitHub URL,
// extract it, and build with it — exactly what a distributed gonako-gui with
// no source next to it does the first time someone uses this menu item.
// Opt-in (network + a few seconds of go build) — GOGEN_GOBUILD_NET=1.
func TestBuildWithGoDownloadsRealSource(t *testing.T) {
	if os.Getenv("GOGEN_GOBUILD_NET") == "" {
		t.Skip("set GOGEN_GOBUILD_NET=1 to actually download from GitHub and build with it")
	}
	empty := filepath.Join(t.TempDir(), "nadesiko3go")
	restore := swapSourceDir(t, func() (string, error) { return empty, nil })
	defer restore()

	dir := t.TempDir()
	src := filepath.Join(dir, "hello.nako3")
	if err := os.WriteFile(src, []byte("「こんにちは、Goビルド！」と表示。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "hello")

	res := buildWithGo(src, out)
	if !res.OK {
		t.Fatalf("ビルドに失敗した: %s", res.Error)
	}
	if !res.Downloaded {
		t.Fatal("ダウンロードしたはずなのに Downloaded=false")
	}
	outBytes, err := exec.Command(res.Path).CombinedOutput()
	if err != nil {
		t.Fatalf("ビルドした実行ファイルが動かない: %v\n%s", err, outBytes)
	}
	if string(outBytes) != "こんにちは、Goビルド！\n" {
		t.Fatalf("出力が違う: %q", outBytes)
	}
}

func writeZipFile(t *testing.T, zw *zip.Writer, name, content string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

// swapZipURL points sourceRepoZipURL at a local test server. Package-level
// const cannot be reassigned, so downloadSource reads a variable instead in
// tests via this indirection — see zipURLForTest below.
func swapZipURL(t *testing.T, url string) func() {
	t.Helper()
	orig := zipURLOverride
	zipURLOverride = url
	return func() { zipURLOverride = orig }
}
