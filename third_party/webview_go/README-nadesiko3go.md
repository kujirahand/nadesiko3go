# nadesiko3go向け webview_go

本ディレクトリは `github.com/webview/webview_go` v0.0.0-20240831120633-6173450d4dd6 を基にしています。

Go側に汎用API `NativeHandle(kind NativeHandleKind) unsafe.Pointer` と種類の定数だけを追加しています。内部のC API `webview_get_native_handle` をそのまま公開し、ウィンドウ・ブラウザー部品・ブラウザーコントローラーを取得できます。返すポインタは借用で、UIスレッドから使い、呼び出し側で解放しないでください。

C++実装と同梱WebView2ヘッダーは元のモジュールと同じです。Windowsのファイルドロップ処理、WebMessageの受信、追加COMインターフェースの定義は `cmd/gonako-gui/drop_windows.*` にあります。

Linuxのpkg-configは元の `webkit2gtk-4.0` 固定から、`gtk_pc_4_0.go`（既定）と `gtk_pc_4_1.go`（`-tags webkit2gtk_4_1`）に分けています。Ubuntu 24.04のように4.0の開発パッケージが無い環境でも、4.1のヘッダでビルドできます。実行時は従来どおり `libwebkit2gtk-4.1.so` と `libwebkit2gtk-4.0.so` の両方を探します。タグの選択はリポジトリ直下の `scripts/webkit-gtk-tag.sh` が行います。

`libs/mswebview2/include/WebView2.h` は、MinGWでは `<eventtoken.h>` を取り込むようにしてあります。Windows SDKのファイル名は `EventToken.h` で、Linux上のクロスコンパイルでは大文字小文字が区別されて見つからないためです。
