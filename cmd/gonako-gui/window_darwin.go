//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

typedef struct {
	int width;
	int height;
	int x;
	int y;
	int state;
	int resizable;
	char *title;
} gonakoWindowInfo;

void gonakoApplyWindowSettings(void *window, int hasPosition, int center, int x, int y,
	int hasState, int state, int hasResizable, int resizable);
gonakoWindowInfo gonakoGetWindowInfo(void *window);
void gonakoFreeWindowTitle(char *title);
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func nativeWindowStateCode(state string) int {
	switch state {
	case "最大化":
		return 1
	case "最小化":
		return 2
	case "全画面":
		return 3
	default:
		return 0
	}
}

func nativeWindowStateName(state int) string {
	switch state {
	case 1:
		return "最大化"
	case 2:
		return "最小化"
	case 3:
		return "全画面"
	default:
		return "通常"
	}
}

func platformApplyWindowSettings(window unsafe.Pointer, settings guilib.WindowSettings) error {
	if window == nil {
		return fmt.Errorf("ネイティブウィンドウを取得できません")
	}
	C.gonakoApplyWindowSettings(
		window,
		C.int(boolInt(settings.HasPosition)), C.int(boolInt(settings.Center)), C.int(settings.X), C.int(settings.Y),
		C.int(boolInt(settings.HasState)), C.int(nativeWindowStateCode(settings.State)),
		C.int(boolInt(settings.HasResizable)), C.int(boolInt(settings.Resizable)),
	)
	return nil
}

func platformWindowInfo(window unsafe.Pointer) (guilib.WindowInfo, error) {
	if window == nil {
		return guilib.WindowInfo{}, fmt.Errorf("ネイティブウィンドウを取得できません")
	}
	info := C.gonakoGetWindowInfo(window)
	if info.title != nil {
		defer C.gonakoFreeWindowTitle(info.title)
	}
	return guilib.WindowInfo{
		Width: int(info.width), Height: int(info.height), X: int(info.x), Y: int(info.y),
		State: nativeWindowStateName(int(info.state)), Title: C.GoString(info.title),
		Resizable: info.resizable != 0,
	}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
