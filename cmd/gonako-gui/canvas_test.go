package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// サンプルの描画からクリックによる消去までを実行し、両方の画面への橋渡しを確認する。
func TestCanvasSampleAndBrowserBridge(t *testing.T) {
	code, err := os.ReadFile("ui/samples/17_キャンバス描画.nako3")
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	initial := session.run(string(code), "canvas.nako3", true, nil, nil)
	if !initial.OK {
		t.Fatalf("サンプルの実行: %s", initial.Error)
	}
	button := 0
	for _, op := range initial.Operations {
		if op.Type == "create" && op.Tag == "button" {
			button = op.Handle
		}
	}
	if button == 0 {
		t.Fatal("消去ボタンがありません")
	}
	cleared := dispatchEvent(t, session, initial.RunID, button, "click", nil)
	if !cleared.OK {
		t.Fatalf("消去: %s", cleared.Error)
	}
	if len(cleared.Operations) != 1 || cleared.Operations[0].Type != "canvas" || cleared.Operations[0].Canvas.Action != "clear" {
		t.Fatalf("消去操作: %#v", cleared.Operations)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("画面側の検証にはNode.jsが必要です")
	}
	ops, err := json.Marshal(append(initial.Operations, cleared.Operations...))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "testdata/canvas-regression.cjs")
	cmd.Stdin = bytes.NewReader(ops)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("キャンバス描画: %v\n%s", err, output)
	}
}
