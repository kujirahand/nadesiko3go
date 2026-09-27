//go:build linux

package nodelib

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// linuxCPUSample は1個のCPU（コア）の使用率を計算するための1回分のサンプルです。
// /proc/stat の値はシステム起動時からの累積jiffiesなので、差分方式に適しています。
type linuxCPUSample struct {
	total float64
	idle  float64
}

var linuxCPUUsageState struct {
	sync.Mutex
	// last は前回取得したCPU（コア）ごとのサンプルです。
	last []linuxCPUSample
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

	samples, err := fetchCPUSamples()
	if err != nil {
		return nil, err
	}

	prev := linuxCPUUsageState.last
	if len(prev) != len(samples) {
		// CPU数が変わったら差分は取り直す。
		prev = nil
	}

	usage := make([]float64, len(samples))
	for i, s := range samples {
		if prev == nil {
			continue // 初回は差分が取れない
		}
		dTotal := s.total - prev[i].total
		dIdle := s.idle - prev[i].idle
		if dTotal <= 0 {
			continue
		}
		u := (dTotal - dIdle) / dTotal * 100
		if u < 0 {
			u = 0
		} else if u > 100 {
			u = 100
		}
		usage[i] = u
	}
	linuxCPUUsageState.last = samples
	return usage, nil
}

// fetchCPUSamples は /proc/stat の各CPU行（cpu0, cpu1 ...）から
// total/idle を読み取ります。全体の集計行（「cpu 」で始まる行）は除きます。
func fetchCPUSamples() ([]linuxCPUSample, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	var samples []linuxCPUSample
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") || strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 4 {
			return nil, fmt.Errorf("CPU使用率取得: /proc/stat の形式が想定外です")
		}
		vals := make([]float64, len(fields))
		for i, f := range fields {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return nil, fmt.Errorf("CPU使用率取得: /proc/stat の値が数値ではありません: %w", err)
			}
			vals[i] = v
		}
		// user(0), nice(1), system(2), idle(3), iowait(4), irq(5), softirq(6), steal(7)...
		total := vals[0] + vals[1] + vals[2] + vals[3]
		idle := vals[3]
		if len(vals) > 4 {
			total += vals[4] // iowait
			idle += vals[4]  // iowaitもCPU待機時間として扱う
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
		samples = append(samples, linuxCPUSample{total: total, idle: idle})
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("CPU使用率取得: /proc/stat に cpu 行が見つかりません")
	}
	return samples, nil
}
