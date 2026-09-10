//go:build windows

package nodelib

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func shellCommand(line string) *exec.Cmd {
	commandProcessor := os.Getenv("ComSpec")
	if commandProcessor == "" {
		commandProcessor = "cmd.exe"
	}

	cmd := exec.Command(commandProcessor)
	// cmd.exeはGo標準のWindows引数引用規則と互換ではないため、
	// 引用符を含むコマンド文字列は加工させずに直接渡す。
	cmd.Args = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`"%s" /d /s /c "%s"`, commandProcessor, line),
	}
	return cmd
}
