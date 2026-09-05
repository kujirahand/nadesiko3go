package main

import (
	"path/filepath"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

// showOpenFileDialog opens the OS native open file dialog used by the editor.
func showOpenFileDialog(defaultDir string) (string, error) {
	if defaultDir == "" {
		defaultDir = getDesktopDir()
	}
	return guilib.OpenFileDialog(defaultDir, "")
}

// showSaveFileDialog opens the OS native save file dialog used by the editor.
func showSaveFileDialog(defaultDir, defaultName string) (string, error) {
	if defaultDir == "" {
		defaultDir = getDesktopDir()
	}
	if defaultName == "" {
		defaultName = "新規プログラム.nako3"
	}
	return guilib.SaveFileDialog(defaultDir, defaultName, filepath.Ext(defaultName))
}
