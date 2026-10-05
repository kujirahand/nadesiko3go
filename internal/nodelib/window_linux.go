//go:build linux

package nodelib

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Linux版は X11 の xdotool を呼ぶ（キー送信と同じ方針）。Waylandでは利用できない。

func runXdotool(args ...string) (string, error) {
	bin, err := exec.LookPath("xdotool")
	if err != nil {
		return "", errors.New(xdotoolGuide)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotoolに失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

type systemWindowDriver struct{}

func (systemWindowDriver) List() ([]windowInfo, error) {
	// タイトルが空でない表示中のウィンドウIDを列挙する
	out, err := runXdotool("search", "--onlyvisible", "--name", ".")
	if err != nil {
		// 該当なしは終了コード1になるため空として扱う
		if strings.Contains(err.Error(), xdotoolGuide) {
			return nil, err
		}
		return nil, nil
	}
	var list []windowInfo
	for _, f := range strings.Fields(out) {
		id, perr := strconv.ParseInt(f, 10, 64)
		if perr != nil {
			continue
		}
		title, terr := runXdotool("getwindowname", f)
		if terr != nil || title == "" {
			continue
		}
		list = append(list, windowInfo{Handle: id, Title: title})
	}
	return list, nil
}

func (systemWindowDriver) Activate(h int64) error {
	_, err := runXdotool("windowactivate", strconv.FormatInt(h, 10))
	return err
}

func (systemWindowDriver) Move(h int64, x, y int) error {
	_, err := runXdotool("windowmove", strconv.FormatInt(h, 10), strconv.Itoa(x), strconv.Itoa(y))
	return err
}

func (systemWindowDriver) Size(h int64) (int, int, error) {
	out, err := runXdotool("getwindowgeometry", "--shell", strconv.FormatInt(h, 10))
	if err != nil {
		return 0, 0, err
	}
	return parseShellGeometry(out)
}

// parseShellGeometry は xdotool getwindowgeometry --shell の出力から幅と高さを取り出す。
func parseShellGeometry(out string) (int, int, error) {
	w, h := -1, -1
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		switch k {
		case "WIDTH":
			w = n
		case "HEIGHT":
			h = n
		}
	}
	if w < 0 || h < 0 {
		return 0, 0, fmt.Errorf("ウィンドウのサイズを取得できませんでした")
	}
	return w, h, nil
}

func (systemWindowDriver) Resize(h int64, w, hgt int) error {
	_, err := runXdotool("windowsize", strconv.FormatInt(h, 10), strconv.Itoa(w), strconv.Itoa(hgt))
	return err
}
