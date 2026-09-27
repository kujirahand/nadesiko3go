//go:build darwin

package nodelib

import (
	"fmt"
	"os/exec"
)

// getCPUUsagePercent はCPU使用率（0〜100）を配列で返します。
// macOS ではCPUごとの値を簡単に取得できないため、top コマンドが計算した
// システム全体の使用率を1要素の配列で返します。
// top は2回サンプリングするため、1〜2秒ほど待たされます。
func getCPUUsagePercent() ([]float64, error) {
	// -l 2 : 2回サンプリングして終了（1回目は起動時からの平均なので2回目を使う）
	// -n 0 : プロセス表示なし
	out, err := exec.Command("top", "-l", "2", "-n", "0").Output()
	if err != nil {
		return nil, fmt.Errorf("CPU使用率取得: top コマンドの実行に失敗しました: %w", err)
	}
	usage, err := parseTopCPUUsage(string(out))
	if err != nil {
		return nil, err
	}
	return []float64{usage}, nil
}
