//go:build windows

package main

/*
#cgo LDFLAGS: -lole32 -luuid -lshell32 -luser32
#include <stdlib.h>

void gonakoInstallFileDropHook(void *hwnd);
char *gonakoTakeDroppedFiles(int *length);
*/
import "C"

import "unsafe"

func platformInstallFileDrop(window unsafe.Pointer) {
	C.gonakoInstallFileDropHook(window)
}

func platformTakeDroppedFiles() []string {
	var length C.int
	buf := C.gonakoTakeDroppedFiles(&length)
	if buf == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(buf))
	return splitDroppedPaths(C.GoBytes(unsafe.Pointer(buf), length))
}
