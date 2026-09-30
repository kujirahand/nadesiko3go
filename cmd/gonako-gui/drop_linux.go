//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

// 直近のドロップのパス。NUL区切りのUTF-8で持つ。GTKのシグナルも
// WebViewのバインド呼び出しもメインスレッドで動くので、ロックは不要。
static char *gonakoDroppedBuf = NULL;
static int gonakoDroppedLen = 0;
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
	if (buf == NULL) return;
	free(gonakoDroppedBuf);
	gonakoDroppedBuf = buf;
	gonakoDroppedLen = len;
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

char *gonakoTakeDroppedFiles(int *length) {
	char *buf = gonakoDroppedBuf;
	*length = gonakoDroppedLen;
	gonakoDroppedBuf = NULL;
	gonakoDroppedLen = 0;
	return buf;
}
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
