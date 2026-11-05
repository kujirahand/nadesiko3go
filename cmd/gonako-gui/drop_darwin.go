//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void gonakoInstallFileDropHook(void);
char *gonakoTakeDroppedFiles(int *length);
*/
import "C"

import (
	"unsafe"

	webview "github.com/webview/webview_go"
)

func platformInstallFileDrop(_ *webview.Protected) error {
	C.gonakoInstallFileDropHook()
	return nil
}

// macOS は記録時にコピーを返す単一スロットで、名前による照合はしない。
func platformTakeDroppedFiles(_ []string) []string {
	var length C.int
	buf := C.gonakoTakeDroppedFiles(&length)
	if buf == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(buf))
	return splitDroppedPaths(C.GoBytes(unsafe.Pointer(buf), length))
}
