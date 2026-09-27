//go:build linux

package nodelib

import (
	"os"
	"sync"
)

var linuxCPUUsageState struct {
	sync.Mutex
	// last は前回取得したCPU（コア）ごとのサンプルです。
	last []cpuSample
}

// getCPUUsagePercent はCPU（コア）ごとの使用率（0〜100）を配列で返します。
// 要素は /proc/stat の cpu0, cpu1 ... の並び順に対応します。
// 初回呼び出し時は差分が取れないため、すべて0の配列を返します
// （v1マニュアルの「定期的に呼び出して使う」に合わせます）。
func getCPUUsagePercent() ([]float64, error) {
	// サンプル取得から前回値の更新まですべてロックで囲み、
	// 複数の呼び出しが重なっても順序が崩れないようにします。
	linuxCPUUsageState.Lock()
	defer linuxCPUUsageState.Unlock()

	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	samples, err := parseProcStat(string(data))
	if err != nil {
		return nil, err
	}
	usage := cpuUsageFromSamples(linuxCPUUsageState.last, samples)
	linuxCPUUsageState.last = samples
	return usage, nil
}
