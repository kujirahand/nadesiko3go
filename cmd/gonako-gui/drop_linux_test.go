//go:build linux

package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func TestDroppedFilesQueueFIFO(t *testing.T) {
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

	if got := platformTakeDroppedFiles(); !reflect.DeepEqual(got, []string{"/a/1.txt"}) {
		t.Fatalf("1回目: %#v", got)
	}
	if got := platformTakeDroppedFiles(); !reflect.DeepEqual(got, []string{"/b/2.txt", "/b/3.txt"}) {
		t.Fatalf("2回目: %#v", got)
	}
	// マーカーは「何も取れない」として消費され、後続との対応がずれない。
	if got := platformTakeDroppedFiles(); got != nil {
		t.Fatalf("マーカー: %#v", got)
	}
	if got := platformTakeDroppedFiles(); !reflect.DeepEqual(got, []string{"/c/4.txt"}) {
		t.Fatalf("4回目: %#v", got)
	}
	if got := platformTakeDroppedFiles(); got != nil {
		t.Fatalf("空のキュー: %#v", got)
	}
}

// TestDroppedFilesQueueConcurrent は、記録(複数スレッド)と取得を重ねても
// 項目が壊れたり重複したりせず、記録した側ごとの順序が保たれることを見る。
// C側のメモリ安全性そのものは ASan/TSan ハーネスでも別途確認している。
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
	received := make([]string, 0, total)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for len(received) < total {
			paths := platformTakeDroppedFiles()
			if paths == nil {
				runtime.Gosched()
				continue
			}
			received = append(received, paths...)
		}
	}()
	producersWG.Wait()
	<-done

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
