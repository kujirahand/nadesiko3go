//go:build windows

package nodelib

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows版は user32 のウィンドウAPIを直接呼ぶ（CGOを使わない）。
var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procIsWindow             = user32.NewProc("IsWindow")
	procIsIconic             = user32.NewProc("IsIconic")
	procShowWindow           = user32.NewProc("ShowWindow")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
)

const (
	swRestore     = 9
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

type winRect struct{ Left, Top, Right, Bottom int32 }

type systemWindowDriver struct{}

func checkHWND(h int64) (uintptr, error) {
	if r, _, _ := procIsWindow.Call(uintptr(h)); r == 0 {
		return 0, fmt.Errorf("ウィンドウ(ハンドル%d)が存在しません", h)
	}
	return uintptr(h), nil
}

// enumWindowsCallback は EnumWindows に渡す固定のコールバック。
// windows.NewCallback で登録した枠は解放できず上限(2000)を超えるとプロセスが
// 終了するため、プロセスで1度だけ登録して使い回す。結果の格納先は enumList。
var (
	enumMu       sync.Mutex
	enumList     []windowInfo
	enumCallback = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		n, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
		enumList = append(enumList, windowInfo{Handle: int64(hwnd), Title: windows.UTF16ToString(buf)})
		return 1
	})
)

func (systemWindowDriver) List() ([]windowInfo, error) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumList = nil
	if r, _, err := procEnumWindows.Call(enumCallback, 0); r == 0 {
		return nil, fmt.Errorf("EnumWindowsに失敗しました: %w", err)
	}
	list := enumList
	enumList = nil
	return list, nil
}

func (systemWindowDriver) Activate(h int64) error {
	hwnd, err := checkHWND(h)
	if err != nil {
		return err
	}
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
	if r, _, _ := procSetForegroundWindow.Call(hwnd); r == 0 {
		return fmt.Errorf("ウィンドウをアクティブにできませんでした")
	}
	return nil
}

func (systemWindowDriver) Move(h int64, x, y int) error {
	hwnd, err := checkHWND(h)
	if err != nil {
		return err
	}
	r, _, e := procSetWindowPos.Call(hwnd, 0, uintptr(int32(x)), uintptr(int32(y)), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	if r == 0 {
		return fmt.Errorf("SetWindowPosに失敗しました: %w", e)
	}
	return nil
}

func (systemWindowDriver) Size(h int64) (int, int, error) {
	hwnd, err := checkHWND(h)
	if err != nil {
		return 0, 0, err
	}
	var rc winRect
	if r, _, e := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return 0, 0, fmt.Errorf("GetWindowRectに失敗しました: %w", e)
	}
	return int(rc.Right - rc.Left), int(rc.Bottom - rc.Top), nil
}

func (systemWindowDriver) Resize(h int64, w, hgt int) error {
	hwnd, err := checkHWND(h)
	if err != nil {
		return err
	}
	r, _, e := procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(int32(w)), uintptr(int32(hgt)), swpNoMove|swpNoZOrder|swpNoActivate)
	if r == 0 {
		return fmt.Errorf("SetWindowPosに失敗しました: %w", e)
	}
	return nil
}
