//go:build windows

package main

/*
#cgo CXXFLAGS: -std=c++14 -I${SRCDIR}/../../third_party/webview_go/libs/mswebview2/include
#cgo LDFLAGS: -luuid
#include <stdlib.h>
long gonakoInstallFileDropListener(void *controller, const char *origin, void *trustHandle);
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
	// trustが失効した後にlistenerがローカルパスを返さないよう、native trustの
	// 照会ハンドルも渡す。
	result := C.gonakoInstallFileDropListener(controller, origin, w.NativeTrustHandle())
	if result < 0 {
		return fmt.Errorf("ファイルドロップ受付の登録に失敗しました (HRESULT=0x%08X)", uint32(result))
	}
	return nil
}

// gonakoTrustInstalled は drop_windows.cc から native trust の現在値を照会する
// 入口である。cgoの //export は同じ package の C オブジェクトに定義を置くため、
// cmd/gonako-gui の C++ が他 package の C シンボルを直接参照しなくて済む。
//
//export gonakoTrustInstalled
func gonakoTrustInstalled(handle unsafe.Pointer) C.int {
	if webview.TrustInstalled(handle) {
		return 1
	}
	return 0
}

func platformTakeDroppedFiles(_ []string) []string { return nil }
