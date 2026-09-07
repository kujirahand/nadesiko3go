# gonako --- なでしこ3(Go言語版)

なぜ作るのか?

- 日本語プログラミング言語「なでしこ3」のGo言語実装バージョンを作る
- 目的は**配布性**。インストール不要のCUI版・GUI版を作る
- **現行のTypeScript版が公式実装**。Go版は置き換えではなく追加のバックエンド
- ブラウザ版（`wnako3`）はaltJS方式を継続
- **互換性を保証する範囲は `plugin_system` のみ**
- **文字列はGoネイティブ（UTF-8 / rune基準）**。UTF-16互換層は作らない（→ 1節・`docs/compat.md`）
- **プログラムとリソースを1ファイルに梱包して配布できる**ことを目標に入れる
- GUIを提供するgonako-guiでは、WebViewとgonakoをシームレスに連携して、gonakoからWebViewを操作する

---

## エージェントの応答について

- 必ず日本語で応答するようにして
- ソースコードのコメントは日本語で書き込んで

---

公式のTypeScript版はローカルの `./nadesiko3/` にcloneしてあるので、これを参照して作業を行う。
ただし `./nadesiko3/` は参照用の別リポジトリであり、Go版のコミットには含めない。

## 1. 公式実装との互換性

互換性を保証する範囲（`plugin_system`）と、それを裏付ける差分fixtureの
仕組み、文字列（UTF-16 vs rune基準）の扱い、エラーの種類・行番号・文面、
本家TypeScript版とのやり取りのルールをまとめてあります。

**開発中ずっと唯一の進捗指標になる**差分fixtureの通過率もここにあります。

→ 詳細は [`docs/compat.md`](docs/compat.md)

---

## 2. リポジトリ構成

本リポジトリが `nadesiko3go` で、本家リポジトリを `./nadesiko3` にcloneしてある。
`./nadesiko3` は `.gitignore` 対象とし、fixtureだけを `testdata/compat` に同期する。

---

## 3. パッケージ構成

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
公開APIが必要な部分だけ `pkg/` へ昇格させます（→ 10節・`docs/gogen.md`）。
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
`/pattern/flags` 文字列の解釈と戻り値の形）を持ちます。前者は互換保証の対象外、
後者は対象です。標準ライブラリの `regexp` と名前が衝突しないよう、
抽象側は `regexp` ではなく `re` という名前にしてあります。

---

## 4. 値モデル

なでしこの値はJavaScriptの値です。ここを最初に決めないと全部が揺れます。

```go
// internal/value
type Kind uint8

const (
    KindUndefined Kind = iota
    KindNull            // なでしこの「空」
    KindBool
    KindNumber          // float64。JSのnumberと同じ
    KindString          // Goネイティブの文字列(UTF-8)
    KindArray
    KindDict            // 挿入順を保持する
    KindFunc
)

type Value struct {
    data unsafe.Pointer // 文字列・配列・辞書・関数で共用。GC追跡対象
    num  float64        // 数値・真偽値
    aux  uintptr        // 文字列のUTF-8バイト長
    kind Kind
}
```

64bit環境では32バイトです。文字列は `data` に `unsafe.StringData` の返す
バッキング配列へのポインタ、`aux` にバイト長を持たせ、`unsafe.String` で
再構築します。`data` を `uintptr` にしてはいけません。`unsafe.Pointer` のまま
保持することで、文字列を含む参照先がGCの追跡対象になります。

決めておくこと。

- **数値は `float64` 一本**。整数型を別に持つとJSとの差が出る。
  `9007199254740993` が `9007199254740992` になるのも含めて互換
- **文字列はGoネイティブ（UTF-8）**。UTF-16互換層は作らない（→ 1節・`docs/compat.md`）
- **辞書は挿入順を保持**する。Goの `map` は順序を保証しないので、
  `map[string]*Value` に加えてキーの順序を保つスライスを持つ
- 配列は**疎（穴あき）になりうる**。`A=[1]` に `A[3]=9` を代入したときの
  中間要素は `undefined` であって `null` ではない
