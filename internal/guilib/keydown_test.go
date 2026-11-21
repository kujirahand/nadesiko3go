package guilib

import (
	"encoding/json"
	"strings"
	"testing"
)

// Issue #523: 画面部品のキー押下イベント。押されたキー名はシステム変数
// 『押下キー』で届く (ドロップファイルと同じ流儀で、専用フィールドで渡す)。
func TestKeyDownEventDeliversKeyName(t *testing.T) {
	screen, out := runGUIProgram(t, `絵=[320,200]のキャンバス作成
絵をキー押下した時には
　押下キーを表示
ここまで`)
	ops := screen.DrainOperations()
	found := false
	for _, op := range ops {
		if op.Type == "listen" && op.Event == "keydown" {
			found = true
		}
	}
	if !found {
		t.Fatalf("keydown の listen 操作がありません: %#v", ops)
	}
	if err := screen.DispatchEvent(1, "keydown", EventPayload{Key: "ArrowLeft"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "ArrowLeft" {
		t.Fatalf("押下キー = %q, want ArrowLeft", got)
	}
}

// JS側からは {"key": "ArrowLeft"} の形で届く。EventPayload へ読み込めること。
func TestKeyDownPayloadJSON(t *testing.T) {
	var payload EventPayload
	if err := json.Unmarshal([]byte(`{"key":"ArrowRight"}`), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Key != "ArrowRight" {
		t.Fatalf("Key = %q, want ArrowRight", payload.Key)
	}
}
