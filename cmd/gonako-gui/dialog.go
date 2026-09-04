package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// showOpenFileDialog opens an OS native open file dialog.
// It returns the selected file path, or an empty string if canceled by user.
func showOpenFileDialog(defaultDir string) (string, error) {
	if defaultDir == "" {
		defaultDir = getDesktopDir()
	}
	absDir, _ := filepath.Abs(defaultDir)

	switch runtime.GOOS {
	case "darwin":
		return openDialogDarwin(absDir)
	case "windows":
		return openDialogWindows(absDir)
	default:
		return openDialogLinux(absDir)
	}
}

// showSaveFileDialog opens an OS native save file dialog.
// It returns the selected file path, or an empty string if canceled by user.
func showSaveFileDialog(defaultDir, defaultName string) (string, error) {
	if defaultDir == "" {
		defaultDir = getDesktopDir()
	}
	if defaultName == "" {
		defaultName = "新規プログラム.nako3"
	}
	absDir, _ := filepath.Abs(defaultDir)

	switch runtime.GOOS {
	case "darwin":
		return saveDialogDarwin(absDir, defaultName)
	case "windows":
		return saveDialogWindows(absDir, defaultName)
	default:
		return saveDialogLinux(absDir, defaultName)
	}
}

// macOS (osascript / AppleScript)
func openDialogDarwin(defaultDir string) (string, error) {
	// try with default location
	script := fmt.Sprintf(`try
tell application (path to frontmost application as text)
    set f to choose file with prompt "ファイルを開く" default location POSIX file %q
    return POSIX path of f
end tell
on error number -128
    return ""
end try`, defaultDir)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		// fallback without default location
		fallback := `try
tell application (path to frontmost application as text)
    set f to choose file with prompt "ファイルを開く"
    return POSIX path of f
end tell
on error number -128
    return ""
end try`
		cmd2 := exec.Command("osascript", "-e", fallback)
		out2, err2 := cmd2.Output()
		if err2 != nil {
			return "", err2
		}
		return strings.TrimSpace(string(out2)), nil
	}
	return strings.TrimSpace(string(out)), nil
}

func saveDialogDarwin(defaultDir, defaultName string) (string, error) {
	script := fmt.Sprintf(`try
tell application (path to frontmost application as text)
    set f to choose file name with prompt "名前を付けて保存" default name %q default location POSIX file %q
    return POSIX path of f
end tell
on error number -128
    return ""
end try`, defaultName, defaultDir)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		fallback := fmt.Sprintf(`try
tell application (path to frontmost application as text)
    set f to choose file name with prompt "名前を付けて保存" default name %q
    return POSIX path of f
end tell
on error number -128
    return ""
end try`, defaultName)
		cmd2 := exec.Command("osascript", "-e", fallback)
		out2, err2 := cmd2.Output()
		if err2 != nil {
			return "", err2
		}
		res := strings.TrimSpace(string(out2))
		if res != "" && filepath.Ext(res) == "" {
			res += ".nako3"
		}
		return res, nil
	}
	res := strings.TrimSpace(string(out))
	if res != "" && filepath.Ext(res) == "" {
		res += ".nako3"
	}
	return res, nil
}

// Windows (PowerShell / System.Windows.Forms)
func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func openDialogWindows(defaultDir string) (string, error) {
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.OpenFileDialog
$f.Filter = 'なでしこプログラム (*.nako3;*.nako)|*.nako3;*.nako|テキストファイル (*.txt)|*.txt|すべてのファイル (*.*)|*.*'
$f.Title = 'ファイルを開く'
$f.InitialDirectory = '%s'
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
    [Console]::Write($f.FileName)
}
`, escapePS(defaultDir))

	cmd := exec.Command("powershell", "-Sta", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func saveDialogWindows(defaultDir, defaultName string) (string, error) {
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.SaveFileDialog
$f.Filter = 'なでしこプログラム (*.nako3)|*.nako3|テキストファイル (*.txt)|*.txt|すべてのファイル (*.*)|*.*'
$f.DefaultExt = 'nako3'
$f.AddExtension = $true
$f.FileName = '%s'
$f.InitialDirectory = '%s'
$f.Title = '名前を付けて保存'
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
    [Console]::Write($f.FileName)
}
`, escapePS(defaultName), escapePS(defaultDir))

	cmd := exec.Command("powershell", "-Sta", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	res := strings.TrimSpace(string(out))
	if res != "" && filepath.Ext(res) == "" {
		res += ".nako3"
	}
	return res, nil
}

// Linux (zenity / kdialog)
func openDialogLinux(defaultDir string) (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--title=ファイルを開く",
			"--file-filter=なでしこプログラム (*.nako3, *.nako) | *.nako3 *.nako",
			"--file-filter=すべてのファイル | *"}
		if defaultDir != "" {
			args = append(args, "--filename="+defaultDir+"/")
		}
		cmd := exec.Command("zenity", args...)
		out, err := cmd.Output()
		if err != nil {
			return "", nil // canceled or closed
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command("kdialog", "--getopenfilename", defaultDir,
			"*.nako3 *.nako|なでしこプログラム (*.nako3 *.nako)\n*|すべてのファイル (*)")
		out, err := cmd.Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("zenity または kdialog が見つかりません")
}

func saveDialogLinux(defaultDir, defaultName string) (string, error) {
	defaultPath := filepath.Join(defaultDir, defaultName)

	if _, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--save", "--confirm-overwrite",
			"--title=名前を付けて保存",
			"--filename=" + defaultPath,
			"--file-filter=なでしこプログラム (*.nako3) | *.nako3",
			"--file-filter=すべてのファイル | *"}
		cmd := exec.Command("zenity", args...)
		out, err := cmd.Output()
		if err != nil {
			return "", nil // canceled or closed
		}
		res := strings.TrimSpace(string(out))
		if res != "" && filepath.Ext(res) == "" {
			res += ".nako3"
		}
		return res, nil
	}

	if _, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command("kdialog", "--getsavefilename", defaultPath,
			"*.nako3|なでしこプログラム (*.nako3)\n*|すべてのファイル (*)")
		out, err := cmd.Output()
		if err != nil {
			return "", nil
		}
		res := strings.TrimSpace(string(out))
		if res != "" && filepath.Ext(res) == "" {
			res += ".nako3"
		}
		return res, nil
	}

	return "", fmt.Errorf("zenity または kdialog が見つかりません")
}
