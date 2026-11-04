package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
// (everything under one "<repo>-<branch>/" folder) — no network or listener.
func TestDownloadSourceExtractsAndStripsTopDir(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "nadesiko3go-master/go.mod", "module github.com/kujirahand/nadesiko3go\n")
	writeZipFile(t, zw, "nadesiko3go-master/pkg/runtime/runtime.go", "package runtime\n")
	writeZipFile(t, zw, "nadesiko3go-master/README.md", "# なでしこ3\n")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	dest = filepath.Join(dest, "nadesiko3go") // まだ存在しないフォルダへ展開する
	if err := extractDownloadedSource(bytes.NewReader(buf.Bytes()), dest); err != nil {
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

// replacePathToken は goModContent が生成した go.mod から replace 先の
// 引用文字列を取り出す。引用を解いた値の検証に使う。
func replacePathToken(t *testing.T, content string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "replace ") {
			continue
		}
		_, token, ok := strings.Cut(line, " => ")
		if !ok {
			t.Fatalf("replace 行に => がない: %q", line)
		}
		return strings.TrimSpace(token)
	}
	t.Fatalf("replace 行がない: %q", content)
	return ""
}

// TestGoModContentReplacePathParsedByToolchain は、生成した go.mod を
// 実際の Go ツールチェイン (go mod edit -json) で解析し、replace 先が
// 配置パス (パス区切りを「/」にそろえた形) と一致することを確かめる
// (Issue #352)。標準ライブラリだけで検証するため x/mod には依存しない。
func TestGoModContentReplacePathParsedByToolchain(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go コマンドがない")
	}
	base := t.TempDir()
	cases := []struct{ name, dir string }{
		{"空白なし", filepath.Join(base, "checkout")},
		{"空白入り", filepath.Join(base, "source checkout")},
		{"引用符入り", filepath.Join(base, `a"b`)},
		{"空白と日本語", filepath.Join(base, "日本語 フォルダ")},
		{"NBSP入り", filepath.Join(base, "a\u00a0b")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent(tc.dir)), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "mod", "edit", "-json")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go mod edit -json が生成 go.mod を解析できない: %v\n%s\ngo.mod:\n%s", err, out, goModContent(tc.dir))
			}
			var mod struct {
				Replace []struct {
					Old struct{ Path string }
					New struct{ Path string }
				}
			}
			if err := json.Unmarshal(out, &mod); err != nil {
				t.Fatal(err)
			}
			if len(mod.Replace) != 1 {
				t.Fatalf("replace の数 = %d, want 1", len(mod.Replace))
			}
			if mod.Replace[0].Old.Path != "github.com/kujirahand/nadesiko3go" {
				t.Fatalf("replace 元 = %q", mod.Replace[0].Old.Path)
			}
			// goModContent は区切りを「/」にそろえるので、解析結果もそろう。
			if want := filepath.ToSlash(tc.dir); mod.Replace[0].New.Path != want {
				t.Fatalf("replace 先 = %q, want %q", mod.Replace[0].New.Path, want)
			}
		})
	}
}

// TestGoModContentEscapesReplacePath は、実行中の OS ではローカルパス
// として解釈できない形式 (Windows のドライブパスなど) でも、引用した
// replace 先が文字列として往復し、元のパス (区切りをそろえた形) に
// 戻ることを確かめる。引用符やバックスラッシュのエスケープ漏れは、
// 単にダブルクォートで囲むだけの実装ではここで失敗する。
func TestGoModContentEscapesReplacePath(t *testing.T) {
	cases := []struct{ name, dir string }{
		{"Windowsの空白と引用符", `C:\Users\My Documents\a"quoted"\repo`},
		{"Windowsの空白のみ", `D:\source checkout\nadesiko3go`},
		{"Unixの空白と引用符", `/home/user/source checkout/a"b/repo`},
		{"空白なし", `/home/user/nadesiko3go`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := replacePathToken(t, goModContent(tc.dir))
			got, err := strconv.Unquote(token)
			if err != nil {
				t.Fatalf("replace 先 %q が Go 文字列として解釈できない: %v", token, err)
			}
			if want := filepath.ToSlash(tc.dir); got != want {
				t.Fatalf("復元したパス = %q, want %q", got, want)
			}
		})
	}
}

// TestBuildGoSourceWithSpacedReplacePath は、空白入りの配置パスにある
// 最小の置換先モジュールに対して buildGoSource (go mod tidy + go build)
// が通ることを確かめる (Issue #352)。置換先は依存のない最小構成にして、
// ネットワークなしで replace の解決だけを検証する。
func TestBuildGoSourceWithSpacedReplacePath(t *testing.T) {
	if testing.Short() {
		t.Skip("go mod tidy/go build; skipped under -short")
	}
	for _, tc := range []struct{ name, dirName string }{
		{"空白なし", "checkout"},
		{"空白入り", "source checkout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srcDir := filepath.Join(t.TempDir(), tc.dirName)
			runtimeDir := filepath.Join(srcDir, "pkg", "runtime")
			if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(srcDir, "go.mod"),
				[]byte("module github.com/kujirahand/nadesiko3go\n\ngo 1.23\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(runtimeDir, "runtime.go"),
				[]byte("package runtime\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			src := []byte("package main\n\nimport _ \"github.com/kujirahand/nadesiko3go/pkg/runtime\"\n\nfunc main() {}\n")
			outPath := filepath.Join(t.TempDir(), "gonako-test")
			built, err := buildGoSource(src, srcDir, outPath)
			if err != nil {
				t.Fatalf("buildGoSource が失敗した: %v", err)
			}
			if _, err := os.Stat(built); err != nil {
				t.Fatalf("ビルド成果物がない: %v", err)
			}
		})
	}
}

// TestBuildWithGoSpacedCheckoutPath は、本物のチェックアウトを空白入り
// パス経由 (シンボリックリンク) で指して buildWithGo がビルドでき、
// 成果物が動くことを確かめる (Issue #352 の受け入れ条件)。リンクを
// 作れない環境 (Windows の権限など) ではスキップする。
func TestBuildWithGoSpacedCheckoutPath(t *testing.T) {
	if testing.Short() {
		t.Skip("go mod tidy/go build; skipped under -short")
	}
	root := repoRoot(t)
	link := filepath.Join(t.TempDir(), "source checkout", "nadesiko3go")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("シンボリックリンクを作れないためスキップ: %v", err)
	}
	restore := swapSourceDir(t, func() (string, error) { return link, nil })
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
		t.Fatalf("空白入りパス経由のビルドに失敗した: %s", res.Error)
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
