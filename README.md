# nadesiko3go

日本語プログラミング言語「なでしこ3」のGo言語実装です。
現行のTypeScript版を置き換えるものではなく、インストール不要で配布しやすい
CUI/GUIバックエンドを目標にしています。

差分fixtureは **235/236 (99.6%)** 通過しています。残る1件は期待値が
TypeScript版の生成コードに依存しているケースです
（[本家Issue #2456](https://github.com/kujirahand/nadesiko3/issues/2456)）。

## 使う

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

### gonako 自体を配布する

```bash
make release VERSION=1.0.0
```

`bin/` に各プラットフォーム向けの単一バイナリができます。

```
gonako-1.0.0-darwin-arm64      gonako-1.0.0-linux-amd64
gonako-1.0.0-darwin-amd64      gonako-1.0.0-linux-arm64
gonako-1.0.0-windows-amd64.exe
```

## できること

| 分野 | 内容 |
|---|---|
| 言語 | 変数・定数・演算子・制御構文・ユーザー定義関数・クロージャ・エラー監視 |
| 標準命令 | 文字列・配列・辞書・JSON・正規表現・タイマー（`plugin_system` 相当） |
| OS連携 | ファイル読み書き・フォルダ操作・パス操作・環境変数・外部コマンド実行（`plugin_node` 相当） |
| 配布 | プログラムとリソースを単一の実行ファイルに梱包 |

**互換性を保証するのは `plugin_system` の範囲だけ**です。OS連携（`internal/nodelib`）は
Goらしく再設計してよい領域としています（AGENTS.md 3節）。

## 開発

```bash
make test           # テスト
make sync-compat    # 本家の差分fixtureをGo側へ同期
make compat-run     # 全ケースを実行して out/ へ出力
make compat-check   # 本家のoracleと照合して通過率を出す
```

設計と開発上の制約は [AGENTS.md](./AGENTS.md)、VMの詳細は
[docs/vm.md](./docs/vm.md) を参照してください。
残作業は [Issue #9](https://github.com/kujirahand/nadesiko3go/issues/9) にまとめています。
