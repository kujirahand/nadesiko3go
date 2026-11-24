package main

import (
	"fmt"
	"runtime"
)

// guiVersionText は他の gonako コマンドと同じ形式で情報を返す。
func guiVersionText() string {
	return fmt.Sprintf("gonako-gui v%s (%s/%s)", appVersion, runtime.GOOS, runtime.GOARCH)
}

func showGUIVersion() { outputGUIVersion(guiVersionText()) }
