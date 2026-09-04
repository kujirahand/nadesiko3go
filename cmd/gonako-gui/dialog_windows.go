//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow sets the HideWindow flag to prevent flashing a console window on Windows.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
