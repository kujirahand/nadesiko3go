package doctest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installが生成する起動用ファイルへ、パス・引数・終了コードがそのまま渡ることを確認する。
func TestInstallLauncher(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "gonako runtime")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/gonako")
	build.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOROOT=") {
			build.Env = append(build.Env, entry)
		}
	}
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("ビルド: %v\n%s", err, output)
	}
	// installの生成物からパス・引数・終了コードがそのまま渡ることを確認する。
	toolSource := filepath.Join(dir, "空白 ' $ utility.nako3")
	if err := os.WriteFile(toolSource, []byte("コマンドラインをJSONエンコードして表示。\n7で強制終了。"), 0600); err != nil {
		t.Fatal(err)
	}
	install := exec.Command(binary, "install", toolSource, "--name", "test-tool")
	install.Dir = dir
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, output)
	}
	installed := filepath.Join(dir, "bin", "test-tool")
	var launch *exec.Cmd
	if runtime.GOOS == "windows" {
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			launch = exec.Command(pwsh, "-NoProfile", "-File", installed+".ps1", "空白 引数", "literal$(echo injected)")
		}
	} else {
		launch = exec.Command(installed, "空白 引数", "literal$(echo injected)")
	}
	if launch != nil {
		launch.Dir = dir
		output, err := launch.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 7 || strings.TrimSpace(string(output)) != `["空白 引数","literal$(echo injected)"]` {
			t.Fatalf("生成スクリプトの実行: %v\n%s", err, output)
		}
	}
}
