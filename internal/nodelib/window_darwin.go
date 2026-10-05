//go:build darwin

package nodelib

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// macOS版は osascript で System Events を操作する。
// ウィンドウのハンドルは「プロセスID*1000 + そのプロセス内のウィンドウ番号(1始まり)」。
// 窓の並びが変わると指す先も変わるため、取得したらすぐ使うこと。
// システム設定の「プライバシーとセキュリティ」→「アクセシビリティ」の許可が必要。
const macWindowGuide = "システム設定の「プライバシーとセキュリティ」→「アクセシビリティ」で、このプログラムを許可してください"

func runOsascript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascriptに失敗しました: %s（%s）", strings.TrimSpace(string(out)), macWindowGuide)
	}
	return strings.TrimSpace(string(out)), nil
}

type systemWindowDriver struct{}

// splitMacHandle はハンドルをプロセスIDとウィンドウ番号に分ける。
func splitMacHandle(h int64) (pid, idx int64) { return h / 1000, h % 1000 }

func (systemWindowDriver) List() ([]windowInfo, error) {
	out, err := runOsascript(`set out to ""
tell application "System Events"
	repeat with p in (processes where background only is false)
		set pid to unix id of p
		set i to 0
		try
			repeat with w in windows of p
				set i to i + 1
				try
					set out to out & (pid * 1000 + i) & tab & (name of w) & linefeed
				end try
			end repeat
		end try
	end repeat
end tell
return out`)
	if err != nil {
		return nil, err
	}
	var list []windowInfo
	for _, line := range strings.Split(out, "\n") {
		hs, title, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || title == "" || title == "missing value" {
			continue
		}
		h, perr := strconv.ParseInt(hs, 10, 64)
		if perr != nil {
			continue
		}
		list = append(list, windowInfo{Handle: h, Title: title})
	}
	return list, nil
}

// macWindowScript は対象ウィンドウに対する処理bodyを包んだAppleScriptを作る。
func macWindowScript(h int64, body string) string {
	pid, idx := splitMacHandle(h)
	return fmt.Sprintf(`tell application "System Events"
	set p to first process whose unix id is %d
	set w to window %d of p
	%s
end tell`, pid, idx, body)
}

func (systemWindowDriver) Activate(h int64) error {
	_, err := runOsascript(macWindowScript(h, `set frontmost of p to true
	perform action "AXRaise" of w`))
	return err
}

func (systemWindowDriver) Move(h int64, x, y int) error {
	_, err := runOsascript(macWindowScript(h, fmt.Sprintf("set position of w to {%d, %d}", x, y)))
	return err
}

func (systemWindowDriver) Size(h int64) (int, int, error) {
	out, err := runOsascript(macWindowScript(h, "return (item 1 of (get size of w) as text) & \",\" & (item 2 of (get size of w) as text)"))
	if err != nil {
		return 0, 0, err
	}
	ws, hs, ok := strings.Cut(out, ",")
	w, e1 := strconv.Atoi(strings.TrimSpace(ws))
	hgt, e2 := strconv.Atoi(strings.TrimSpace(hs))
	if !ok || e1 != nil || e2 != nil {
		return 0, 0, fmt.Errorf("ウィンドウのサイズを取得できませんでした: %q", out)
	}
	return w, hgt, nil
}

func (systemWindowDriver) Resize(h int64, w, hgt int) error {
	_, err := runOsascript(macWindowScript(h, fmt.Sprintf("set size of w to {%d, %d}", w, hgt)))
	return err
}
