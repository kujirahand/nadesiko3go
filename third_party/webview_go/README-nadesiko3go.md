# nadesiko3go向け webview_go

本ディレクトリは `github.com/webview/webview_go` v0.0.0-20240831120633-6173450d4dd6 を基にしています。

Windows版のファイルドロップでフルパスを取得するため、WebView2のWebMessage受信処理を拡張しています。ドロップしたDOM `File` を `postMessageWithAdditionalObjects` で受け取り、`ICoreWebView2File::get_Path` の値をWeb側へ返します。

WebView2 SDKのヘッダーは v1.0.4022.49 のものです。ライセンスと第三者通知は `libs/mswebview2/LICENSE.txt` と `libs/mswebview2/NOTICE.txt` を参照してください。
