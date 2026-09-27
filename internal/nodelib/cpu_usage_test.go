package nodelib

import (
	"runtime"
	"testing"
)

func TestCPUUsage(t *testing.T) {
	usage, err := getCPUUsagePercent()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if err == nil {
			t.Fatalf("未対応プラットフォーム %s ではエラーを返すべきです", runtime.GOOS)
		}
		return
	}
	if err != nil {
		t.Fatalf("CPU使用率取得でエラー: %v", err)
	}
	if usage < 0 || usage > 100 {
		t.Fatalf("CPU使用率取得は 0〜100 の範囲であるべきですが %v を返しました", usage)
	}
}
