//go:build linux

package imagelib

import (
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
)

// captureScreen はLinuxで画面全体、または指定タイトルを含む最初のウィンドウを
// 撮影する。X11プロトコルを直接実装するのは手間が大きいため、キー送信の
// Linux実装と同じ方針で xdotool / import(ImageMagick) コマンドへシェルアウト
// する。Waylandでは動作しない。
func captureScreen(target string) (*image.RGBA, error) {
	if _, err := exec.LookPath("import"); err != nil {
		return nil, errors.New("スクリーンショット撮影にはImageMagick（importコマンド）が必要です。")
	}
	tmp, err := os.CreateTemp("", "gonako-screenshot-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)

	windowID := findWindowIDLinux(target)
	args := []string{"-window"}
	if windowID != "" {
		args = append(args, windowID)
	} else {
		args = append(args, "root")
	}
	args = append(args, name)
	out, err := exec.Command("import", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("スクリーンショット撮影に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return decodePNGFile(name)
}

// findWindowIDLinux は、タイトルにtargetを含む最初のウィンドウIDを返す。
// targetが空または「全体」のとき、xdotool未導入のとき、該当するウィンドウが
// 見つからないときは空文字を返し、呼び出し側は画面全体を撮る。
func findWindowIDLinux(target string) string {
	target = strings.TrimSpace(target)
	if target == "" || target == "全体" {
		return ""
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		return ""
	}
	out, err := exec.Command("xdotool", "search", "--name", target).Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
