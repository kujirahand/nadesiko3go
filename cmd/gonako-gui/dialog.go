package main

import "github.com/kujirahand/nadesiko3go/internal/guilib"

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
	// 拡張子フィルターは表示しない。空欄ならSaveFileDialogが.nako3を補い、
	// 利用者が.phpなどを明示した場合はその拡張子をそのまま使う。
	return guilib.SaveFileDialog(defaultDir, defaultName, "")
}
