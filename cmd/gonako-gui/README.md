# gonako-gui

なでしこ3 Go言語版の軽量WebViewベースGUIエディタ＆実行環境です。

OS標準のWebView（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を利用し、日本語IMEに完全対応したクリーンなデスクトップGUIを提供します。言語実行本体は `internal/` をCUI版と共有しています。

## ビルド方法

```bash
# GUIバイナリをビルド (bin/gonako-gui)
make gui
```

## 起動方法

```bash
./bin/gonako-gui
```

## 主な機能
- **エディタ機能**: 行番号表示、Tabインデント対応、文字数・カーソル位置表示
- **ショートカットキー**: `Ctrl+Enter` / `Cmd+Enter` / `Ctrl+R` / `Cmd+R` で即時実行
- **サンプル選択**: 基本構文、FizzBuzz、辞書・配列、ハッシュ値計算、CSV処理などのサンプルをワンクリックで読み込み
- **実行結果表示**: 標準出力・エラー出力を色分け表示、実行時間の計測、結果コピー機能
