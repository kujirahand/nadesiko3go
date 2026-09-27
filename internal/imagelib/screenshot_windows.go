//go:build windows

package imagelib

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
)

// captureScreen はWindowsで画面全体、または指定タイトルを含む最初のウィンドウを
// 撮影する。CGOを使わず、PowerShell（System.Drawing / System.Windows.Forms）へ
// シェルアウトして実現する（キー送信のWindows実装がSendInputをsyscallで直接
// 呼んでいるのに対し、ここでは既存プロセスのウィンドウ矩形取得までを含めて
// PowerShellへ委ねたほうが単純なため）。
func captureScreen(target string) (*image.RGBA, error) {
	tmp, err := os.CreateTemp("", "gonako-screenshot-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)

	target = strings.TrimSpace(target)
	var script string
	if target == "" || target == "全体" {
		script = fmt.Sprintf(screenshotPSFullScript, psEscapeSingleQuoted(name))
	} else {
		script = fmt.Sprintf(screenshotPSWindowScript, psEscapeDoubleQuoted(target), psEscapeSingleQuoted(name))
	}
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("スクリーンショット撮影に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return decodePNGFile(name)
}

// psEscapeSingleQuoted はPowerShellの単一引用符文字列として安全な形へエスケープする。
func psEscapeSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// psEscapeDoubleQuoted はPowerShellの二重引用符文字列として安全な形へエスケープする。
func psEscapeDoubleQuoted(s string) string {
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, `"`, "`\"")
	s = strings.ReplaceAll(s, "$", "`$")
	return s
}

// screenshotPSFullScript は画面全体をPNGへ保存するPowerShellスクリプト。
// %s は保存先ファイルパス。
const screenshotPSFullScript = `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$b = [System.Windows.Forms.SystemInformation]::VirtualScreen
$bmp = New-Object System.Drawing.Bitmap($b.Width, $b.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
`

// screenshotPSWindowScript は、タイトルに指定文字列を含む最初のウィンドウを
// 撮影する（見つからなければ画面全体）PowerShellスクリプト。
// 1つ目の%sはウィンドウタイトル（部分一致）、2つ目の%sは保存先ファイルパス。
const screenshotPSWindowScript = `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class GonakoScreenshotWin {
  [StructLayout(LayoutKind.Sequential)]
  public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
  [DllImport("user32.dll")]
  public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
}
"@
$procs = Get-Process | Where-Object { $_.MainWindowTitle -like "*%s*" -and $_.MainWindowHandle -ne [IntPtr]::Zero }
if ($procs.Count -eq 0) {
  $b = [System.Windows.Forms.SystemInformation]::VirtualScreen
  $x = $b.X; $y = $b.Y; $w = $b.Width; $h = $b.Height
} else {
  $rect = New-Object GonakoScreenshotWin+RECT
  [GonakoScreenshotWin]::GetWindowRect($procs[0].MainWindowHandle, [ref]$rect) | Out-Null
  $x = $rect.Left; $y = $rect.Top; $w = $rect.Right - $rect.Left; $h = $rect.Bottom - $rect.Top
}
$bmp = New-Object System.Drawing.Bitmap($w, $h)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($x, $y, 0, 0, (New-Object System.Drawing.Size($w, $h)))
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
`
