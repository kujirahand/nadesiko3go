//go:build darwin

package imagelib

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// captureScreen はmacOSで画面全体、または指定タイトルを含む最初のウィンドウを
// 撮影する。CGOを使わず、標準搭載の screencapture コマンドへシェルアウトする
// （キー送信のosascriptフォールバックと同じ方針）。
func captureScreen(target string) (*image.RGBA, error) {
	tmp, err := os.CreateTemp("", "gonako-screenshot-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)

	args := []string{"-x"}
	if rect, ok := findWindowRectDarwin(target); ok {
		args = append(args, "-R", fmt.Sprintf("%d,%d,%d,%d", rect[0], rect[1], rect[2], rect[3]))
	}
	args = append(args, name)
	out, err := exec.Command("/usr/sbin/screencapture", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("スクリーンショット撮影に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return decodePNGFile(name)
}

// findWindowRectDarwin は、タイトルにtargetを含む最初のウィンドウの位置と
// サイズを[x, y, w, h]で返す。targetが空または「全体」のとき、および該当する
// ウィンドウが見つからないときはokがfalseになり、呼び出し側は画面全体を撮る。
func findWindowRectDarwin(target string) ([4]int, bool) {
	target = strings.TrimSpace(target)
	if target == "" || target == "全体" {
		return [4]int{}, false
	}
	script := fmt.Sprintf(`
tell application "System Events"
	repeat with proc in application processes
		try
			repeat with w in windows of proc
				if (name of w as string) contains %s then
					set p to position of w
					set s to size of w
					return ((item 1 of p) as string) & "," & ((item 2 of p) as string) & "," & ((item 1 of s) as string) & "," & ((item 2 of s) as string)
				end if
			end repeat
		end try
	end repeat
end tell
return ""
`, appleScriptQuote(target))
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return [4]int{}, false
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(fields) != 4 {
		return [4]int{}, false
	}
	var rect [4]int
	for i, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return [4]int{}, false
		}
		rect[i] = n
	}
	return rect, true
}

// appleScriptQuote はAppleScriptの文字列リテラルとして安全な形へエスケープする。
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
