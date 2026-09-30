//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void gonakoInstallFileDropHook(void);
char *gonakoTakeDroppedFiles(int *length);
*/
import "C"

import "unsafe"

func platformInstallFileDrop(_ unsafe.Pointer) {
	C.gonakoInstallFileDropHook()
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
