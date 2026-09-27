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
	if len(usage) == 0 {
		t.Fatal("CPU使用率取得は少なくとも1要素を返すべきです")
	}
	for i, v := range usage {
		if v < 0 || v > 100 {
			t.Fatalf("CPU使用率取得[%d]は 0〜100 の範囲であるべきですが %v を返しました", i, v)
		}
	}
}
