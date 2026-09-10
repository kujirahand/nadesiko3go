//go:build windows

package nodelib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellCommandWithQuotedExecutableAndArgument(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "space path.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\n<nul set /p =%~1\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	argument := filepath.Join(dir, "argument with spaces.nako")
	line := fmt.Sprintf(`"%s" "%s"`, scriptPath, argument)
	out, err := shellCommand(line).CombinedOutput()
	if err != nil {
		t.Fatalf("引用符付きコマンドを実行できません: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != argument {
		t.Fatalf("引数が正しく渡されていません: got %q, want %q", got, argument)
	}
}
