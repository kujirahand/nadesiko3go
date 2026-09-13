//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	int width;
	int height;
	int x;
	int y;
	int state;
	int resizable;
	char *title;
} gonakoWindowInfo;

static void gonakoRestoreGtkWindow(GtkWindow *window) {
	gtk_window_deiconify(window);
	gtk_window_unfullscreen(window);
	gtk_window_unmaximize(window);
}

static void gonakoCenterGtkWindow(GtkWindow *window) {
	GdkWindow *gdkWindow = gtk_widget_get_window(GTK_WIDGET(window));
	if (gdkWindow == NULL) {
		gtk_window_set_position(window, GTK_WIN_POS_CENTER);
		return;
	}
	GdkScreen *screen = gtk_window_get_screen(window);
	int monitor = gdk_screen_get_monitor_at_window(screen, gdkWindow);
	GdkRectangle workarea;
	gdk_screen_get_monitor_workarea(screen, monitor, &workarea);
	int width = 0;
	int height = 0;
	gtk_window_get_size(window, &width, &height);
	gtk_window_move(window,
		workarea.x + (workarea.width - width) / 2,
		workarea.y + (workarea.height - height) / 2);
}

static void gonakoApplyGtkWindowSettings(void *windowPtr, int hasPosition, int center, int x, int y,
		int hasState, int state, int hasResizable, int resizable) {
	GtkWindow *window = GTK_WINDOW(windowPtr);
	if (window == NULL) return;
	if (hasState && state == 0) gonakoRestoreGtkWindow(window);
	if (hasResizable) gtk_window_set_resizable(window, resizable != 0);
	if (hasPosition) {
		if (center) gonakoCenterGtkWindow(window);
		else gtk_window_move(window, x, y);
	}
	if (!hasState || state == 0) return;
	if (state == 1) gtk_window_maximize(window);
	else if (state == 2) gtk_window_iconify(window);
	else if (state == 3) gtk_window_fullscreen(window);
}

static gonakoWindowInfo gonakoGetGtkWindowInfo(void *windowPtr) {
	GtkWindow *window = GTK_WINDOW(windowPtr);
	gonakoWindowInfo result = {0};
	if (window == NULL) return result;
	gtk_window_get_size(window, &result.width, &result.height);
	gtk_window_get_position(window, &result.x, &result.y);
	result.resizable = gtk_window_get_resizable(window);
	GdkWindow *gdkWindow = gtk_widget_get_window(GTK_WIDGET(window));
	if (gdkWindow != NULL) {
		GdkWindowState state = gdk_window_get_state(gdkWindow);
		if (state & GDK_WINDOW_STATE_ICONIFIED) result.state = 2;
		else if (state & GDK_WINDOW_STATE_FULLSCREEN) result.state = 3;
		else if (state & GDK_WINDOW_STATE_MAXIMIZED) result.state = 1;
	}
	const char *title = gtk_window_get_title(window);
	result.title = strdup(title == NULL ? "" : title);
	return result;
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func platformApplyWindowSettings(window unsafe.Pointer, settings guilib.WindowSettings) error {
	if window == nil {
		return fmt.Errorf("ネイティブウィンドウを取得できません")
	}
	C.gonakoApplyGtkWindowSettings(
		window,
		C.int(windowBoolInt(settings.HasPosition)), C.int(windowBoolInt(settings.Center)), C.int(settings.X), C.int(settings.Y),
		C.int(windowBoolInt(settings.HasState)), C.int(windowStateCode(settings.State)),
		C.int(windowBoolInt(settings.HasResizable)), C.int(windowBoolInt(settings.Resizable)),
	)
	return nil
}

func platformWindowInfo(window unsafe.Pointer) (guilib.WindowInfo, error) {
	if window == nil {
		return guilib.WindowInfo{}, fmt.Errorf("ネイティブウィンドウを取得できません")
	}
	info := C.gonakoGetGtkWindowInfo(window)
	if info.title != nil {
		defer C.free(unsafe.Pointer(info.title))
	}
	return guilib.WindowInfo{
		Width: int(info.width), Height: int(info.height), X: int(info.x), Y: int(info.y),
		State: windowStateName(int(info.state)), Title: C.GoString(info.title), Resizable: info.resizable != 0,
	}, nil
}

func windowBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func windowStateCode(state string) int {
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

func windowStateName(state int) string {
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
