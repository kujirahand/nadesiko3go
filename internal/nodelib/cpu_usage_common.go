package nodelib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// cpuSample は1個のCPU（コア）の使用率を計算するための1回分のサンプルです。
// 値は起動時からの累積時間なので、2回のサンプルの差分から使用率を求めます。
type cpuSample struct {
	total float64
	idle  float64
}

// cpuUsageFromSamples は前回と今回のサンプルからCPUごとの使用率（0〜100）を返します。
// prev が nil、または要素数が異なる場合は差分が取れないので、すべて0を返します。
func cpuUsageFromSamples(prev, cur []cpuSample) []float64 {
	usage := make([]float64, len(cur))
	if len(prev) != len(cur) {
		return usage
	}
	for i, s := range cur {
		dTotal := s.total - prev[i].total
		dIdle := s.idle - prev[i].idle
		if dTotal <= 0 {
			continue
		}
		usage[i] = clampPercent((dTotal - dIdle) / dTotal * 100)
	}
	return usage
}

// clampPercent は値を0〜100の範囲に収めます。
func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// parseProcStat は /proc/stat の内容から、各CPU行（cpu0, cpu1 ...）のサンプルを返します。
// 全体の集計行（「cpu 」で始まる行）は除きます。
func parseProcStat(data string) ([]cpuSample, error) {
	var samples []cpuSample
	for _, line := range strings.Split(data, "\n") {
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
		// user(0), nice(1), system(2), idle(3), iowait(4), irq(5), softirq(6), steal(7)
		// guest系はuser/niceに含まれるので足さない。iowaitはCPU待機時間としてidle扱いにする。
		var total float64
		for i := 0; i < len(vals) && i < 8; i++ {
			total += vals[i]
		}
		idle := vals[3]
		if len(vals) > 4 {
			idle += vals[4]
		}
		samples = append(samples, cpuSample{total: total, idle: idle})
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("CPU使用率取得: /proc/stat に cpu 行が見つかりません")
	}
	return samples, nil
}

var topIdleRE = regexp.MustCompile(`CPU usage:\s*[\d.]+%?\s*user,\s*[\d.]+%?\s*sys,\s*([\d.]+)%?\s*idle`)

// parseTopCPUUsage は macOS の top の出力から、最後の「CPU usage」行の使用率を返します。
// top -l 2 の1回目は起動時からの平均なので、最後の行（2回目）を使います。
func parseTopCPUUsage(out string) (float64, error) {
	lastIdle := -1.0
	for _, line := range strings.Split(out, "\n") {
		m := topIdleRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			lastIdle = v
		}
	}
	if lastIdle < 0 {
		return 0, fmt.Errorf("CPU使用率取得: top の出力から CPU usage を解析できません")
	}
	return clampPercent(100 - lastIdle), nil
}
