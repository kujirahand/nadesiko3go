//go:build windows

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS

var (
	guiRunKernel32    = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = guiRunKernel32.NewProc("AttachConsole")
	procFreeConsole   = guiRunKernel32.NewProc("FreeConsole")
	procWriteConsoleW = guiRunKernel32.NewProc("WriteConsoleW")
)

// deliverGUIRunError は起動前エラーを、呼び出し元が見られる場所へ書く。
// 標準エラーがパイプ・ファイル・コンソールならそこへ書く。windowsgui で
// 標準エラーが捨てられているときは、親プロセスのコンソールへ接続して書く。
// どちらも無いときは false を返し、呼び出し元がメッセージボックスを出す。
func deliverGUIRunError(text string) bool {
	visible := stderrHandleVisible(os.Stderr)
	parentConsole := false
	if !visible {
		// 親コンソールへ書けた時点で届け先は標準エラー側になる。
		parentConsole = writeParentConsole(text)
	}
	if chooseGUIRunErrorDestination(visible, parentConsole) == guiRunErrorToDialog {
		return false
	}
	if parentConsole {
		return true
	}
	fmt.Fprintln(os.Stderr, text)
	return true
}

// stderrHandleVisible は f が呼び出し元から読める標準エラーかを返す。
func stderrHandleVisible(f *os.File) bool {
	if f == nil {
		return false
	}
	h := windows.Handle(f.Fd())
	if h == 0 || h == windows.InvalidHandle {
		return classifyStderrVisibility(false, 0, false)
	}
	fileType, err := windows.GetFileType(h)
	if err != nil {
		return classifyStderrVisibility(false, 0, false)
	}
	isConsole := false
	if fileType&^windows.FILE_TYPE_REMOTE == windows.FILE_TYPE_CHAR {
		var mode uint32
		isConsole = windows.GetConsoleMode(h, &mode) == nil
	}
	return classifyStderrVisibility(true, fileType, isConsole)
}

// writeParentConsole は親プロセスのコンソールへ UTF-16 で書く。
// 接続できなければ false。書いたあとはコンソールから離れる。
func writeParentConsole(text string) bool {
	r, _, _ := procAttachConsole.Call(uintptr(attachParentProcess))
	if r == 0 {
		return false
	}
	wrote := writeConsoleOut(text)
	_, _, _ = procFreeConsole.Call()
	return wrote
}

// writeConsoleOut は接続済みコンソールへ text を1行書く。
// コンソールのコードページを変えず、日本語の文面をそのまま出す。
func writeConsoleOut(text string) bool {
	f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	utf16, err := windows.UTF16FromString(text + "\n")
	if err != nil || len(utf16) <= 1 {
		return false
	}
	buf := utf16[:len(utf16)-1] // 末尾の NUL は書かない
	var written uint32
	r, _, _ := procWriteConsoleW.Call(
		f.Fd(),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	return r != 0
}

// platformShowGUIRunErrorDialog は Windows のメッセージボックスでエラーを見せる。
func platformShowGUIRunErrorDialog(text string) {
	title, err := windows.UTF16PtrFromString("gonako-gui")
	if err != nil {
		fmt.Fprintln(os.Stderr, text)
		return
	}
	body, err := windows.UTF16PtrFromString(text)
	if err != nil {
		fmt.Fprintln(os.Stderr, text)
		return
	}
	flags := uint32(windows.MB_OK | windows.MB_ICONERROR | windows.MB_SETFOREGROUND)
	if _, err := windows.MessageBox(0, body, title, flags); err != nil {
		fmt.Fprintln(os.Stderr, text)
	}
}
