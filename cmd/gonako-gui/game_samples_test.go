package main

import (
	"os"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

// ゲームサンプル (#522) をGUIセッションで実行し、キー入力・クリック・
// タイマーポンプ (#528) による進行を確かめる受け入れテスト。
func runGameSample(t *testing.T, path string) (*guiSession, RunResult, int) {
	t.Helper()
	code, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	done := make(chan RunResult, 1)
	go func() { done <- session.run(string(code), "game.nako3", true, nil, nil) }()
	var initial RunResult
	select {
	case initial = <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("サンプルの実行が10秒以内に返りません (タイマーでブロック?)")
	}
	if !initial.OK {
		t.Fatalf("サンプルの実行: %s", initial.Error)
	}
	canvas := 0
	for _, op := range initial.Operations {
		if op.Type == "create" && op.Tag == "canvas" {
			canvas = op.Handle
		}
	}
	if canvas == 0 {
		t.Fatal("キャンバスがありません")
	}
	return session, initial, canvas
}

func TestGameSampleSnake(t *testing.T) {
	session, initial, canvas := runGameSample(t, "ui/samples/20_スネークゲーム.nako3")
	// 下キーで方向転換できること (エラーなく処理されること)
	res := dispatchEvent(t, session, initial.RunID, canvas, "keydown", guilib.EventPayload{Key: "ArrowDown"})
	if !res.OK {
		t.Fatalf("keydown: %s", res.Error)
	}
	// タイマーが進むとキャンバス描画が続くこと: しばらく待ってからもう一度キー入力し、
	// その間の操作にエラーがないことを確認する
	time.Sleep(400 * time.Millisecond)
	res = dispatchEvent(t, session, initial.RunID, canvas, "keydown", guilib.EventPayload{Key: "ArrowLeft"})
	if !res.OK {
		t.Fatalf("keydown2: %s", res.Error)
	}
	t.Logf("operations after keys: %d", len(res.Operations))
}

// 神経衰弱: カードをめくる・外れたらタイマーで裏返ることを確認する。
func TestGameSampleMemory(t *testing.T) {
	code, err := os.ReadFile("ui/samples/22_神経衰弱ゲーム.nako3")
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	// 実機のエディタと同じく、メイン実行は非同期で生かし続ける (タイマーが回るため)
	mainRunID := session.start(string(code), "memory.nako3", true, nil, nil)
	collector := newAsyncCollector(session, mainRunID)
	deadline := time.Now().Add(5 * time.Second)
	var buttons []int
	for time.Now().Before(deadline) && len(buttons) < 16 {
		collector.poll()
		buttons = buttons[:0]
		for _, op := range collector.ops {
			if op.Type == "create" && op.Tag == "button" {
				buttons = append(buttons, op.Handle)
			}
		}
		if len(buttons) < 16 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	initial := RunResult{RunID: mainRunID, OK: true}
	if len(buttons) != 16 {
		t.Fatalf("ボタンが16個ありません: %d", len(buttons))
	}
	textOf := func(ops []guilib.Operation, handle int) (string, bool) {
		text := ""
		found := false
		for _, op := range ops {
			if op.Type == "text" && op.Handle == handle {
				text = op.Text
				found = true
			}
		}
		return text, found
	}
	// 1枚目をめくる
	r1 := dispatchEvent(t, session, initial.RunID, buttons[0], "click", noEvent)
	if !r1.OK {
		t.Fatalf("click1: %s", r1.Error)
	}
	e0, ok := textOf(r1.Operations, buttons[0])
	if !ok || e0 == "？" || e0 == "" {
		t.Fatalf("1枚目がめくれませんでした: %q", e0)
	}
	// 2枚目をめくる (絵柄が違えばタイマーで裏返る)
	r2 := dispatchEvent(t, session, initial.RunID, buttons[1], "click", noEvent)
	if !r2.OK {
		t.Fatalf("click2: %s", r2.Error)
	}
	e1, _ := textOf(r2.Operations, buttons[1])
	t.Logf("cards: %q vs %q", e0, e1)
	if e1 != e0 {
		// 外れ: 0.8秒後に裏返る。タイマーポンプを回して裏返しを進める
		flipBack := false
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !flipBack {
			started := session.startPump(initial.RunID)
			if started.Error != "" {
				t.Fatalf("pump: %s", started.Error)
			}
			c := newAsyncCollector(session, started.RunID)
			waitForAsyncDone(t, c)
			for _, op := range c.ops {
				if op.Type == "text" && op.Text == "？" && (op.Handle == buttons[0] || op.Handle == buttons[1]) {
					flipBack = true
				}
			}
			time.Sleep(60 * time.Millisecond)
		}
		if !flipBack {
			t.Fatal("外れたカードがタイマーで裏返りませんでした")
		}
		// ロックが解けて3枚目がめくれること
		r3 := dispatchEvent(t, session, initial.RunID, buttons[2], "click", noEvent)
		if !r3.OK {
			t.Fatalf("click3: %s", r3.Error)
		}
		e2, ok2 := textOf(r3.Operations, buttons[2])
		if !ok2 || e2 == "？" {
			t.Fatalf("ロックが解けず3枚目がめくれません: ops=%#v", r3.Operations)
		}
		t.Logf("3rd card opened: %q (flip-back timer path OK)", e2)
	}
}

// 落ちものゲーム: 初期描画・キー操作・タイマー落下を確認する。
func TestGameSampleTetris(t *testing.T) {
	session, initial, canvas := runGameSample(t, "ui/samples/23_落ちものゲーム.nako3")
	canvasOps := 0
	for _, op := range initial.Operations {
		if op.Type == "canvas" {
			canvasOps++
		}
	}
	// 背景1 + ブロック4マス = 5回以上の描画があること
	t.Logf("initial canvas ops: %d", canvasOps)
	if canvasOps < 5 {
		t.Fatalf("初期描画が不足しています: %d", canvasOps)
	}
	for _, key := range []string{"ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"} {
		res := dispatchEvent(t, session, initial.RunID, canvas, "keydown", guilib.EventPayload{Key: key})
		if !res.OK {
			t.Fatalf("keydown %s: %s", key, res.Error)
		}
	}
	// ポンプで落下を進め、描画が増え続けること (落下・固定・新ブロック生成)
	before := canvasOps
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		started := session.startPump(initial.RunID)
		if started.Error != "" {
			t.Fatalf("pump: %s", started.Error)
		}
		c := newAsyncCollector(session, started.RunID)
		st := waitForAsyncDone(t, c)
		if st.Result != nil && !st.Result.OK {
			t.Fatalf("ポンプの実行エラー: %s", st.Result.Error)
		}
		for _, op := range c.ops {
			if op.Type == "canvas" {
				canvasOps++
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("canvas ops after pumps: %d (before %d)", canvasOps, before)
	if canvasOps <= before {
		t.Fatal("タイマー落下の描画が増えていません")
	}
}

// 落ちものゲーム: ブロックが固定されて積み上がることを確認する。
func TestGameSampleTetrisLocking(t *testing.T) {
	session, initial, canvas := runGameSample(t, "ui/samples/23_落ちものゲーム.nako3")
	pumpFor := func(d time.Duration) []guilib.Operation {
		var ops []guilib.Operation
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			started := session.startPump(initial.RunID)
			if started.Error != "" {
				t.Fatalf("pump: %s", started.Error)
			}
			c := newAsyncCollector(session, started.RunID)
			st := waitForAsyncDone(t, c)
			if st.Result != nil && !st.Result.OK {
				t.Fatalf("ポンプの実行エラー: %s", st.Result.Error)
			}
			ops = append(ops, c.ops...)
			time.Sleep(40 * time.Millisecond)
		}
		return ops
	}
	isGameOverText := func(ops []guilib.Operation) bool {
		for _, op := range ops {
			if op.Type == "text" && len(op.Text) >= len("ゲームオーバー") && op.Text[:len("ゲームオーバー")] == "ゲームオーバー" {
				return true
			}
		}
		return false
	}
	gameOver := false
	lastDrawOps := 0
	for i := 0; i < 14 && !gameOver; i++ {
		// ソフトドロップで底まで落としてから、タイマーのtickで固定する
		for j := 0; j < 25; j++ {
			res := dispatchEvent(t, session, initial.RunID, canvas, "keydown", guilib.EventPayload{Key: "ArrowDown"})
			if !res.OK {
				t.Fatalf("drop: %s", res.Error)
			}
			if isGameOverText(res.Operations) {
				gameOver = true
			}
		}
		if isGameOverText(pumpFor(700 * time.Millisecond)) {
			gameOver = true
		}
		// 再描画1回分のキャンバス操作数 = 消去+背景+固定済みマス+現在のブロック4
		res := dispatchEvent(t, session, initial.RunID, canvas, "keydown", guilib.EventPayload{Key: "ArrowLeft"})
		if !res.OK {
			t.Fatalf("redraw: %s", res.Error)
		}
		n := 0
		for _, op := range res.Operations {
			if op.Type == "canvas" {
				n++
			}
		}
		if isGameOverText(res.Operations) {
			gameOver = true
		}
		lastDrawOps = n
		t.Logf("piece %d: redraw canvas ops=%d gameOver=%v", i, n, gameOver)
	}
	// 固定が進めば再描画のマス数が増える (初期は6)。ゲームオーバー到達でもよい
	if !gameOver && lastDrawOps <= 6 {
		t.Fatalf("ブロックが固定されていません: redraw ops=%d", lastDrawOps)
	}
}

// タイマーが実際に進むことを、タイマーポンプ (#528) で確認する。
func TestGameSampleSnakeTimerTicks(t *testing.T) {
	code, err := os.ReadFile("ui/samples/20_スネークゲーム.nako3")
	if err != nil {
		t.Fatal(err)
	}
	session := &guiSession{}
	runID := session.start(string(code), "game.nako3", true, nil, nil)
	mainCollector := newAsyncCollector(session, runID)
	waitForAsyncDone(t, mainCollector)
	canvasOps := 0
	for _, op := range mainCollector.ops {
		if op.Type == "canvas" {
			canvasOps++
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		started := session.startPump(runID)
		if started.Error != "" {
			t.Fatalf("pump: %s", started.Error)
		}
		c := newAsyncCollector(session, started.RunID)
		st := waitForAsyncDone(t, c)
		if st.Result != nil && !st.Result.OK {
			t.Fatalf("ポンプの実行エラー: %s", st.Result.Error)
		}
		for _, op := range c.ops {
			if op.Type == "canvas" {
				canvasOps++
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	// 初回描画 (消去1+背景1+餌1+ヘビ3=6) を超えて、タイマー分の描画が増えていること
	t.Logf("canvas ops in 2s: %d", canvasOps)
	if canvasOps <= 6 {
		t.Fatalf("タイマーの描画が増えていません: %d", canvasOps)
	}
}