- 暗黙の型変換（`0` と `"0"` の比較、`+` が加算か連結か）は
  **JSの規則をそのまま移植**する。ここは自分で考えず、差分fixtureに従う
- **日時に専用の型を作らない。** 現行TS版の `plugin_system_datetime` は
  日時を**文字列**で表す（`今日` → `"YYYY/MM/DD"`、`今` → `"HH:mm:ss"`、
  `日時差` などが扱うのは `"YYYY/MM/DD HH:mm:ss"` 形式）。`システム時間` は
  UNIX秒の数値。したがって `KindDate` は不要で、`stdlib/datetime` は
  **文字列と数値の変換だけ**を行う。SPEC.md の値表現に `{"t":"date"}` があるのは
  `Date` オブジェクトが値として漏れた場合の保険であり、`plugin_system` の範囲では
  出現しない（`expected/*.json` に1件もない）
- **多倍長整数は当面サポートしない**（→ 12節）。SPEC.md の `{"t":"bigint"}` も
  現状の期待値には出現しない。必要になった時点で `KindBigInt` を足す

---

## 5. AST・IR・パーサー

字句解析からAST、そして直列化可能なバイトコードIRに至るまでの設計境界を
まとめてあります。VMは値スタック方式で、`Inst` のオペランドは `A`/`B` の
2つだけという規約もここで決めています。VMの速度計測（ベンチマークの
取り方と過去の最適化の記録）もここにあります。

→ 詳細は [`docs/parser.md`](docs/parser.md)

---

## 6. Host API

VMと外界の境界です。**Goのポインタやmapを直接公開しません。**

```go
// internal/host
type Host interface {
    Print(s string)                       // 『表示』の出力先
    Now() time.Time                       // 日時（テストで固定できるように）
    Env() Env                             // ファイル・OS・プロセス・ネットワーク
    Timer() Timer                         // イベントキューへの登録
}
```

CUI版は `os` / `net` / `io` を、GUI版はWebViewへの橋渡しを、
差分fixture実行時は**出力を集めるだけの実装**を差します。
外部境界では整数handleと明示的なValue APIを使い、GC境界を跨がせません。

---

## 7. 非同期とイベントキュー

**goroutineの実行順を仕様にしません。** 専用のイベントキューを1本持ち、
そこに積まれた順・時刻順にシングルスレッドで回します。

```go
// internal/event
type Loop struct { ... }
func (l *Loop) Post(at time.Time, fn func()) TimerID  // 秒後・秒毎
func (l *Loop) Cancel(id TimerID)                     // タイマー停止
func (l *Loop) Run()                                  // 空になるまで回す
```

**時計はイベントループが進めます。** `Run()` は実時間を待たず、
キューの中で最も早い `at` まで**仮想時刻をジャンプさせて**次のコールバックを呼びます。
`Host.Now()` はこの仮想時刻を返します（CUI版は実時刻で初期化し、
`3秒後` を実行しても実際には3秒待たない）。こうしないと `10_async` の
5ケースを実行するだけで待ち時間の総和だけ止まりますし、
実時刻に依存すると結果が決定的になりません。

同じ `at` に複数のコールバックが積まれた場合は **`Post` した順**に呼びます。

差分fixtureの `10_async` グループが、この実行順を固定しています
（`秒後` のコールバックが本体より後に動く、待ち時間の短いものが先に動く、など）。
goroutineを使ってもよいのは、**観測可能な順序に影響しない範囲**だけです。

---

## 8. 単一ファイル梱包

なでしこ1で好評だった「作ったものをそのまま渡せる」を取り戻す部分です。
ランタイム本体の末尾にペイロード（IR + リソース）を追記する方式で、
署名まわりの注意点（追記後にcodesignし直すと壊れる、など）も含めて
まとめてあります。

→ 詳細は [`docs/package.md`](docs/package.md)（運用手順は `docs/bundle-resources.md`）

---

## 9. GUI版（webview_go）とエディタ

日本語入力（IME）が死活問題なので、OSネイティブのWebView（macOS: WKWebView,
Windows: WebView2, Linux: WebKitGTK）を使う軽量な `webview_go` を採用して
います。`internal/host` や `internal/vm` をCUI版と共有し、バイナリ埋め込み
（`//go:embed`）のWebエディタUIからなでしこプログラムを実行できます。

