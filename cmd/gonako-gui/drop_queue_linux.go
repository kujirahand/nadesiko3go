//go:build linux

package main

/*
#include <stdlib.h>
#include "drop_buffer_linux.h"
*/
import "C"

import (
	"strings"
)

// このファイルの関数は検証用。GTKコールバックを通さずに
// drop_buffer_linux.c のキューを Go のテストから確認するために置く。
// 実行時のコードからは呼ばれない。

// recordDroppedFilesForTest はドロップ1回分のパスをキューへ積む。
// paths が空なら「パスなし」のマーカー項目を積む。
func recordDroppedFilesForTest(paths ...string) bool {
	if len(paths) == 0 {
		return C.gonakoDropQueuePushCopy(nil, 0) != 0
	}
	joined := strings.Join(paths, "\x00") + "\x00"
	copied := C.CBytes([]byte(joined))
	defer C.free(copied)
	return C.gonakoDropQueuePushCopy((*C.char)(copied), C.size_t(len(joined))) != 0
}

// recordDroppedRawForTest は NUL 区切りのバイト列をそのまま積む。
// 並行記録のストレス確認に使う。
func recordDroppedRawForTest(joined string) bool {
	copied := C.CBytes([]byte(joined))
	defer C.free(copied)
	return C.gonakoDropQueuePushCopy((*C.char)(copied), C.size_t(len(joined))) != 0
}

// clearDroppedFilesForTest はキューに残った項目をすべて解放する。
// 記録側も取得側も動いていないときにだけ使う。
func clearDroppedFilesForTest() {
	C.gonakoDropQueueClear()
}
