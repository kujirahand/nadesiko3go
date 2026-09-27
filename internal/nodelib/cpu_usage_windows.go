//go:build windows

package nodelib

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

var windowsLoadRE = regexp.MustCompile(`LoadPercentage\s*=\s*(\d+)`)

// getCPUUsagePercent は wmic コマンドで取得したCPUごとの現在の負荷率（0〜100）を配列で返します。
// wmic はCPUごとに LoadPercentage を出力するため、その順番に対応します。
func getCPUUsagePercent() ([]float64, error) {
	out, err := exec.Command("wmic", "cpu", "get", "loadpercentage", "/value").Output()
	if err != nil {
		return nil, fmt.Errorf("CPU使用率取得: wmic コマンドの実行に失敗しました: %w", err)
	}
	matches := windowsLoadRE.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("CPU使用率取得: wmic の出力から LoadPercentage を解析できません")
	}
	usage := make([]float64, 0, len(matches))
	for _, m := range matches {
		load, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil, fmt.Errorf("CPU使用率取得: LoadPercentage の解析に失敗しました: %w", err)
		}
		if load < 0 {
			load = 0
		} else if load > 100 {
			load = 100
		}
		usage = append(usage, load)
	}
	return usage, nil
}