エディタの色分けの仕組み、「フォルダを実行ファイルに変換」、
「Go言語でビルド」（gogen連携）の設計は次を参照してください。

→ 詳細は [`docs/gonako-gui-editor.md`](docs/gonako-gui-editor.md)（使い方は `docs/gonako-gui.md`）

---

## 10. Goコード生成バックエンド（gogen）

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

---

## 11. 実装手順

各段階の完了条件を**差分fixtureの通過率**で定義します。

| 段階 | 内容 | 完了条件 |
|---|---|---|
| 0 | fixtureのGo側同期、`value` / `text` / `ir` / `host` の定義、`compat run` の器 | `compat run` が全ケースを `status: "error"` / `UnsupportedError` として出力し、`compat:check` が「未実行 0件」で動く |
| 1 | `prepare` / `lexer` / `parser`。ASTまで | `09_error` を除く全ケースがエラーなく解析でき、`09_error` の4件（`NakoSyntaxError` 3件・`NakoLexerError` 1件）は**期待どおりの種別**でエラーになる |
| 2 | `compiler`（AST→IR）と `vm`。`stdlib` の中核（表示・演算・変数・制御構文） | `01`〜`03` `07` グループが通過 |
| 3 | `stdlib` の残り（文字列・配列・辞書・数学・日時・JSON）と `errs` | `04`〜`09` グループが通過。全体9割 |
| 4 | `re` / `stdlib/regexp`（RE2の範囲） | `11` グループが通過（`unsupported` 3件・`intentionalDiff` 1件を除く） |
| 5 | `event` と非同期命令 | `10` グループが通過。**全グループ通過（非対応3件・意図的差異2件を除く）** |
| 6 | `nodelib`。CUI版の完成 | 実用スクリプトが動く。単一バイナリを配布できる |
| 7 | `bundle`。プログラムとリソースの単一ファイル梱包 | `gonako build` の成果物が別マシンで動く |
| 8 | GUI版（webview_go） | IMEで日本語入力ができる |
| 9 | オフィス処理・PDF作成・画像生成 | 各機能のテストが通る |
| 10 | `gogen`。Goコード生成バックエンド | **完了。** 3系統の差分fixtureが一致する（非同期関数を除く230/230件、`GOGEN_COMPAT=1 go test ./internal/gogen/`） |

**段階6で単体の価値が出ます**（インストール不要のCUI版）。
**段階7で当初の目的が達成されます**（作ったものをそのまま渡せる）。
段階8以降はそれぞれ独立して判断できます。

段階4を段階5より前に置いたのは、`11_regexp` の11件が段階3の「全体9割」に
含まれておらず、そのままだと**どの段階にも割り当てられないまま
段階5の「全グループ通過」だけが残る**からです。後方参照・先読みの3件は
`unsupported` のままでよく、RE2で通る範囲を通せば完了とします。

### 進め方

1. まず落ちているケースを1つ選ぶ
2. 通すために必要な最小の実装を書く
3. `compat run` して通過率を見る
4. 下がっていないことを確認して次へ

順番に迷ったら、**グループ番号の小さい順**に潰すのが安全です。
`01_literal` → `02_operator` → `03_type_convert` の順に、
言語の土台から固まっていくように並べてあります。

---

## 12. 未決事項

決め打ちせず、実装しながら判断する項目です。

- **多倍長整数**: 現行TS版は一部でBigIntを扱う。Go版で `math/big` に載せるか、
  そもそも対象外にするか。当面は対象外とし `KindBigInt` を作らない（→ 4節）
- **正規表現エンジン**: 標準 `regexp`(RE2) で始める。後方参照・先読みが必要になったら
  `dlclark/regexp2` を後付けし、失敗したものだけ流す二段構えにする（#2448）
- **`plugin_node` の命令名**: TS版に寄せる範囲をどこまでにするか
- **オフィス処理などのライブラリ選定**: ライセンス（MIT/BSD系を優先）と日本語の扱いで選ぶ
