package nodelib

import (
	"math"
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

func TestParseProcStat(t *testing.T) {
	data := "cpu  100 0 100 800 0 0 0 0 0 0\n" +
		"cpu0 50 0 50 400 50 10 10 30 5 0\n" +
		"cpu1 10 5 5 80\n" +
		"intr 1 2 3\n"
	got, err := parseProcStat(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("cpu行は2つのはず: %d", len(got))
	}
	// cpu0: user..steal の合計、idle=idle+iowait
	if got[0].total != 600 || got[0].idle != 450 {
		t.Errorf("cpu0 = %+v", got[0])
	}
	// cpu1: 4フィールドだけでも読める
	if got[1].total != 100 || got[1].idle != 80 {
		t.Errorf("cpu1 = %+v", got[1])
	}
	if _, err := parseProcStat("intr 1 2 3\n"); err == nil {
		t.Error("cpu行が無ければエラーのはず")
	}
	if _, err := parseProcStat("cpu0 1 2\n"); err == nil {
		t.Error("フィールド不足はエラーのはず")
	}
}

func TestCPUUsageFromSamples(t *testing.T) {
	prev := []cpuSample{{total: 100, idle: 80}, {total: 100, idle: 0}}
	cur := []cpuSample{{total: 200, idle: 130}, {total: 100, idle: 0}}
	got := cpuUsageFromSamples(prev, cur)
	if math.Abs(got[0]-50) > 1e-9 {
		t.Errorf("cpu0 = %v, want 50", got[0])
	}
	if got[1] != 0 { // 時間が進んでいなければ0
		t.Errorf("cpu1 = %v, want 0", got[1])
	}
	// 初回（前回なし）はすべて0
	for _, v := range cpuUsageFromSamples(nil, cur) {
		if v != 0 {
			t.Errorf("初回は0のはず: %v", v)
		}
	}
}

func TestParseTopCPUUsage(t *testing.T) {
	out := "CPU usage: 30.00% user, 10.00% sys, 60.00% idle\n" +
		"CPU usage: 5.26% user, 8.77% sys, 85.96% idle\n"
	got, err := parseTopCPUUsage(out)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-14.04) > 1e-9 {
		t.Errorf("got %v, want 14.04（最後の行を使う）", got)
	}
	if _, err := parseTopCPUUsage("no cpu line"); err == nil {
		t.Error("解析できなければエラーのはず")
	}
}
