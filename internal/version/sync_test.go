package version

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestFilesAreInSync は `just version-update --check` を実行し、
// このパッケージの定数とインストーラー・READMEなどのバージョン表記が
// ズレていないことを保証する。CIでの更新漏れ検出用。
func TestFilesAreInSync(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller に失敗しました")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	cmd := exec.Command("go", "run", "./scripts/version-update.go", "--check")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("バージョン番号にズレがあります。`just version-update` を実行してください:\n%s", out)
	}
}
