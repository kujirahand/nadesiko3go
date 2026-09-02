# nadesiko3go

日本語プログラミング言語「なでしこ3」のGo言語実装です。
基本ライブラリを含めて、本家(TypeScript版)と同じように動きます。
現行のTypeScript版を置き換えるものではなく、ランタイムのインストール不要で配布しやすい、CUI/GUIバックエンドを目標にしています。

ランタイム兼開発エディタで、ファイル1つだけで動く、`gonako-gui`と、コマンドラインから使う`gonako`を提供しています。

----------------------------

## インストール方法

コマンドラインでコマンドを貼り付けて実行するとスムーズにインストールできます。

### コマンド一発でインストール (ワンライナー)

**macOS / Linux (ターミナル):**
```bash
curl -fsSL https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.ps1 | iex
```

----------------------------

### Homebrewでインストール

Homebrewからもインストールできます。ただし、エディタ一体型の`gonako-gui`を使う場合には、Gatekeeperのブロックを解除が必要です。

```sh
# Homebrewでtapを追加
brew tap kujirahand/nadesiko3
brew trust kujirahand/nadesiko3
# インストール(CUI + GUI)
brew install gonako          # CLI版
brew install --cask gonako-gui   # GUI版
# Gatekeeperのブロックを解除
xattr -cr /Applications/なでしこ3.app
```

----------------------------

## gonakoを使ってみよう

```bash
make build              # bin/gonako ができる

bin/gonako run hello.nako3        # ファイルを実行する
bin/gonako run hello.nako3 引数    # 引数は『コマンドライン』で受け取れる
bin/gonako -e '「こんにちは」と表示'  # その場で実行する
cat hello.nako3 | bin/gonako run - # 標準入力から読む
```

```nadesiko
# hello.nako3
名前=「お名前は? 」と尋ねる
「こんにちは、{名前}さん」と表示

3回
    「{回数}回目」と表示
ここまで
```

----------------------------

## 作ったものをそのまま渡す

プログラムとリソースを1つの実行ファイルに固められます。

```bash
gonako build かんたんゲーム.nako3 --resource ./images --out かんたんゲーム
```

できあがった `かんたんゲーム` を渡すだけで動きます。
**受け取る側にはランタイムもインストールも要りません。**

同梱したリソースは、**開発中と同じ書き方**で読めます。

```nadesiko
設定=「images/setting.txt」を開く
```

開発中(`gonako run`)は実ファイルを、固めた後は同梱したものを読むので、
プログラムを書き換える必要はありません。

他のOS向けに固めるときは、そのOS向けのランタイムを土台に指定します。
バイト列を後ろに足すだけなので、**固める側にGoツールチェインは要りません**。

```bash
gonako build ゲーム.nako3 --runtime ./gonako-linux-amd64 --out ゲーム-linux
```

> macOSの署名済みバイナリは末尾追記で署名が壊れます。
> 配布時は `codesign` し直してください。

----------------------------

## Go言語でソースコードからインストール

gonako / gonako-gui をソースコードからコンパイルするのも簡単です。以下のコマンドを実行すると、/binフォルダ以下にgonako/gonako-guiが作成されます。

```bash
# ビルド
git clone https://github.com/kujirahand/nadesiko3go.git
cd nadesiko3go
make build
# もし各種OSのリリースファイルを生成するなら
make release VERSION=3.8.1
```

----------------------------

## 「gnako-gui」「gonako」で実現できること

| 分野 | 内容 |
|---|---|
| 言語 | 変数・定数・演算子・制御構文・ユーザー定義関数・クロージャ・エラー監視 |
| 標準命令 | 文字列・配列・辞書・JSON・正規表現・タイマー（`plugin_system` 相当） |
| OS連携 | ファイル読み書き・フォルダ操作・パス操作・環境変数・外部コマンド実行（`plugin_node` 相当） |
| SQLite | DB開閉・切替・SQL実行・単一行/全行取得・位置/名前付きパラメータ・コールバック |
| 配布 | プログラムとリソースを単一の実行ファイルに梱包 |

**互換性を保証するのは `plugin_system` の範囲だけ**です。OS連携（`internal/nodelib`）は
Goらしく再設計してよい領域としています（AGENTS.md 3節）。

### SQLite

SQLiteはgonako本体に組み込まれているため、`取り込む`文や外部ライブラリは不要です。

```nadesiko
「books.sqlite3」をSQLITE3開く
「CREATE TABLE IF NOT EXISTS books(id INTEGER PRIMARY KEY, name TEXT)」を[]でSQLITE3実行
「INSERT INTO books(name) VALUES(?)」を[「クジラ」]でSQLITE3実行

行一覧=「SELECT id,name FROM books ORDER BY id」を[]でSQLITE3全取得
行一覧をJSONエンコード整形して表示
SQLITE3閉じる
```

`SQLITE3開く`の戻り値はDBハンドルです。複数開いた場合は、ハンドルを
`SQLITE3切替`に渡して操作対象を変更できます。SQLite実装はpure Goなので、
CGOや別配布のSQLiteライブラリは不要です。

## 開発

必要なGoバージョンとSQLiteドライバは`go.mod`で固定しています。
現在はGo 1.27.0、`modernc.org/sqlite` v1.57.0です。Go 1.21以降を導入済みで
`GOTOOLCHAIN=auto`なら、リポジトリ内で`go`を実行した際にGo 1.27.0が
自動取得・選択されます。

```bash
go version          # go version go1.27.0 ... を確認
make test           # テスト
make doctest        # manualとtestdata/doctestのサンプルを実行
make sync-compat    # 本家の差分fixtureをGo側へ同期
make compat-run     # 全ケースを実行して out/ へ出力
make compat-check   # 本家のoracleと照合して通過率を出す
```

将来、GoとSQLiteを新しい確定版へ更新する場合は、バージョンを明示して実行します。

```bash
go get go@1.27.0
go get modernc.org/sqlite@v1.57.0
go mod tidy
go test ./...
```

設計と開発上の制約は [AGENTS.md](./AGENTS.md)、VMの詳細は
[docs/vm.md](./docs/vm.md) を参照してください。
残作業は [Issue #9](https://github.com/kujirahand/nadesiko3go/issues/9) にまとめています。
