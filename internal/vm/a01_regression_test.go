package vm_test

import (
	"strconv"
	"strings"
	"testing"
)

// A01: イベントキューの登録と個別キャンセルをVMの入口（『秒後』『タイマー停止』）
// から通し、件数を上げても実行順とタイマーIDの払い出しが変わらないことを固定する。
func TestA01VMTimerRegisterAndCancel(t *testing.T) {
	const n = 8192

	var code strings.Builder

	// 同一時刻に n 件の単発タイマーを積む。コールバックは自分のタイマーIDを出す。
	code.WriteString("F=関数()\n対象を表示\nここまで\nTimerIDs=[]\n")
	code.WriteString(strconv.Itoa(n) + "回\nID=Fを0.01秒後\nTimerIDsにIDを配列追加\nここまで\n")

	// 偶数IDだけを個別に停止する。停止が失敗した件数を数える。
	code.WriteString("Bad=0\n")
	code.WriteString(strconv.Itoa(n) + "回\n")
	code.WriteString("Idx=回数-1\nTid=TimerIDs[Idx]\nRem=Tid%2\n")
	code.WriteString("もしRem=0ならば\n")
	code.WriteString("R=Tidのタイマー停止\nもしR=オフならば\nBad=Bad+1\nここまで\n")
	code.WriteString("ここまで\nここまで\n")

	code.WriteString("Badを表示\n999999のタイマー停止を表示\n0.1秒待つ")

	got := run(t, code.String())
	lines := strings.Split(got, "\n")
	if len(lines) != n/2+2 {
		t.Fatalf("出力行数 = %d, want %d（停止失敗件数と未知IDの結果を含む）", len(lines), n/2+2)
	}
	if lines[0] != "0" {
		t.Errorf("キャンセル失敗件数 = %q, want \"0\"", lines[0])
	}
	if lines[1] != "false" {
		t.Errorf("未知IDのタイマー停止 = %q, want \"false\"", lines[1])
	}
	for k, line := range lines[2:] {
		if want := strconv.Itoa(2*k + 1); line != want {
			t.Fatalf("実行順[%d] = %q, want %q", k, line, want)
		}
	}
}
