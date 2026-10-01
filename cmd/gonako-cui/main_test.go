package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
)

// TestHTMLBundleCUIRejects は、HTML種バンドルをgonako-cuiで実行したときに
// panicせず明示的なエラーメッセージを返すことを確認する（Issue #198）。
func TestHTMLBundleCUIRejects(t *testing.T) {
	if testing.Short() {
		t.Skip("go build が必要なので --short では飛ばす")
	}
	dir := t.TempDir()
	// リソースフォルダにindex.htmlを作成
	resDir := filepath.Join(dir, "html")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resDir, "index.html"), []byte("<html><body>test</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// gonako-cuiバイナリをビルド
	gonakoPath := filepath.Join(dir, "gonako-cui")
	buildCmd := exec.Command("go", "build", "-o", gonakoPath, ".")
	buildCmd.Dir = filepath.Join(findRepoRoot(t), "cmd", "gonako-cui")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build 失敗: %v\n%s", err, out)
	}

	// HTML種バンドルを作成（gonako-cuiをランタイムとして使用）
	htmlAppPath := filepath.Join(dir, "html_app")
	spec := bundle.Spec{
		Kind:        bundle.KindHTML,
		Entry:       "index.html",
		Title:       "テストHTMLアプリ",
		ResourceDir: resDir,
	}
	if err := bundle.BuildSpec(htmlAppPath, gonakoPath, spec); err != nil {
		t.Fatal(err)
	}

	// HTML種バンドルを実行してエラーになることを確認
	cmd := exec.Command(htmlAppPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("HTML種バンドルを実行してもエラーになりませんでした")
	}
	// panicではなく明示的なエラーメッセージが出ることを確認
	if strings.Contains(string(out), "panic") {
		t.Errorf("panicが発生しました（明示的なエラーになるべき）:\n%s", out)
	}
	if !strings.Contains(string(out), "GUI用") {
		t.Errorf("エラーメッセージに「GUI用」がありません:\n%s", out)
	}
}

// findRepoRoot はテスト実行中のリポジトリルートを返す。
func findRepoRoot(t *testing.T) string {
	t.Helper()
	// go.mod を探す
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つかりません")
		}
		dir = parent
	}
}
