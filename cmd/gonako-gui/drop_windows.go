//go:build windows

package main

import "unsafe"

// WebView2内部のIDropTargetを差し替える方法は、ウィンドウ起動時にアクセス違反を
// 起こすため使わない。標準のJavaScriptドロップイベントからファイル名を受け取る。
func platformInstallFileDrop(_ unsafe.Pointer) {}

func platformTakeDroppedFiles() []string { return nil }
