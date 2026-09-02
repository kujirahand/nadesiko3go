package main

// 「Go言語でビルド」機能。開いているなでしこプログラムをGoソースに変換し
// (internal/gogen)、go build でそのままネイティブ実行ファイルにする
// (AGENTS.md §12・docs/gogen.md)。
//
// gogenが生成するコードは github.com/kujirahand/nadesiko3go/pkg/runtime
// だけに依存するが、それを go.mod の replace で指す先として、この
// リポジトリ自身のソースチェックアウトが要る。配布される gonako-gui は
// バイナリ単体なので、実行ファイルと同じフォルダに `nadesiko3go/` が
// 無ければ、GitHubの最新masterをダウンロードして展開する。

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/gogen"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// goBuildPlugins is what a program built through this menu item can call.
// It matches `gonako gengo` の既定値 と同じにしてあるが、guilib
// （『ウィンドウ作成』）は含めない。gogenがguilibを引き込むと、
// CLI版のgonakoまでcgo/WebViewへ依存してしまうため（→
// internal/gogen/plugins.go）。ウィンドウを開くプログラムは、この機能では
// なくエディタの「実行」か、VM実行（gonako run）を使う。
var goBuildPlugins = []string{"nodelib", "csvlib", "mathlib", "sqlitelib", "officelib", "pdflib", "imagelib"}

// sourceRepoZipURL is where the source is downloaded from when none is found
// next to the running executable.
const sourceRepoZipURL = "https://github.com/kujirahand/nadesiko3go/archive/refs/heads/master.zip"

// zipURLOverride lets a test point downloadSource at a local server instead
// of GitHub. Empty means use sourceRepoZipURL.
var zipURLOverride string

// GoBuildResult is the JSON structure returned to JavaScript.
type GoBuildResult struct {
	OK         bool   `json:"ok"`
	Path       string `json:"path,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Downloaded bool   `json:"downloaded,omitempty"`
	Error      string `json:"error,omitempty"`
}

// sourceDirForGoBuild names where this feature looks for (and, if missing,
// downloads) the なでしこ3 Go版 source: a "nadesiko3go" folder next to the
// running gonako-gui. A variable so a test can point it elsewhere.
var sourceDirForGoBuild = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "nadesiko3go"), nil
}

// downloadSourceFunc fetches the source into a folder. A variable so a test
// can fake the download instead of reaching GitHub.
var downloadSourceFunc = downloadSource

// buildWithGo compiles sourcePath (a .nako3 file) to Go and builds it into a
// native executable at outPath, downloading this project's own source next
// to the running binary first if it is not there already.
func buildWithGo(sourcePath, outPath string) GoBuildResult {
	code, err := os.ReadFile(sourcePath)
	if err != nil {
		return GoBuildResult{Error: fmt.Sprintf("ファイルを読み込めません: %v", err)}
	}

	registry, err := gogen.BuildRegistry(goBuildPlugins)
	if err != nil {
		return GoBuildResult{Error: err.Error()}
	}
	name := filepath.Base(sourcePath)
	tree, err := parser.ParseSource(string(code), name, registry.FuncList())
	if err != nil {
		return GoBuildResult{Error: err.Error()}
	}
	prog, err := compiler.Compile(tree, name, registry)
	if err != nil {
		return GoBuildResult{Error: err.Error()}
	}
	src, err := gogen.Generate(prog, gogen.Options{Plugins: goBuildPlugins})
	if err != nil {
		return GoBuildResult{Error: err.Error()}
	}

	srcDir, err := sourceDirForGoBuild()
	if err != nil {
		return GoBuildResult{Error: fmt.Sprintf("実行ファイルの場所が分かりません: %v", err)}
	}
	downloaded := false
	if !isValidSourceCheckout(srcDir) {
		if err := downloadSourceFunc(srcDir); err != nil {
			return GoBuildResult{Error: fmt.Sprintf("ソースコードをダウンロードできません: %v", err)}
		}
		downloaded = true
		if !isValidSourceCheckout(srcDir) {
			return GoBuildResult{Error: fmt.Sprintf("ダウンロードしたソースが不完全です: %s", srcDir)}
		}
	}

	out, err := buildGoSource(src, srcDir, outPath)
	if err != nil {
		return GoBuildResult{Error: err.Error()}
	}

	size := int64(0)
	if info, statErr := os.Stat(out); statErr == nil {
		size = info.Size()
	}
	return GoBuildResult{OK: true, Path: out, Size: size, Downloaded: downloaded}
}

// buildGoSource writes src into a throwaway module that replaces
// github.com/kujirahand/nadesiko3go with srcDir, then builds it.
func buildGoSource(src []byte, srcDir, outPath string) (string, error) {
	buildDir, err := os.MkdirTemp("", "gonako-gobuild-*")
	if err != nil {
		return "", fmt.Errorf("作業フォルダを作れません: %w", err)
	}
	defer os.RemoveAll(buildDir)

	if err := os.WriteFile(filepath.Join(buildDir, "main.go"), src, 0o644); err != nil {
		return "", fmt.Errorf("Goソースを書き出せません: %w", err)
	}
	goMod := "module gonakobuild\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + srcDir + "\n"
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return "", fmt.Errorf("go.modを書き出せません: %w", err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = buildDir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod tidyに失敗しました（Goツールチェインが必要です）: %v\n%s", err, out)
	}

	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return "", fmt.Errorf("出力先が分かりません: %w", err)
	}
	build := exec.Command("go", "build", "-o", absOut, ".")
	build.Dir = buildDir
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go buildに失敗しました:\n%s", out)
	}
	return absOut, nil
}

// isValidSourceCheckout reports whether dir looks like a nadesiko3go
// checkout gogen-built code can replace against.
func isValidSourceCheckout(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil || !strings.Contains(string(data), "module github.com/kujirahand/nadesiko3go") {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "pkg", "runtime"))
	return err == nil && info.IsDir()
}

// downloadSource fetches this project's own source from GitHub and unpacks
// it into destDir, so ビルドする側 needs nothing but network access and Go.
func downloadSource(destDir string) error {
	url := sourceRepoZipURL
	if zipURLOverride != "" {
		url = zipURLOverride
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "nadesiko3go-src-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}

	r, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return fmt.Errorf("ダウンロードしたzipを開けません: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	for _, f := range r.File {
		// GitHubのzipballは "nadesiko3go-master/" というトップフォルダを
		// 持つので、それを取り除いて destDir 直下に展開する。
		rel := stripZipTopDir(f.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(destAbs, filepath.FromSlash(rel))
		// zip slip対策: 展開先が必ずdestAbsの内側になることを確かめる
		if target != destAbs && !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func stripZipTopDir(name string) string {
	_, rest, found := strings.Cut(name, "/")
	if !found {
		return ""
	}
	return rest
}

func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
