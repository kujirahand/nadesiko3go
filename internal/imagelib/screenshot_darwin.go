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
	if id, ok := findWindowIDDarwin(target); ok {
		// -l はウィンドウID自身の内容を撮影するため、-R（矩形指定）と違って
		// 対象ウィンドウの手前に別のウィンドウが重なっていても写り込まない。
		args = append(args, "-l", strconv.Itoa(id))
	}
	args = append(args, name)
	out, err := exec.Command("/usr/sbin/screencapture", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("スクリーンショット撮影に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return decodePNGFile(name)
}

// findWindowIDDarwin は、タイトルにtargetを含む最初のウィンドウのCGWindowIDを
// 返す。System Eventsのアクセシビリティ属性 AXWindowNumber はCGWindowIDと
// 同じ値を返すため、これをscreencapture -lへそのまま渡せる。
// targetが空または「全体」のとき、および該当するウィンドウが見つからない
// ときはokがfalseになり、呼び出し側は画面全体を撮る。
func findWindowIDDarwin(target string) (int, bool) {
	target = strings.TrimSpace(target)
	if target == "" || target == "全体" {
		return 0, false
	}
	script := fmt.Sprintf(`
tell application "System Events"
	repeat with proc in application processes
		try
			repeat with w in windows of proc
				if (name of w as string) contains %s then
					return value of attribute "AXWindowNumber" of w
				end if
			end repeat
		end try
	end repeat
end tell
return ""
`, appleScriptQuote(target))
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return id, true
}

// appleScriptQuote はAppleScriptの文字列リテラルとして安全な形へエスケープする。
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
