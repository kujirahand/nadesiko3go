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

// getCPUUsagePercent はCPUごとの使用率（0〜100）を配列で返します。
// macOS では /proc/stat に相当するCPUごとの値を簡単に取得できないため、
// top コマンドが計算したシステム全体の使用率を1要素の配列で返します。
func getCPUUsagePercent() ([]float64, error) {
	// -l 2 : 2回サンプリングして終了
	// -n 0 : プロセス表示なし
	// -F   : フレームワークビュー（メモリ等の追加情報を省きやすい）
	out, err := exec.Command("top", "-l", "2", "-n", "0", "-F").Output()
	if err != nil {
		return nil, fmt.Errorf("CPU使用率取得: top コマンドの実行に失敗しました: %w", err)
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
		return nil, fmt.Errorf("CPU使用率取得: top の出力から CPU usage を解析できません")
	}

	usage := 100 - lastIdle
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}
	return []float64{usage}, nil
}
