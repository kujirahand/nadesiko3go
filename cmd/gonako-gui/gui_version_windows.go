//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// GUI サブシステムでは親コンソールや標準出力がない場合もある。
func outputGUIVersion(text string) {
	if stderrHandleVisible(os.Stdout) {
		fmt.Fprintln(os.Stdout, text)
		return
	}
	if writeParentConsole(text) {
		return
	}
	title, _ := windows.UTF16PtrFromString("gonako-gui")
	body, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
}
