//go:build !windows

package main

import "os/exec"

// hideWindow does nothing on non-Windows platforms.
func hideWindow(cmd *exec.Cmd) {}
