//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#cgo LDFLAGS: -pthread
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>
#include "drop_buffer_linux.h"

// ドロップのパス(NUL区切りのUTF-8)の受け渡しは drop_buffer_linux.c の
// FIFOキューが担う。記録はGTKのUIスレッド、取得はGoのイベント処理
// ワーカーと、動くスレッドが異なるため、共有はキュー側のmutexで守る。
// ここに残る static はフック済みビューの記録だけで、UIスレッドからしか
// 触らない。
static GtkWidget *gonakoHookedView = NULL;

static void gonakoDragDataReceived(GtkWidget *widget, GdkDragContext *context, gint x, gint y,
		GtkSelectionData *data, guint info, guint time, gpointer user) {
	gchar **uris = gtk_selection_data_get_uris(data);
	if (uris == NULL) return;
	char *buf = NULL;
	int len = 0;
	for (int i = 0; uris[i] != NULL; i++) {
		gchar *path = g_filename_from_uri(uris[i], NULL, NULL);
		if (path == NULL) continue;
		int n = (int)strlen(path) + 1;
		char *grown = (char *)realloc(buf, len + n);
		if (grown != NULL) {
			buf = grown;
			memcpy(buf + len, path, n);
			len += n;
		}
		g_free(path);
	}
	g_strfreev(uris);
	if (buf == NULL) {
		// URIのドロップだが変換できるパスがなかった。何も積まないと、
		// このイベントの取得が次のドロップのパスを取ってしまうので、
		// 「パスなし」のマーカー項目を積んで対応を保つ。
		gonakoDropQueuePush(NULL, 0);
		return;
	}
	// 組み立てたbufferの所有権をキューへ移す。失敗したときだけ自分で捨てる。
	if (!gonakoDropQueuePush(buf, (size_t)len)) {
		free(buf);
	}
}

static void gonakoFindWebView(GtkWidget *widget, gpointer data) {
	GtkWidget **found = (GtkWidget **)data;
	if (*found != NULL) return;
	if (strcmp(G_OBJECT_TYPE_NAME(widget), "WebKitWebView") == 0) {
		*found = widget;
		return;
	}
	if (GTK_IS_CONTAINER(widget)) {
		gtk_container_forall(GTK_CONTAINER(widget), gonakoFindWebView, data);
	}
}

// WebKitWebViewのdrag-data-receivedを、WebKit本来の処理より先に受けてパスを記録する。
// g_signal_connectで繋ぐとクラスハンドラ(=WebKitの処理)より前に呼ばれる。
void gonakoInstallFileDropHook(void *windowPtr) {
	if (windowPtr == NULL) return;
	GtkWidget *view = NULL;
	gonakoFindWebView(GTK_WIDGET(windowPtr), &view);
	if (view == NULL || view == gonakoHookedView) return;
	gonakoHookedView = view;
	g_signal_connect(view, "drag-data-received", G_CALLBACK(gonakoDragDataReceived), NULL);
}
*/
import "C"

import (
	"unsafe"

	webview "github.com/webview/webview_go"
)

func platformInstallFileDrop(w *webview.Protected) error {
	C.gonakoInstallFileDropHook(w.Window())
	return nil
}

// platformTakeDroppedFiles は最も古い未取得ドロップのパスを返す。
// 取り出したbufferの所有権はここへ移り、コピーし終えたら1回だけ free する。
func platformTakeDroppedFiles() []string {
	var length C.int
	buf := C.gonakoTakeDroppedFiles(&length)
	if buf == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(buf))
	return splitDroppedPaths(C.GoBytes(unsafe.Pointer(buf), length))
}
