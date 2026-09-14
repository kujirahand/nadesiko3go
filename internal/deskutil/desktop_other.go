//go:build !windows

package deskutil

import (
	"os"
	"path/filepath"
)

// Dir はデスクトップフォルダの絶対パスを返す（Windows以外）。
func Dir() string {
	dir, _ := os.UserHomeDir()
	return filepath.Join(dir, "Desktop")
}
