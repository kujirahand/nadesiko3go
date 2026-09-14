//go:build !windows

package nodelib

import (
	"os"
	"path/filepath"
)

// desktopDir はデスクトップフォルダの絶対パスを返す（Windows以外）。
func desktopDir() string {
	dir, _ := os.UserHomeDir()
	return filepath.Join(dir, "Desktop")
}
