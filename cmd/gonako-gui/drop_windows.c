//go:build windows

#define COBJMACROS
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <objidl.h>
#include <oleidl.h>
#include <shellapi.h>
#include <stdlib.h>
#include <string.h>

// WebView2は内部の子ウィンドウにIDropTargetを登録している。それを包む
// IDropTargetに差し替え、ドロップ時にCF_HDROPからフルパスを記録してから
// 元のIDropTargetへ処理を渡す(JavaScriptのdropイベントはその後に届く)。

static CRITICAL_SECTION gonakoDropLock;
static INIT_ONCE gonakoDropLockOnce = INIT_ONCE_STATIC_INIT;
static char *gonakoDroppedBuf = NULL;
static int gonakoDroppedLen = 0;

static BOOL CALLBACK gonakoInitLock(PINIT_ONCE once, PVOID param, PVOID *ctx) {
	InitializeCriticalSection(&gonakoDropLock);
	return TRUE;
}

static void gonakoEnsureLock(void) {
	InitOnceExecuteOnce(&gonakoDropLockOnce, gonakoInitLock, NULL, NULL);
}

typedef struct {
	IDropTarget iface;
	LONG refs;
	IDropTarget *inner;
} gonakoDropTarget;

static const WCHAR *gonakoOleDropProp = L"OleDropTargetInterface";

static void gonakoRecordDrop(IDataObject *data) {
	FORMATETC fmt = {CF_HDROP, NULL, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
	STGMEDIUM medium;
	if (data == NULL || FAILED(IDataObject_GetData(data, &fmt, &medium))) return;
	// CF_HDROPはGlobalLockの戻り値ではなく、hGlobal自体がHDROPハンドル。
	HDROP drop = (HDROP)medium.hGlobal;
	char *buf = NULL;
	int len = 0;
	if (drop != NULL) {
		UINT count = DragQueryFileW(drop, 0xFFFFFFFF, NULL, 0);
		for (UINT i = 0; i < count; i++) {
			UINT wlen = DragQueryFileW(drop, i, NULL, 0);
			WCHAR *wide = (WCHAR *)malloc((wlen + 1) * sizeof(WCHAR));
			if (wide == NULL) continue;
			DragQueryFileW(drop, i, wide, wlen + 1);
			int n = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
			char *grown = n > 0 ? (char *)realloc(buf, len + n) : NULL;
			if (grown != NULL) {
				buf = grown;
				// nに終端のNULを含むので、そのままNUL区切りになる。
				WideCharToMultiByte(CP_UTF8, 0, wide, -1, buf + len, n, NULL, NULL);
				len += n;
			}
			free(wide);
		}
	}
	ReleaseStgMedium(&medium);
	EnterCriticalSection(&gonakoDropLock);
	free(gonakoDroppedBuf);
	gonakoDroppedBuf = buf;
	gonakoDroppedLen = len;
	LeaveCriticalSection(&gonakoDropLock);
}

static HRESULT STDMETHODCALLTYPE gonakoQueryInterface(IDropTarget *self, REFIID riid, void **out) {
	if (IsEqualIID(riid, &IID_IUnknown) || IsEqualIID(riid, &IID_IDropTarget)) {
		*out = self;
		IDropTarget_AddRef(self);
		return S_OK;
	}
	*out = NULL;
	return E_NOINTERFACE;
}

static ULONG STDMETHODCALLTYPE gonakoAddRef(IDropTarget *self) {
	return InterlockedIncrement(&((gonakoDropTarget *)self)->refs);
}

static ULONG STDMETHODCALLTYPE gonakoRelease(IDropTarget *self) {
	gonakoDropTarget *t = (gonakoDropTarget *)self;
	LONG refs = InterlockedDecrement(&t->refs);
	if (refs == 0) {
		IDropTarget_Release(t->inner);
		free(t);
	}
	return refs;
}

static HRESULT STDMETHODCALLTYPE gonakoDragEnter(IDropTarget *self, IDataObject *data, DWORD keys, POINTL pt, DWORD *effect) {
	return IDropTarget_DragEnter(((gonakoDropTarget *)self)->inner, data, keys, pt, effect);
}

static HRESULT STDMETHODCALLTYPE gonakoDragOver(IDropTarget *self, DWORD keys, POINTL pt, DWORD *effect) {
	return IDropTarget_DragOver(((gonakoDropTarget *)self)->inner, keys, pt, effect);
}

static HRESULT STDMETHODCALLTYPE gonakoDragLeave(IDropTarget *self) {
	return IDropTarget_DragLeave(((gonakoDropTarget *)self)->inner);
}

static HRESULT STDMETHODCALLTYPE gonakoDrop(IDropTarget *self, IDataObject *data, DWORD keys, POINTL pt, DWORD *effect) {
	gonakoRecordDrop(data);
	return IDropTarget_Drop(((gonakoDropTarget *)self)->inner, data, keys, pt, effect);
}

static IDropTargetVtbl gonakoDropVtbl = {
	gonakoQueryInterface, gonakoAddRef, gonakoRelease,
	gonakoDragEnter, gonakoDragOver, gonakoDragLeave, gonakoDrop,
};

static BOOL CALLBACK gonakoHookChild(HWND hwnd, LPARAM lparam) {
	IDropTarget *current = (IDropTarget *)GetPropW(hwnd, gonakoOleDropProp);
	if (current == NULL || current->lpVtbl == &gonakoDropVtbl) return TRUE;
	gonakoDropTarget *wrapper = (gonakoDropTarget *)calloc(1, sizeof(gonakoDropTarget));
	if (wrapper == NULL) return TRUE;
	wrapper->iface.lpVtbl = &gonakoDropVtbl;
	wrapper->refs = 1;
	wrapper->inner = current;
	IDropTarget_AddRef(current);
	// RevokeDragDropで元のIDropTargetの参照が外れるので、上でAddRefしてある。
	if (SUCCEEDED(RevokeDragDrop(hwnd))) {
		if (FAILED(RegisterDragDrop(hwnd, &wrapper->iface))) {
			RegisterDragDrop(hwnd, current);
			IDropTarget_Release(current);
			free(wrapper);
		} else {
			// RegisterDragDropが保持する分。wrapperの初期参照は解放して良い。
			IDropTarget_Release(&wrapper->iface);
		}
	} else {
		IDropTarget_Release(current);
		free(wrapper);
	}
	return TRUE;
}

// ウィンドウ配下の全ての子ウィンドウについて、ドロップ受付を差し替える。
// RegisterDragDropを登録したスレッド(UIスレッド)から呼ぶこと。
void gonakoInstallFileDropHook(void *hwnd) {
	gonakoEnsureLock();
	if (hwnd == NULL) return;
	EnumChildWindows((HWND)hwnd, gonakoHookChild, 0);
}

// 記録したパスを取り出して消す。呼び出し側がfreeする。
char *gonakoTakeDroppedFiles(int *length) {
	gonakoEnsureLock();
	EnterCriticalSection(&gonakoDropLock);
	char *buf = gonakoDroppedBuf;
	*length = gonakoDroppedLen;
	gonakoDroppedBuf = NULL;
	gonakoDroppedLen = 0;
	LeaveCriticalSection(&gonakoDropLock);
	return buf;
}
