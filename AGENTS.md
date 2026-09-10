# gonako --- なでしこ3(Go言語版)

- 日本語プログラミング言語「なでしこ3」のGo言語実装バージョンである
- 目的は**配布性**。インストール不要のCUI版・GUI版
- **現行のTypeScript版が公式実装**。Go版は置き換えではなく追加のバックエンド
- **互換性を保証する範囲は `plugin_system` のみ**
- **プログラムとリソースを1ファイルに梱包して配布できる**ことを目標に入れる
- **文字列はGoネイティブ（UTF-8 / rune基準）**。UTF-16互換層は作らない
- GUIを提供するgonako-guiでは、WebViewとgonakoをシームレスに連携して、gonakoからWebViewを操作する
- **バージョン番号の定義元は `internal/version/version.go` の1箇所のみ**。更新は必ず
  `just version-update X.Y.Z` で行う（インストーラー・README・リリース手順書も自動で揃う）

---

## エージェントの応答について

- 必ず日本語で応答するようにして
- ソースコードのコメントは日本語で書き込んで

---

## リポジトリのプロジェクト構成

本リポジトリが `nadesiko3go` で、本家リポジトリを `./nadesiko3` にcloneしてある。
`./nadesiko3` は `.gitignore` 対象とし、fixtureだけを `testdata/compat` に同期する。

```text
nadesiko3go/
├── go.mod
├── cmd/
│   ├── gonako/            CUI本体（フル機能）。実行・ビルド・fixture実行のサブコマンド
│   ├── gonako-cui/        軽量CUI版。標準コア+SQLiteのみに絞った別バイナリ（8MB台）
│   └── gonako-gui/        GUI版（webview_go）。段階8
│       └── ui/            埋め込みエディタUI（//go:embed）。→ 9節・`docs/gonako-gui-editor.md`
├── internal/
│   ├── prepare/           前処理（全角記号の正規化など。nako_prepare 相当）
│   ├── lexer/             字句解析（nako_lexer 相当）
│   │   └── josi/          助詞リスト（nako_josi_list 相当）
│   ├── indent/            インデント構文の変換（nako_indent 相当）
│   ├── dncl/              DNCL変換（nako_from_dncl 相当）
│   ├── parser/            構文解析。ASTを作る（nako_parser3 相当）
│   ├── ast/               AST定義
│   ├── ir/                直列化可能IR。バージョン付き。★境界
│   ├── compiler/          AST → IR
│   ├── vm/                IR（バイトコード） → 実行
│   ├── value/             値モデル。★言語の心臓部
│   │   └── text/          文字列のrune基準ヘルパ
│   ├── re/                正規表現エンジンの抽象。RE2とregexp2を差し替える
│   ├── event/             イベントキュー。非同期の実行順を決定的にする
│   ├── errs/              エラー型・文面・ソース位置
│   ├── host/              Host API定義。VMと標準命令の境界
│   ├── stdlib/            plugin_system 相当。★互換保証の対象
│   │   └── system.go, string.go, array.go, dict.go, math.go, datetime.go, json.go, regexp.go
│   ├── nodelib/           plugin_node 相当。ファイル・OS・プロセス・ネットワーク
│   ├── csvlib/            plugin_csv 相当。CSVの読み書き
│   ├── mathlib/           plugin_math 相当。三角関数など数学命令
│   ├── sqlitelib/         nadesiko3-sqlite3 相当。pure GoのSQLite命令
│   ├── officelib/         表計算（Excelブック等）の作成命令。CUI版向け
│   ├── pdflib/            PDF作成命令。CUI版向け
│   ├── imagelib/          決定的なラスタ画像生成命令
│   ├── guilib/            GUI版向け。WebViewウィンドウを開く命令（`ウィンドウ作成`）
│   ├── bundle/            単一ファイル梱包。リソースの仮想ファイルシステム。→ 8節・`docs/package.md`
│   ├── gogen/             Goソース生成バックエンド（段階10・→ 10節・`docs/gogen.md`）。実装済み
│   ├── commanddoc/        命令一覧の埋め込みと検索（`gonako doc`）。→ `docs/doc-search.md`
│   ├── compat/            差分fixtureの実行と結果出力
│   └── doctest/           本家マニュアルのdoctestブロックを実行して結果を照合
├── pkg/
│   └── runtime/           gogenが生成したGoソースの唯一の依存先（→ 10節・`docs/gogen.md`）
├── testdata/
│   └── compat/            本家からコピーした cases/ と expected/、コピー元のSOURCE
├── scripts/
│   └── sync-compat-fixtures.sh
└── docs/
```

`internal/` に置くのは、外部から不用意に依存されないようにするためです。
公開APIが必要な部分だけ `pkg/` へ昇格させます（`docs/gogen.md`）。
`.gitignore` で `pkg/` を無視しないこと。旧GOPATH時代の慣習で無視すると、
段階10で作った `pkg/runtime/` がコミットされません。

