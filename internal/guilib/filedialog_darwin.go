//go:build darwin

package guilib

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

char *gonakoShowOpenPanel(const char *defaultDir, const char *extension);
char *gonakoShowSavePanel(const char *defaultDir, const char *defaultName, const char *extension);
char *gonakoShowFolderPanel(const char *defaultDir);
void gonakoFreeDialogResult(char *result);
*/
import "C"

import "unsafe"

// パネルの生成と runModal は AppKit の都合でメインスレッド専用だが、
// filedialog_darwin.m 側がメインスレッド判定と main queue へのディスパッチを
// 行うため、ここはバックグラウンド goroutine から呼んでよい。
// パネルが閉じる（または取り消される）まで、呼び出した goroutine は待つ。
func openFileDialogPlatform(defaultDir, extension string) (string, error) {
	cDefaultDir := C.CString(defaultDir)
	defer C.free(unsafe.Pointer(cDefaultDir))
	cExtension := C.CString(extension)
	defer C.free(unsafe.Pointer(cExtension))

	return goStringAndFree(C.gonakoShowOpenPanel(cDefaultDir, cExtension)), nil
}

func saveFileDialogPlatform(defaultDir, defaultName, extension string) (string, error) {
	cDefaultDir := C.CString(defaultDir)
	defer C.free(unsafe.Pointer(cDefaultDir))
	cDefaultName := C.CString(defaultName)
	defer C.free(unsafe.Pointer(cDefaultName))
	cExtension := C.CString(extension)
	defer C.free(unsafe.Pointer(cExtension))

	return goStringAndFree(C.gonakoShowSavePanel(cDefaultDir, cDefaultName, cExtension)), nil
}

func folderDialogPlatform(defaultDir string) (string, error) {
	cDefaultDir := C.CString(defaultDir)
	defer C.free(unsafe.Pointer(cDefaultDir))

	return goStringAndFree(C.gonakoShowFolderPanel(cDefaultDir)), nil
}

func goStringAndFree(result *C.char) string {
	if result == nil {
		return ""
	}
	defer C.gonakoFreeDialogResult(result)
	return C.GoString(result)
}
