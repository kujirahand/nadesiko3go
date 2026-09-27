//go:build windows

package nodelib

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

var windowsLoadRE = regexp.MustCompile(`LoadPercentage\s*=\s*(\d+)`)

// getCPUUsagePercent は wmic コマンドで取得した現在のCPU負荷率（0〜100）を返します。
func getCPUUsagePercent() (float64, error) {
	out, err := exec.Command("wmic", "cpu", "get", "loadpercentage", "/value").Output()
	if err != nil {
		return 0, fmt.Errorf("CPU使用率取得: wmic コマンドの実行に失敗しました: %w", err)
	}
	m := windowsLoadRE.FindStringSubmatch(string(out))
	if m == nil {
		return 0, fmt.Errorf("CPU使用率取得: wmic の出力から LoadPercentage を解析できません")
	}
	load, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("CPU使用率取得: LoadPercentage の解析に失敗しました: %w", err)
	}
	if load < 0 {
		load = 0
	} else if load > 100 {
		load = 100
	}
	return load, nil
}
