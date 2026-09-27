//go:build linux

package nodelib

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cpuSample はCPU使用率を計算するための1回分のサンプルです。
// /proc/stat の値はシステム起動時からの累積jiffiesなので、差分方式に適しています。
type cpuSample struct {
	total float64
	idle  float64
}

var cpuUsageState struct {
	sync.Mutex
	last    cpuSample
	hasLast bool
	lastAt  time.Time
}

// getCPUUsagePercent は前回呼び出しからのCPU使用率（0〜100）を返します。
// 初回呼び出し時は差分が取れないため0を返します（v1マニュアルの
// 「定期的に呼び出して使う」に合わせます）。
func getCPUUsagePercent() (float64, error) {
	sample, err := fetchCPUSample()
	if err != nil {
		return 0, err
	}

	cpuUsageState.Lock()
	defer cpuUsageState.Unlock()

	if !cpuUsageState.hasLast {
		cpuUsageState.last = sample
		cpuUsageState.hasLast = true
		cpuUsageState.lastAt = time.Now()
		return 0, nil
	}

	dTotal := sample.total - cpuUsageState.last.total
	dIdle := sample.idle - cpuUsageState.last.idle
	cpuUsageState.last = sample
	cpuUsageState.lastAt = time.Now()

	if dTotal <= 0 {
		return 0, nil
	}
	usage := (dTotal - dIdle) / dTotal * 100
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}
	return usage, nil
}

// fetchCPUSample は /proc/stat の先頭行（cpu）から total/idle を読み取ります。
func fetchCPUSample() (cpuSample, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 4 {
			return cpuSample{}, fmt.Errorf("CPU使用率取得: /proc/stat の形式が想定外です")
		}
		vals := make([]float64, len(fields))
		for i, f := range fields {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return cpuSample{}, fmt.Errorf("CPU使用率取得: /proc/stat の値が数値ではありません: %w", err)
			}
			vals[i] = v
		}
		// user(0), nice(1), system(2), idle(3), iowait(4), irq(5), softirq(6), steal(7)...
		total := vals[0] + vals[1] + vals[2] + vals[3]
		idle := vals[3]
		if len(vals) > 4 {
			total += vals[4] // iowait
		}
		if len(vals) > 5 {
			total += vals[5] // irq
		}
		if len(vals) > 6 {
			total += vals[6] // softirq
		}
		if len(vals) > 7 {
			total += vals[7] // steal
		}
		return cpuSample{total: total, idle: idle}, nil
	}
	return cpuSample{}, fmt.Errorf("CPU使用率取得: /proc/stat に cpu 行が見つかりません")
}
