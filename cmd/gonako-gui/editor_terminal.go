package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// terminalSpec はターミナル起動コマンドの組み立て結果です。
// Dir は起動するプロセスの作業フォルダ（空なら指定なし）です。
type terminalSpec struct {
	Name string
	Args []string
	Dir  string
}

// terminalSpecFor はOSごとのターミナル起動コマンドを組み立てます (#325)。
// lookPath はターミナル本体の有無を確認するために使います（テストで差し替え可能）。
func terminalSpecFor(goos, dir string, lookPath func(string) (string, error)) (terminalSpec, error) {
	switch goos {
	case "darwin":
		// Terminal.app を対象フォルダで開く
		return terminalSpec{Name: "open", Args: []string{"-a", "Terminal", dir}}, nil
	case "windows":
		// Windows Terminal があればそれを使い、なければ PowerShell を開く。
		// どちらも作業フォルダは起動プロセス側へ渡すので、パスをコマンド文字列へ埋め込まない。
		if _, err := lookPath("wt.exe"); err == nil {
			return terminalSpec{Name: "wt.exe", Args: []string{"-d", dir}}, nil
		}
		return terminalSpec{Name: "powershell.exe", Args: []string{"-NoExit"}, Dir: dir}, nil
	case "linux":
		candidates := []terminalSpec{
			{Name: "gnome-terminal", Args: []string{"--working-directory=" + dir}},
			{Name: "konsole", Args: []string{"--workdir", dir}},
			{Name: "xfce4-terminal", Args: []string{"--working-directory=" + dir}},
			{Name: "x-terminal-emulator", Dir: dir},
			{Name: "xterm", Dir: dir},
		}
		for _, candidate := range candidates {
			if _, err := lookPath(candidate.Name); err == nil {
				return candidate, nil
			}
		}
		return terminalSpec{}, fmt.Errorf("利用できるターミナルが見つかりません")
	default:
		return terminalSpec{}, fmt.Errorf("このOSではターミナルを開けません: %s", goos)
	}
}

// openTerminal は dirPath のフォルダでOSのターミナルを開きます (#325)。
// ファイルが渡された場合は、その親フォルダで開きます。
func openTerminal(dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		absPath = dirPath
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		absPath = filepath.Dir(absPath)
	}
	spec, err := terminalSpecFor(runtime.GOOS, absPath, exec.LookPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(spec.Name, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	return cmd.Start()
}
