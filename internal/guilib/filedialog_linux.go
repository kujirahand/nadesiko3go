//go:build !darwin && !windows

package guilib

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func openFileDialogPlatform(defaultDir, extension string) (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--title=ファイルを開く", "--filename=" + defaultDir + "/"}
		if extension != "" {
			args = append(args, "--file-filter=対象ファイル | *"+extension)
		}
		args = append(args, "--file-filter=すべてのファイル | *")
		return runLinuxDialog("zenity", args...)
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		filter := "*|すべてのファイル (*)"
		if extension != "" {
			filter = "*" + extension + "|対象ファイル (*" + extension + ")\n" + filter
		}
		return runLinuxDialog("kdialog", "--getopenfilename", defaultDir, filter)
	}
	return "", fmt.Errorf("zenity または kdialog が見つかりません")
}

func saveFileDialogPlatform(defaultDir, defaultName, extension string) (string, error) {
	defaultPath := filepath.Join(defaultDir, defaultName)
	if _, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--save", "--confirm-overwrite", "--title=名前を付けて保存", "--filename=" + defaultPath}
		if extension != "" {
			args = append(args, "--file-filter=対象ファイル | *"+extension)
		}
		args = append(args, "--file-filter=すべてのファイル | *")
		return runLinuxDialog("zenity", args...)
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		filter := "*|すべてのファイル (*)"
		if extension != "" {
			filter = "*" + extension + "|対象ファイル (*" + extension + ")\n" + filter
		}
		return runLinuxDialog("kdialog", "--getsavefilename", defaultPath, filter)
	}
	return "", fmt.Errorf("zenity または kdialog が見つかりません")
}

func folderDialogPlatform(defaultDir string) (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		return runLinuxDialog("zenity", "--file-selection", "--directory", "--title=フォルダを選択", "--filename="+defaultDir+"/")
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		return runLinuxDialog("kdialog", "--getexistingdirectory", defaultDir)
	}
	return "", fmt.Errorf("zenity または kdialog が見つかりません")
}

func runLinuxDialog(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
