//go:build !windows

package nodelib

import "os/exec"

func shellCommand(line string) *exec.Cmd {
	return exec.Command("sh", "-c", line)
}
