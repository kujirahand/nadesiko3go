# nadesiko3go向け webview_go

本ディレクトリは `github.com/webview/webview_go` v0.0.0-20240831120633-6173450d4dd6 を基にしています。

Go側に汎用API `NativeHandle(kind NativeHandleKind) unsafe.Pointer` と種類の定数だけを追加しています。内部のC API `webview_get_native_handle` をそのまま公開し、ウィンドウ・ブラウザー部品・ブラウザーコントローラーを取得できます。返すポインタは借用で、UIスレッドから使い、呼び出し側で解放しないでください。

C++実装と同梱WebView2ヘッダーは元のモジュールと同じです。Windowsのファイルドロップ処理、WebMessageの受信、追加COMインターフェースの定義は `cmd/gonako-gui/drop_windows.*` にあります。
