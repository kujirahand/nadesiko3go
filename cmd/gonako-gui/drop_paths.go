package main

import (
	"encoding/json"

	webview "github.com/webview/webview_go"
)

// ネイティブ側が記録したドロップ元のフルパスを、JavaScriptの
// ドロップイベントの内容に合流させる。WebViewの標準 File にはパスが
// 入らないため、各OSのネイティブ層でドロップ時にパスを取得しておく。

// bindFileDrop はJavaScriptからドロップのフルパス取得を有効にする関数を公開する。
// ウィンドウ(ハンドル0)にドロップ受付が登録されたときに画面側が呼ぶ。
func bindFileDrop(w webview.WebView) {
	_ = w.Bind("enableFileDropPaths", func() {
		platformInstallFileDrop(w.Window())
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
