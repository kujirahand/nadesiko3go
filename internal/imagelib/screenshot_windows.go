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
//
// タイトルは環境変数（GONAKO_SCREENSHOT_TITLE）でスクリプトへ渡す。
// スクリプト本文は固定文字列であり、タイトルをスクリプトソースへ文字列展開
// しないため、タイトルにバッククォートや引用符を含めてもPowerShellコードとして
// 解釈されない。
func captureScreen(target string) (*image.RGBA, error) {
	tmp, err := os.CreateTemp("", "gonako-screenshot-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", screenshotPSScript)
	cmd.Env = append(os.Environ(),
		"GONAKO_SCREENSHOT_TITLE="+strings.TrimSpace(target),
		"GONAKO_SCREENSHOT_OUTPATH="+name,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("スクリーンショット撮影に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return decodePNGFile(name)
}

// screenshotPSScript は画面全体、またはタイトルに部分一致する最初のウィンドウを
// 撮影するPowerShellスクリプト（固定文字列。ユーザー入力の文字列展開はしない）。
//
// タイトル検索は Get-Process の MainWindowTitle（プロセスにつき1つだけ）ではなく
// EnumWindows で全てのトップレベルウィンドウを列挙して比較するため、同じ
// プロセスが複数ウィンドウを開いていても目的のウィンドウを見つけられる。
// 撮影は対象ウィンドウのハンドルへ PrintWindow するため、画面上の矩形を
// 切り取る方式と違って手前に別ウィンドウが重なっていても写り込まない。
const screenshotPSScript = `
Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class GonakoScreenshotWin {
  [StructLayout(LayoutKind.Sequential)]
  public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")]
  public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  [DllImport("user32.dll")]
  public static extern int GetWindowTextLength(IntPtr hWnd);
  [DllImport("user32.dll", CharSet = CharSet.Unicode)]
  public static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);
  [DllImport("user32.dll")]
  public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")]
  public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
  [DllImport("user32.dll")]
  public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint nFlags);
  public static IntPtr FindByTitle(string part) {
    IntPtr found = IntPtr.Zero;
    EnumWindows(delegate(IntPtr hWnd, IntPtr lParam) {
      if (!IsWindowVisible(hWnd)) return true;
      int len = GetWindowTextLength(hWnd);
      if (len == 0) return true;
      StringBuilder sb = new StringBuilder(len + 1);
      GetWindowText(hWnd, sb, sb.Capacity);
      if (sb.ToString().IndexOf(part, StringComparison.OrdinalIgnoreCase) >= 0) {
        found = hWnd;
        return false;
      }
      return true;
    }, IntPtr.Zero);
    return found;
  }
}
"@

$outPath = $env:GONAKO_SCREENSHOT_OUTPATH
$title = $env:GONAKO_SCREENSHOT_TITLE
$hwnd = [IntPtr]::Zero
if ($title -and $title -ne "全体") {
  $hwnd = [GonakoScreenshotWin]::FindByTitle($title)
}

if ($hwnd -eq [IntPtr]::Zero) {
  Add-Type -AssemblyName System.Windows.Forms
  $b = [System.Windows.Forms.SystemInformation]::VirtualScreen
  $bmp = New-Object System.Drawing.Bitmap($b.Width, $b.Height)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
} else {
  $rect = New-Object GonakoScreenshotWin+RECT
  [GonakoScreenshotWin]::GetWindowRect($hwnd, [ref]$rect) | Out-Null
  $w = $rect.Right - $rect.Left
  $h = $rect.Bottom - $rect.Top
  $bmp = New-Object System.Drawing.Bitmap($w, $h)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $hdc = $g.GetHdc()
  [GonakoScreenshotWin]::PrintWindow($hwnd, $hdc, 0) | Out-Null
  $g.ReleaseHdc($hdc)
}
$bmp.Save($outPath, [System.Drawing.Imaging.ImageFormat]::Png)
`
