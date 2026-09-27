//go:build darwin

package nodelib

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var darwinIdleRE = regexp.MustCompile(`CPU usage:\s*[\d.]+%?\s*user,\s*[\d.]+%?\s*sys,\s*([\d.]+)%?\s*idle`)

// getCPUUsagePercent は top コマンドで取得した現在のCPU使用率（0〜100）を返します。
// macOS では /proc/stat に相当する累積時間を簡単に取得できないため、
// top が計算した使用率から idle 率を引いて返します。
func getCPUUsagePercent() (float64, error) {
	// -l 2 : 2回サンプリングして終了
	// -n 0 : プロセス表示なし
	// -F   : フレームワークビュー（メモリ等の追加情報を省きやすい）
	out, err := exec.Command("top", "-l", "2", "-n", "0", "-F").Output()
	if err != nil {
		return 0, fmt.Errorf("CPU使用率取得: top コマンドの実行に失敗しました: %w", err)
	}

	var lastIdle float64 = -1
	for _, line := range strings.Split(string(out), "\n") {
		m := darwinIdleRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		lastIdle = v
	}
	if lastIdle < 0 {
		return 0, fmt.Errorf("CPU使用率取得: top の出力から CPU usage を解析できません")
	}

	usage := 100 - lastIdle
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}
	return usage, nil
}
