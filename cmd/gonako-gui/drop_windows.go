//go:build windows

package main

/*
#cgo CXXFLAGS: -std=c++14 -I${SRCDIR}/../../third_party/webview_go/libs/mswebview2/include
#cgo LDFLAGS: -luuid
long gonakoInstallFileDropListener(void *controller, const char *origin);
*/
import "C"

import (
	"fmt"
	"unsafe"

	webview "github.com/webview/webview_go"
)

// WebView2の公開コントローラーから、ファイルのフルパスを返すイベントを登録する。
func platformInstallFileDrop(w *webview.Protected) error {
	controller := w.NativeHandle(webview.NativeHandleBrowserController)
	if controller == nil {
		return fmt.Errorf("WebView2のコントローラーを取得できません")
	}
	origin := C.CString(w.TrustOrigin())
	defer C.free(unsafe.Pointer(origin))
	result := C.gonakoInstallFileDropListener(controller, origin)
	if result < 0 {
		return fmt.Errorf("ファイルドロップ受付の登録に失敗しました (HRESULT=0x%08X)", uint32(result))
	}
	return nil
}

func platformTakeDroppedFiles() []string { return nil }
