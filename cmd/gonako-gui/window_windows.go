//go:build windows

package main

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

const (
	gwlStyle                = ^uintptr(15) // -16
	wsThickFrame            = 0x00040000
	wsMaximizeBox           = 0x00010000
	wsPopup                 = 0x80000000
	wsVisible               = 0x10000000
	swRestore               = 9
	swMinimize              = 6
	swMaximize              = 3
	swpNoSize               = 0x0001
	swpNoMove               = 0x0002
	swpNoZOrder             = 0x0004
	swpFrameChanged         = 0x0020
	monitorDefaultToNearest = 2
)

type windowRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor windowRect
	Work    windowRect
	Flags   uint32
}

type fullscreenWindowState struct {
	style uintptr
	rect  windowRect
}

var (
	windowUser32      = syscall.NewLazyDLL("user32.dll")
	getWindowRect     = windowUser32.NewProc("GetWindowRect")
	setWindowPos      = windowUser32.NewProc("SetWindowPos")
	showWindow        = windowUser32.NewProc("ShowWindow")
	isIconic          = windowUser32.NewProc("IsIconic")
	isZoomed          = windowUser32.NewProc("IsZoomed")
	monitorFromWindow = windowUser32.NewProc("MonitorFromWindow")
	getMonitorInfoW   = windowUser32.NewProc("GetMonitorInfoW")
	getWindowLongW    = windowUser32.NewProc("GetWindowLongW")
	setWindowLongW    = windowUser32.NewProc("SetWindowLongW")
	getWindowTextW    = windowUser32.NewProc("GetWindowTextW")
	fullscreenWindows sync.Map
)

func platformApplyWindowSettings(window unsafe.Pointer, settings guilib.WindowSettings) error {
	hwnd := uintptr(window)
	if hwnd == 0 {
		return fmt.Errorf("ネイティブウィンドウを取得できません")
	}

	if settings.HasState && settings.State == "通常" {
		restoreWindowsFullscreen(hwnd)
		showWindow.Call(hwnd, swRestore)
	}
	if settings.HasResizable {
		style, _, _ := getWindowLongW.Call(hwnd, gwlStyle)
		if settings.Resizable {
			style |= wsThickFrame | wsMaximizeBox
		} else {
			style &^= wsThickFrame | wsMaximizeBox
		}
		setWindowLongW.Call(hwnd, gwlStyle, style)
		setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
	}
	if settings.HasPosition {
		var rect windowRect
		if ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			return fmt.Errorf("ウィンドウの位置を取得できません")
		}
		x, y := int32(settings.X), int32(settings.Y)
		if settings.Center {
			monitor, _, _ := monitorFromWindow.Call(hwnd, monitorDefaultToNearest)
			info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
			if ok, _, _ := getMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
				return fmt.Errorf("画面の作業領域を取得できません")
			}
			x = info.Work.Left + (info.Work.Right-info.Work.Left-(rect.Right-rect.Left))/2
			y = info.Work.Top + (info.Work.Bottom-info.Work.Top-(rect.Bottom-rect.Top))/2
		}
		setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder)
	}

	if settings.HasState {
		switch settings.State {
		case "最大化":
			restoreWindowsFullscreen(hwnd)
			showWindow.Call(hwnd, swMaximize)
		case "最小化":
			showWindow.Call(hwnd, swMinimize)
		case "全画面":
			setWindowsFullscreen(hwnd)
		}
	}
	return nil
}

func setWindowsFullscreen(hwnd uintptr) {
	if _, exists := fullscreenWindows.Load(hwnd); exists {
		return
	}
	var rect windowRect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	style, _, _ := getWindowLongW.Call(hwnd, gwlStyle)
	fullscreenWindows.Store(hwnd, fullscreenWindowState{style: style, rect: rect})
	monitor, _, _ := monitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	getMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	setWindowLongW.Call(hwnd, gwlStyle, wsPopup|wsVisible)
	setWindowPos.Call(hwnd, 0, uintptr(info.Monitor.Left), uintptr(info.Monitor.Top),
		uintptr(info.Monitor.Right-info.Monitor.Left), uintptr(info.Monitor.Bottom-info.Monitor.Top),
		swpNoZOrder|swpFrameChanged)
}

func restoreWindowsFullscreen(hwnd uintptr) {
	raw, exists := fullscreenWindows.LoadAndDelete(hwnd)
	if !exists {
		return
	}
	saved := raw.(fullscreenWindowState)
	setWindowLongW.Call(hwnd, gwlStyle, saved.style)
	setWindowPos.Call(hwnd, 0, uintptr(saved.rect.Left), uintptr(saved.rect.Top),
		uintptr(saved.rect.Right-saved.rect.Left), uintptr(saved.rect.Bottom-saved.rect.Top),
		swpNoZOrder|swpFrameChanged)
}

func platformWindowInfo(window unsafe.Pointer) (guilib.WindowInfo, error) {
	hwnd := uintptr(window)
	if hwnd == 0 {
		return guilib.WindowInfo{}, fmt.Errorf("ネイティブウィンドウを取得できません")
	}
	var rect windowRect
	if ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return guilib.WindowInfo{}, fmt.Errorf("ウィンドウ情報を取得できません")
	}
	state := "通常"
	if _, fullscreen := fullscreenWindows.Load(hwnd); fullscreen {
		state = "全画面"
	} else if yes, _, _ := isIconic.Call(hwnd); yes != 0 {
		state = "最小化"
	} else if yes, _, _ := isZoomed.Call(hwnd); yes != 0 {
		state = "最大化"
	}
	style, _, _ := getWindowLongW.Call(hwnd, gwlStyle)
	titleBuffer := make([]uint16, 1024)
	length, _, _ := getWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuffer[0])), uintptr(len(titleBuffer)))
	runtime.KeepAlive(titleBuffer)
	return guilib.WindowInfo{
		Width: int(rect.Right - rect.Left), Height: int(rect.Bottom - rect.Top),
		X: int(rect.Left), Y: int(rect.Top), State: state,
		Title:     syscall.UTF16ToString(titleBuffer[:int(length)]),
		Resizable: style&wsThickFrame != 0,
	}, nil
}
