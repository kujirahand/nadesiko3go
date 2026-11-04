# gonako/gonako-guiをLinuxでビルドする手順

Linuxでgonako-gui（GUI版）および `internal/guilib` をビルド・テストする際の手順をまとめたものです。

CUI版（`cmd/gonako`）はcgoを使わないため特別な準備は要りませんが、GUI版は同梱の
`webview_go` がcgo経由でGTKとWebKitGTKを使うため、開発ライブラリが必要です。
`webview_go` のcgo指示（`third_party/webview_go/webview.go`）がpkg-configで要求するのは
`gtk+-3.0` と `webkit2gtk-4.0` の2つです（`cmd/gonako-gui` 側のLinuxファイルも
`gtk+-3.0` を要求します）。

これらが無い環境では、次のようなエラーでビルドできません。

```text
Package 'gtk+-3.0', required by 'virtual:world', not found
Package 'webkit2gtk-4.0', required by 'virtual:world', not found
```

このドキュメントは Ubuntu 24.04（noble）x86_64 での手順です。Ubuntu 24.04 には
WebKitGTK 4.0 の開発パッケージが存在しないため、後述するpkg-configエイリアスによる
回避策を含みます。

---

## 前提条件

- Ubuntu 24.04（x86_64）
- Go、`gcc` / `g++`、`pkg-config`、`git` がインストール済みであること（cgoのビルドに必要）

---

## 1. 開発ライブラリのインストール

まずパッケージ一覧を更新してから、開発ライブラリをインストールします。

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

インストールするのは `libwebkit2gtk-4.0-dev` ではなく **`libwebkit2gtk-4.1-dev`** です
（理由は次節）。2026-11-04 時点の検証環境では、この指定で新規に約120パッケージが
入りました（`--no-install-recommends` を付けた最小構成でもビルドできることを確認済みです）。

> **注意**: `apt-get update` を先に実行してください。パッケージ一覧が古いままだと、
> `libgtk-3-dev` にインストール候補が表示されず「導入できない」ように見えます。

### なぜ 4.0 ではなく 4.1 なのか

Ubuntu 24.04（noble）では WebKitGTK 4.0 API が削除され、`libwebkit2gtk-4.0-dev`
パッケージ自体が存在しません。提供されているのは 4.1 系統の
`libwebkit2gtk-4.1-dev` のみです。一方、`webview_go` は `webkit2gtk-4.0` を
名指しで要求するため、このままでは `pkg-config` が解決できません。
そこで次節のとおり、`webkit2gtk-4.0` の名前で 4.1 を指すpkg-configエイリアスを作成します。

4.1 で代用できる根拠は、同梱webviewのC++実装
（`third_party/webview_go/libs/webview/include/webview.h`）が、WebKitの関数を
リンク時に固定せず、実行時に動的シンボル解決していることです。あわせて実行時の
WebKitバージョンを見て `webkit_web_view_evaluate_javascript` と
`webkit_web_view_run_javascript` を切り替える作りになっているため、4.1 の
ヘッダ・ライブラリでビルドでき、動作にも問題ないことを実測で確認しています。

---

## 2. webkit2gtk-4.0 のpkg-configエイリアスを作成する

`/usr/lib/x86_64-linux-gnu/pkgconfig/webkit2gtk-4.0.pc` を、次の内容で作成します。
これは 4.1 の `.pc` ファイルを土台に、`Libs`・`Cflags`・`Requires` を 4.1 へ
向けたものです。

```text
prefix=/usr
exec_prefix=${prefix}
libdir=/usr/lib/x86_64-linux-gnu
includedir=${prefix}/include
revision=tarball

Name: WebKitGTK (4.0 alias to 4.1 on Ubuntu 24.04)
Description: Web content engine for GTK - webkit2gtk-4.0 compatibility alias pointing at 4.1 (noble has no 4.0 dev package)
URL: https://webkitgtk.org
Version: 2.52.6
Requires: glib-2.0 gtk+-3.0 libsoup-3.0 javascriptcoregtk-4.1
Libs: -L${libdir} -lwebkit2gtk-4.1
Cflags: -I${includedir}/webkitgtk-4.1
```

> **注意**: `Version:` は、実際にインストールした `libwebkit2gtk-4.1-dev` の
> バージョンに合わせてください（上記は検証環境の 2.52.6 の例です）。
> `pkg-config --modversion webkit2gtk-4.1` で確認できます。

作成できたら、pkg-configで両方とも解決できることを確認します。

```bash
pkg-config --modversion gtk+-3.0 webkit2gtk-4.0
```

```text
3.24.41
2.52.6
```

（バージョン番号は環境によって異なります。）

---

## 3. ビルドとテスト

リポジトリのルートで実行します。

```bash
go build ./internal/guilib/ ./cmd/gonako-gui/
go test ./internal/guilib/ ./cmd/gonako-gui/
```

ビルド時にGTKの非推奨APIに関する警告（`'gdk_screen_get_monitor_at_window' is deprecated` など）
が出ることがありますが、無害です。

---

## 注意

- 表示デバイス（X / Wayland）が無いヘッドレス環境でも、ビルドとテストは実行できます。
  ただし実ウィンドウを開く目視確認はできないため、画面の確認は表示デバイスのある
  環境（実機）で行ってください。
- エイリアスの `.pc` ファイルはシステム側（`/usr/lib/...`）に置くファイルなので、
  OSの再インストールやVMの入れ替え後は再作成が必要です。その場合はこのドキュメントの
  手順を最初から実行し直してください。

---

## 検証環境

このドキュメントの手順は、Ubuntu 24.04 x86_64 のVMで次の状態を確認しています
（2026-11-04 実測）。

- `libgtk-3-dev` 3.24.41-4ubuntu1.3（`gtk+-3.0` 3.24.41）
- `libwebkit2gtk-4.1-dev` 2.52.6-0ubuntu0.24.04.1（`webkit2gtk-4.1` 2.52.6）
- エイリアス `webkit2gtk-4.0.pc` を配置し、`pkg-config --modversion gtk+-3.0 webkit2gtk-4.0`
  が `3.24.41` / `2.52.6` を返すことを確認
- `go build ./internal/guilib/ ./cmd/gonako-gui/` が成功すること（非推奨警告のみ）
- `go test ./internal/guilib/ ./cmd/gonako-gui/` が全件合格すること

---

## 参考リンク

- [build-gonako-windows.md](build-gonako-windows.md): Windowsでビルドする手順
- [gonako-gui.md](gonako-gui.md): gonako-guiの使い方
- [WebKitGTK](https://webkitgtk.org/)
- [Go cgoコマンドの説明](https://go.dev/cmd/cgo/)
