//go:build windows

package nodelib

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetSystemTimes = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemTimes")

var windowsCPUUsageState struct {
	sync.Mutex
	last []cpuSample
}

func filetimeToFloat(ft windows.Filetime) float64 {
	return float64(uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime))
}

// getCPUUsagePercent はシステム全体のCPU使用率（0〜100）を1要素の配列で返します。
// GetSystemTimes の累積時間を使う差分方式で、外部コマンドは使いません
// （wmic は新しいWindowsで削除されているため）。
// 初回呼び出し時は差分が取れないため、0を返します。
func getCPUUsagePercent() ([]float64, error) {
	var idle, kernel, user windows.Filetime
	r, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return nil, fmt.Errorf("CPU使用率取得: GetSystemTimes に失敗しました: %w", err)
	}
	// kernel にはidle時間が含まれる。
	cur := []cpuSample{{
		total: filetimeToFloat(kernel) + filetimeToFloat(user),
		idle:  filetimeToFloat(idle),
	}}
	windowsCPUUsageState.Lock()
	defer windowsCPUUsageState.Unlock()
	usage := cpuUsageFromSamples(windowsCPUUsageState.last, cur)
	windowsCPUUsageState.last = cur
	return usage, nil
}
