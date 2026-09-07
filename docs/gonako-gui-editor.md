# gonako-gui のエディタと変換機能（設計）

`gonako-gui`（webview_go採用のGUI版）のうち、内蔵エディタの実装と、
ハンバーガーメニューから行う各種変換機能の設計をまとめたドキュメントです。
設計の基準はリポジトリ直下の `AGENTS.md` で、本ドキュメントはそこから
GUI版に関する節を独立させたものです。使い方（コマンドラインオプション等）は
`docs/gonako-gui.md` を参照してください。

- 日本語入力（IME）が死活問題なので、OSネイティブのWebView（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を使う軽量な `webview_go` を採用しています。
- `internal/host` や `internal/vm` をCUI版と共有し、バイナリ埋め込み（`//go:embed`）のWebエディタUIからなでしこプログラムを実行できます。
- 開発エディタは、`cmd/gonako-gui/ui/*`にあるリソースから起動します。実体はgo build時点でコンパイル済みバイナリの中にコピーされ、実行時はembed.FS（メモリ上の仮想FS）から読み出されます。そのため、エディタを変更したら、再ビルドが必要です。

---

## 1. エディタの色分け

`textarea` の背面に色分けした `pre` を重ねる方式で、外部ライブラリは足していません。
差し替えられるよう2つに分けてあります。

- `ui/nako-syntax.js` --- 言語定義。`internal/lexer` の予約語・助詞・字句規則を
  表示用に移植したもので、DOMには触りません。命令名の色分けには
  `command-list.json` を注入します
- `ui/editor-highlight.js` --- 表示部品。`create()` が返す
  `refresh` / `setEnabled` / `destroy` の3つだけを `app.js` が使うので、
  CodeMirror等に置き換えても `app.js` は変えずに済みます

IME変換中は未確定文字が `textarea` にしか無いため、色分けを止めて
`textarea` の文字を見せます（`compositionstart` / `compositionend`）。

---

## 2. フォルダを実行ファイルに変換

ハンバーガーメニューの「このフォルダを実行ファイルに変換」で、開いている
ファイルをメインにして、そのフォルダ以下を1つのアプリに梱包します
（→ `docs/package.md`）。拡張子で種類が決まり、`.nako3` なら起動と同時に
そのプログラムを実行、`.html` ならそのページをWebViewで開くアプリになります。

ランタイムには **gonako-gui 自身のコピー**（`os.Executable()`）を使うので、
**作る側にも受け取る側にもGoツールチェインが要りません**。

OSごとの後始末が要ります。

| OS | 出力 | 追加でやること |
|---|---|---|
| macOS | `app.app` | `ui/macapp/` のひな形から `.app` を組み立てる。`Info.plist` に名前を埋め、アイコンを入れる。署名には触らない（→ `docs/package.md`） |
| Windows | `app.exe` | PEのサブシステムを CUI→GUI に書き換え、コンソール窓が出ないようにする |
| その他 | `app` | なし |

アイコンの持たせ方がOSで違います。macOSは `.app` の中に `AppIcon.icns` を
置くので**アプリごとに差し替えられ**、フォルダに `icon.icns` があれば
それを使います。Windowsはアイコンが実行ファイルのリソース領域に入るため、
`.syso` を **gonako-gui 自身に**持たせています。変換後のexeはそのコピーなので
アイコンを引き継ぎますが、**アプリごとの差し替えはできません**（PEの
リソース書き換えが要る。必要になったら `tc-hib/winres` などを検討）。

アイコンの作り直し方は `scripts/make-app-icon.go` の冒頭に書いてあります。

> Windows向けのビルドは cgo（WebView2）が要るためmacOSからクロスコンパイル
> できません。上のPE書き換えとアイコンは、PEの構造としては検証済みですが、
> **Windows実機での動作確認は未了**です。

---

## 3. Go言語でビルド（gogen連携）

ハンバーガーメニューの「Go言語でビルド」で、開いている `.nako3` を
`internal/gogen` でGoソースに変換し、そのまま `go build` してネイティブ
実行ファイルにします（→ `docs/gogen.md`）。「フォルダを実行ファイルに
変換」と違い、**受け取る側にもGoツールチェインが要ります**（作る仕組み自体
が `go build` だから）。

gogenが生成するコードは `pkg/runtime` だけに依存しますが、それを
`go.mod` の `replace` で指す先として、このリポジトリ自身のソース
チェックアウトが要ります。配布された gonako-gui はバイナリ単体なので、
**実行ファイルと同じフォルダに `nadesiko3go/` フォルダが無ければ、
GitHubの最新masterを自動でダウンロードして展開します**
（`cmd/gonako-gui/gobuild.go`）。2回目以降はダウンロード済みのものを使います。

このメニューが使う命令の組み合わせ（プラグイン）は `gonako gengo` CLIと
同じ既定値ですが、**`guilib`（`ウィンドウ作成`）は含めません**。gogenの
プラグイン表に `guilib` を足すと、CLI版の `gonako` までcgo/WebView2に
依存してしまうため（→ `internal/gogen/plugins.go`）。`ウィンドウ作成`を
使うプログラムは、エディタの「実行」かVM実行（`gonako run`）を使ってください。
