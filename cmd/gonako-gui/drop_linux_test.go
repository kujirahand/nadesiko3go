//go:build linux

package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestDroppedFilesQueueMatching(t *testing.T) {
	clearDroppedFilesForTest()
	t.Cleanup(clearDroppedFilesForTest)

	if !recordDroppedFilesForTest("/a/1.txt") {
		t.Fatal("1回目の記録に失敗")
	}
	if !recordDroppedFilesForTest("/b/2.txt", "/b/3.txt") {
		t.Fatal("2回目の記録に失敗")
	}
	if !recordDroppedFilesForTest() { // URIは来たがパスに変換できなかったドロップ
		t.Fatal("マーカーの記録に失敗")
	}
	if !recordDroppedFilesForTest("/c/4.txt") {
		t.Fatal("4回目の記録に失敗")
	}

	// イベントはファイル名(ベース名)で照合して取る。順序は問わない。
	if got := platformTakeDroppedFiles([]string{"1.txt"}); !reflect.DeepEqual(got, []string{"/a/1.txt"}) {
		t.Fatalf("1回目: %#v", got)
	}
	if got := platformTakeDroppedFiles([]string{"3.txt", "2.txt"}); !reflect.DeepEqual(got, []string{"/b/2.txt", "/b/3.txt"}) {
		t.Fatalf("2回目: %#v", got)
	}
	// 一致しないイベントは何も取らない。このとき先頭のマーカーだけが消費され、
	// 後続の実項目は将来のイベントのために残る。
	if got := platformTakeDroppedFiles([]string{"unknown.txt"}); got != nil {
		t.Fatalf("不一致: %#v", got)
	}
	if got := platformTakeDroppedFiles([]string{"4.txt"}); !reflect.DeepEqual(got, []string{"/c/4.txt"}) {
		t.Fatalf("4回目: %#v", got)
	}
	// 取得済みのドロップは二度取れない。
	if got := platformTakeDroppedFiles([]string{"4.txt"}); got != nil {
		t.Fatalf("取得済み: %#v", got)
	}
	// ファイル名を持たないイベント(テキストのドロップ等)は項目を消費しない。
	if !recordDroppedFilesForTest("/d/5.txt") {
		t.Fatal("5回目の記録に失敗")
	}
	if got := platformTakeDroppedFiles(nil); got != nil {
		t.Fatalf("名前なし: %#v", got)
	}
	if got := platformTakeDroppedFiles([]string{"5.txt"}); !reflect.DeepEqual(got, []string{"/d/5.txt"}) {
		t.Fatalf("5回目: %#v", got)
	}
}

// TestDroppedFilesQueueDiscardsStale は、対応するイベントが失われた
// (実行中のため拒否された等)ドロップの項目が残っていても、後続の
// イベントが自分のドロップを正しく取れることを確認する。
func TestDroppedFilesQueueDiscardsStale(t *testing.T) {
	clearDroppedFilesForTest()
	t.Cleanup(clearDroppedFilesForTest)

	if !recordDroppedFilesForTest("/old/1.txt") {
		t.Fatal("古いドロップの記録に失敗")
	}
	if !recordDroppedFilesForTest("/new/2.txt") {
		t.Fatal("新しいドロップの記録に失敗")
	}

	// 先のイベントが失われても、後のイベントは自分のパスを取る。
	// その際、古い項目は残りとして捨てられる。
	if got := platformTakeDroppedFiles([]string{"2.txt"}); !reflect.DeepEqual(got, []string{"/new/2.txt"}) {
		t.Fatalf("新しいドロップ: %#v", got)
	}
	if got := platformTakeDroppedFiles([]string{"1.txt"}); got != nil {
		t.Fatalf("捨てられたはずの古いドロップ: %#v", got)
	}
}

// TestDroppedFilesQueueConcurrent は、記録(複数スレッド)と取得を重ねても
// 項目が壊れたり重複したりせず、記録した側ごとの順序が保たれることを見る。
// ここでは照合を使わず先頭から取り、キュー自体の並行安全性を確認する。
// (照合はイベント順に取る前提の仕組みなので、順序を乱す取り方はしない。
// 照合の挙動は TestDroppedFilesQueueMatching 等とハーネスで確認する。)
// C側のメモリ安全性そのものは ASan/TSan ハーネスでも別途確認している
// (testdata/drop_buffer_harness.c)。
func TestDroppedFilesQueueConcurrent(t *testing.T) {
	clearDroppedFilesForTest()
	t.Cleanup(clearDroppedFilesForTest)

	const producers = 4
	const perProducer = 250
	total := producers * perProducer

	var producersWG sync.WaitGroup
	for p := 0; p < producers; p++ {
		producersWG.Add(1)
		go func(p int) {
			defer producersWG.Done()
			for i := 0; i < perProducer; i++ {
				if !recordDroppedRawForTest(fmt.Sprintf("/p%d/%d\x00", p, i)) {
					t.Errorf("記録に失敗: producer %d, %d", p, i)
					return
				}
			}
		}(p)
	}

	// 取得はイベント処理と同じく1本で、積まれた順に取り切る。
	// 記録が間に合わない場合は待ち合わせる(無限には待たない)。
	received := make([]string, 0, total)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for len(received) < total {
			paths := takeDroppedFilesHeadForTest()
			if paths == nil {
				runtime.Gosched()
				continue
			}
			received = append(received, paths...)
		}
	}()
	producersWG.Wait()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("時間内に取り切れませんでした: %d 件取得済み", len(received))
	}

	if len(received) != total {
		t.Fatalf("受信数: got %d, want %d", len(received), total)
	}
	last := make([]int, producers)
	for i := range last {
		last[i] = -1
	}
	seen := make(map[string]bool, total)
	for _, path := range received {
		if seen[path] {
			t.Fatalf("重複して受け取った: %q", path)
		}
		seen[path] = true
		var p, i int
		if _, err := fmt.Sscanf(path, "/p%d/%d", &p, &i); err != nil {
			t.Fatalf("壊れたパス: %q", path)
		}
		if i <= last[p] {
			t.Fatalf("producer %d の順序が逆転: %d の後に %d", p, last[p], i)
		}
		last[p] = i
	}
}
