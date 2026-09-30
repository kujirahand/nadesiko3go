package main

import (
	"encoding/json"

	webview "github.com/webview/webview_go"
)

// 標準のFileにはフルパスがないため、各OSのネイティブ層で取得する。
// WindowsはWebMessageの返信で直接渡し、macOS・Linuxは記録したパスを
// Goが受け取るドロップイベントへ合流させる。

// bindFileDrop はJavaScriptからドロップのフルパス取得を有効にする関数を公開する。
// ウィンドウ(ハンドル0)にドロップ受付が登録されたときに画面側が呼ぶ。
func bindFileDrop(w webview.WebView) {
	installed := false
	_ = w.Bind("enableFileDropPaths", func() error {
		if installed {
			return nil
		}
		// BindのコールバックはUIスレッド上で動く。登録が終わってから
		// Promiseを解決し、同じウィンドウには一度だけ登録する。
		if err := platformInstallFileDrop(w); err != nil {
			return err
		}
		installed = true
		return nil
	})
}

// splitDroppedPaths はNUL区切りのUTF-8バイト列をパスの配列へ戻す。
func splitDroppedPaths(buf []byte) []string {
	var paths []string
	start := 0
	for i, b := range buf {
		if b == 0 {
			if i > start {
				paths = append(paths, string(buf[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(buf) {
		paths = append(paths, string(buf[start:]))
	}
	return paths
}

// withDroppedFiles はdropイベントの値へ、ネイティブ側で取れたフルパスを反映する。
// 取れなかった場合(未対応環境など)は、JavaScriptが渡したファイル名のままにする。
func withDroppedFiles(event string, values map[string]string) map[string]string {
	if event != "drop" {
		return values
	}
	paths := platformTakeDroppedFiles()
	if len(paths) == 0 {
		return values
	}
	encoded, err := json.Marshal(paths)
	if err != nil {
		return values
	}
	merged := make(map[string]string, len(values)+1)
	for k, v := range values {
		merged[k] = v
	}
	merged["__gonako_drop_files"] = string(encoded)
	return merged
}