**`stdlib/` と `nodelib/` を分けているのが要点**です。前者だけが互換保証の対象で、
後者はGoらしく再設計してよい領域です。混ぜると保証範囲が曖昧になります。
`sqlitelib/` も外部プラグイン相当なので互換保証の対象外ですが、命令名・助詞・
戻り値は `nadesiko3-sqlite3` に合わせます。DB本体は整数ハンドルで管理し、
GoのポインタをValueとして公開しません。

`re/` と `stdlib` の正規表現命令（`regexp.go`）も役割が違います。`re/` は**エンジンの抽象**
（パターンのコンパイル・マッチ・置換と、RE2 / regexp2 の切り替え）だけを持ち、
`stdlib/regexp.go` は**なでしこの命令**（`正規表現マッチ` など、JS形式の
`/pattern/flags` 文字列の解釈と戻り値の形）を持ちます。前者は互換保証の対象外、後者は対象です。

---

## 実装に関する情報

公式のTypeScript版はローカルの `./nadesiko3/` にcloneしてあるので、これを参照して作業を行う。
ただし `./nadesiko3/` は参照用の別リポジトリであり、Go版のコミットには含めない。

### 公式実装との互換性

互換性を保証する範囲（`plugin_system`）と、それを裏付ける差分fixtureの
仕組み、文字列（UTF-16 vs rune基準）の扱い、エラーの種類・行番号・文面、
本家TypeScript版とのやり取りのルールをまとめてあります。
公式実装との差分fixtureの通過率もここにあります。

- 詳細は [`docs/compat.md`](docs/compat.md)

### AST・IR・パーサー

字句解析からAST、そして直列化可能なバイトコードIRに至るまでの設計境界を
まとめてあります。VMは値スタック方式で、`Inst` のオペランドは `A`/`B` の
2つだけという規約もここで決めています。VMの速度計測（ベンチマークの
取り方と過去の最適化の記録）もここにあります。

→ 詳細は [`docs/parser.md`](docs/parser.md)

### 値モデル・Host API

なでしこの値はJavaScriptの値です。`internal/value` の `Kind`/`Value` の
定義、数値・文字列・辞書・配列・日時・多倍長整数の扱い方針、
VMと外界の境界である `internal/host` の `Host` インターフェースを
まとめてあります。

→ 詳細は [`docs/value.md`](docs/value.md)

### 非同期とイベントキュー

goroutineの実行順を仕様にせず、専用のイベントキュー（`internal/event`）で
決定的に回す設計をまとめてあります。時刻の仮想ジャンプ、`Post` した順の
コールバック実行順、`10_async` fixtureとの関係もここにあります。

→ 詳細は [`docs/event-que.md`](docs/event-que.md)

### 単一ファイル梱包

なでしこ1で好評だった「作ったものをそのまま渡せる」を取り戻す部分です。
ランタイム本体の末尾にペイロード（IR + リソース）を追記する方式で、
署名まわりの注意点（追記後にcodesignし直すと壊れる、など）も含めて
まとめてあります。

→ 詳細は [`docs/package.md`](docs/package.md)（運用手順は `docs/bundle-resources.md`）

### GUI版（webview_go）とエディタ

日本語入力（IME）が死活問題なので、OSネイティブのWebView（macOS: WKWebView,
Windows: WebView2, Linux: WebKitGTK）を使う軽量な `webview_go` を採用して
います。`internal/host` や `internal/vm` をCUI版と共有し、バイナリ埋め込み
（`//go:embed`）のWebエディタUIからなでしこプログラムを実行できます。

エディタの色分けの仕組み、「フォルダを実行ファイルに変換」、
「Go言語でビルド」（gogen連携）の設計は次を参照してください。

→ 詳細は [`docs/gonako-gui-editor.md`](docs/gonako-gui-editor.md)（使い方は `docs/gonako-gui.md`）

### Goコード生成バックエンド（gogen）

速度が必要な場面のための追加バックエンドです（#2448 参照）。**実装済み**です。

```text
IR ──► internal/gogen ──► Goソース ──► go build ──► ネイティブ実行ファイル
```

標準命令の実装を二重に持たず、`internal/gogen` が生成するのは制御フロー
だけで、値の演算・添字アクセス・命令呼び出しなどの意味論は `internal/vm`
に委譲します。型推論による非ボックス化や大域変数の昇格といった最適化、
対応範囲（非同期関数のみ非対応）、レジストリの一致が絶対条件であること
もここにまとめてあります。

→ 詳細と使い方は [`docs/gogen.md`](docs/gogen.md)

### 未決事項

決め打ちせず、実装しながら判断する項目です。

- **多倍長整数**: 現行TS版は一部でBigIntを扱う。Go版で `math/big` に載せるか、
  そもそも対象外にするか。当面は対象外とし `KindBigInt` を作らない（→ 4節）
- **正規表現エンジン**: 標準 `regexp`(RE2) で始める。後方参照・先読みが必要になったら
  `dlclark/regexp2` を後付けし、失敗したものだけ流す二段構えにする（#2448）
- **`plugin_node` の命令名**: TS版に寄せる範囲をどこまでにするか
- **オフィス処理などのライブラリ選定**: ライセンス（MIT/BSD系を優先）と日本語の扱いで選ぶ
