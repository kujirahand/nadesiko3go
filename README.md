# 日本語プログラミング言語「なでしこ3(Go版)」

日本語プログラミング言語「なでしこ3」のGo言語実装です。基本ライブラリを含めて、本家(TypeScript版)と同じように動きます。
ランタイムのインストール不要で配布しやすい、CUI/GUIバックエンドを目標にしています。

ランタイム兼開発エディタで、ファイル1つだけで動く、`gonako-gui`と、コマンドラインから使う`gonako`を提供しています。

----------------------------

## インストール方法

コマンドラインでコマンドを貼り付けて実行するとスムーズにインストールできます。

### コマンド一発でインストール (ワンライナー)

**macOS / Linux (ターミナル):**

```bash
curl -fsSL https://nadesi.com/install/gonako | bash
```

**Windows (PowerShell):**

```powershell
irm https://nadesi.com/install/gonako | iex
```

### Homebrew (macOS) でインストールする方法

```bash
# Homebrewでtapを追加して、CLI/GUI版をインストール
brew tap kujirahand/nadesiko3 && brew trust kujirahand/nadesiko3
brew install gonako && brew install --cask gonako-gui
```

※ Linuxで `gonako-gui` を実行するには、[ソースコードからビルド](#go言語でソースコードからインストール)して利用してください。

----------------------------

## インストールした後は？

上記の手順でインストールすると、`gonako-gui` というコマンドが使えるようになります。

ターミナル(PowerShell/ターミナル.app)上で、`gonako-gui` とタイプすると、開発用エディタが起動します。
プログラムを書いて、実行ボタンを押せば実行できます。例えば、以下のようなプログラムを書いて実行してみると良いでしょう。

```nako3
# hello.nako3
名前=「お名前は? 」と尋ねる
「お名前は、『{名前}』さんで良いでしょうか？」と二択
もし、それがはいならば：
　　「こんにちは、{名前}さん！」と言う
違えば：
　　「処理を中止します」と表示。
```

また、開発用エディタの左側には、「ひな形」というタブがあるので、そこで「ひな形」を選んで実行してみてください。
なでしこの雰囲気を掴むことができます。

### なでしこ3を学ぶには？

- [なでしこ3のチュートリアル](https://nadesi.com/v3/doc/index.php?%E3%83%81%E3%83%A5%E3%83%BC%E3%83%88%E3%83%AA%E3%82%A2%E3%83%AB&show)
- [なでしこ3の文法](https://nadesi.com/v3/doc/index.php?%E6%96%87%E6%B3%95&show)
- [なでしこ3(gonako)の命令一覧](https://nadesi.com/v3/doc/index.php?gonako&show)

----------------------------



## ターミナルから gonako を使う方法

`gonako`は、コマンドラインから使うCLIツールです。なでしこのプログラムを実行したり、配布ファイルを作ったりできます。

```bash
# ファイルを実行する
gonako hello.nako3
gonako run hello.nako3 引数    # 引数は『コマンドライン』で受け取れる
# その場で実行する
gonako -e '「こんにちは」と表示'
cat hello.nako3 | gonako run - # 標準入力から読む
```

----------------------------

## 命令やマニュアルを検索する

`gonako doc` で、なでしこの命令を調べられます。命令一覧はgonakoに埋め込まれているので、
ネットにつながっていなくても検索できます。

```bash
gonako doc 秒待              # 命令一覧から検索する
gonako doc ファイル 読む      # キーワードを複数書くと、そのすべてに一致する命令を探す
gonako doc 秒待 --web        # Webのマニュアル(https://nadesi.com/v3/doc/)も検索する
gonako doc 秒待 --json       # 結果をJSONで出力する(AIエージェント向け)
```

| オプション | 説明 |
|---|---|
| `--command`, `-c` | 命令一覧(JSON)から検索する（既定の動作） |
| `--web`, `-w` | Webのマニュアルも検索する |
| `--json` | 結果をJSONで出力する |
| `--limit N` | 表示する件数の上限（既定: 20、`0`で全件） |

命令名・読みがな・説明・分類のいずれかに一致した命令を、一致度の高い順に表示します。
各命令には、マニュアルの解説ページ（`https://nadesi.com/v3/doc/index.php?gonako/命令名`）と、
Go版の実装場所も表示されます。

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

タスクランナーには[just](https://github.com/casey/just)を使っています。
`brew install just`（またはお使いのOSのパッケージマネージャ）で導入してください。

```bash
# ビルド
git clone https://github.com/kujirahand/nadesiko3go.git
cd nadesiko3go
just build
# もし各種OSのリリースファイルを生成するなら
just release
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

※ 完全な互換性を保証するのは 標準ライブラリ(`plugin_system`の範囲だけ)です。

### gonako-gui に関して

`gonako-gui`は、GUI(WebView)を持ったなでしこ3の実行ランタイムですが、開発エディタを同梱しています。
`gonako-gui`を実行すると、デフォルトエディタが表示されます。

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

## 実行速度

代表的なアルゴリズム7本で、本家(`cnako3` / Node.js)とVM実行(`gonako`)、
Goコード生成(`gogen`)を比較しています。結果と考察は
**[benchmark/README.md](./benchmark/README.md)** にあります。

| | 合計 | 対 cnako3 |
|---|---:|---:|
| cnako3 (Node.js) | 2713.6 ms | 1.00x |
| gonako (VM実行) | 1921.1 ms | **1.41x** |
| gogen (Goネイティブ) | 880.1 ms | **3.08x** |

**VM実行**は得意・不得意が分かれます。起動の軽さ・文字列処理・辞書・関数呼び出しは
本家より速い（文字列は約5倍、再帰は約2.6倍）一方、数値ループはV8のJITに負けます。

**gogen**（`gonako gengo` → `go build`）は7本すべてで本家より速く、
数値計算では3〜4倍です。生成コードは、型推論で数値と証明できた場所を
生の `float64` で計算します（→ [docs/gogen.md](./docs/gogen.md)）。

ベンチマークは自分の環境でも回せます。

```bash
just cmd                      # gonako本体をビルド
go run ./benchmark/runner.go  # 測定してbenchmark/README.mdを再生成
```

## 開発

必要なGoバージョンとSQLiteドライバは`go.mod`で固定しています。
現在はGo 1.27.0、`modernc.org/sqlite` v1.57.0です。Go 1.21以降を導入済みで
`GOTOOLCHAIN=auto`なら、リポジトリ内で`go`を実行した際にGo 1.27.0が
自動取得・選択されます。

```bash
go version          # go version go1.27.0 ... を確認
just test           # テスト
just doctest        # manualとtestdata/doctestのサンプルを実行
just sync-compat    # 本家の差分fixtureをGo側へ同期
just compat-run     # 全ケースを実行して out/ へ出力
just compat-check   # 本家のoracleと照合して通過率を出す
```

将来、GoとSQLiteを新しい確定版へ更新する場合は、バージョンを明示して実行します。

```bash
go get go@1.27.0
go get modernc.org/sqlite@v1.57.0
go mod tidy
go test ./...
```

## 参考

- 設計と開発上の制約: [AGENTS.md](./AGENTS.md)
- VMの詳細: [docs/vm.md](./docs/vm.md)
- 実行速度の比較: [benchmark/README.md](./benchmark/README.md)
