//go:build !linux && !darwin && !windows

package nodelib

import "fmt"

// getCPUUsagePercent は未対応のプラットフォームでは常にエラーを返します。
func getCPUUsagePercent() ([]float64, error) {
	return nil, fmt.Errorf("CPU使用率取得: このプラットフォームでは未対応です")
}
