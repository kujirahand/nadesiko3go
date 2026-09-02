# gonako --- なでしこ3 Go言語版 設計メモ

なぜ作るのか?

- 日本語プログラミング言語「なでしこ3」のGo言語実装バージョンを作る
- 目的は速度ではなく**配布性**。インストール不要のCUI版・GUI版を得る
- **現行のTypeScript版が公式実装**。Go版は置き換えではなく追加のバックエンド
- ブラウザ版（`wnako3`）はaltJS方式を継続。WASMは対象外
- **互換性を保証する範囲は `plugin_system` のみ**
- **文字列はGoネイティブ（UTF-8 / rune基準）**。UTF-16互換層は作らない（→ 5節）
- **プログラムとリソースを1ファイルに梱包して配布できる**ことを目標に入れる

---

公式のTypeScript版はローカルの `./nadesiko3/` にcloneしてあるので、これを参照して作業を行う。
ただし `./nadesiko3/` は参照用の別リポジトリであり、Go版のコミットには含めない。

## 1. 差分fixtureについて

コードより先に「何が同じなら正しいのか」を固定します。

```text
nadesiko3/core/test/fixtures/compat/
  SPEC.md              データ形式の仕様（言語非依存）
  cases/*.json         ケース定義（人が書く）
  expected/*.json      期待値。TypeScript版から自動生成
  compat_case.mjs      読み込み・実行・値の正規化
  make_compat_golden.mjs   期待値を生成する
  check_compat.mjs     別実装の結果と突き合わせ、通過率を出す
nadesiko3/core/test/compat_fixture_test.mjs   期待値が勝手に変わらないことを守る回帰テスト
```

Go版から見た使い方は次のとおりです。

```bash
# 1. ケースを読み、1件ずつ実行して SPEC.md の形式でJSONに書き出す（Go側の仕事）
gonako compat run --cases ./testdata/compat/cases --out ./out

# 2. 通過率を見る（なでしこ3リポジトリ側のツール）
(cd nadesiko3 && npm run compat:check -- /path/to/out)
#   合計: 180/236 通過 (76.3%)  不一致 56件 / 未実行 0件 / 非対応 3件 / 意図的差異 2件
```

分母の `236` は、ケース総数 `241` から `unsupported`(3件) と `intentionalDiff`(2件) を
引いた**集計対象の件数**です。ケースを足せばこの数は動きます。

fixtureは `scripts/sync-compat-fixtures.sh` で本家からコピーし、コピー元の
コミットを `testdata/compat/SOURCE` に記録する。`cases/` と `expected/` を
手で片方だけ更新しないこと。

**この通過率が、開発中ずっと唯一の進捗指標になります。** 実装 → 実行 → 通過率、
という短いループが回るので、コーディングエージェントに任せられる形になります。

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
│       └── ui/            埋め込みエディタUI（//go:embed）。→ 11節
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
│   ├── bundle/            単一ファイル梱包。リソースの仮想ファイルシステム。→ 10節
│   ├── gogen/             Goソース生成バックエンド（段階10・→ 12節）。実装済み
│   ├── compat/            差分fixtureの実行と結果出力
│   └── doctest/           本家マニュアルのdoctestブロックを実行して結果を照合
├── pkg/
│   └── runtime/           gogenが生成したGoソースの唯一の依存先（→ 12節）
├── testdata/
│   └── compat/            本家からコピーした cases/ と expected/、コピー元のSOURCE
├── scripts/
│   └── sync-compat-fixtures.sh
└── docs/
```

`internal/` に置くのは、外部から不用意に依存されないようにするためです。
公開APIが必要な部分だけ `pkg/` へ昇格させます（→ 12節）。
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
    kind Kind
    num  float64
    str  string    // Goの文字列をそのまま持つ
    arr  *Array
    dict *Dict
    fn   *Func
}
```

決めておくこと。

- **数値は `float64` 一本**。整数型を別に持つとJSとの差が出る。
  `9007199254740993` が `9007199254740992` になるのも含めて互換
- **文字列はGoネイティブ（UTF-8）**。UTF-16互換層は作らない（→ 5節）
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
- **多倍長整数は当面サポートしない**（→ 15節）。SPEC.md の `{"t":"bigint"}` も
  現状の期待値には出現しない。必要になった時点で `KindBigInt` を足す

---

## 5. 文字列 --- Goネイティブ（UTF-8 / rune基準）

**Go版の文字列はGoの `string`（UTF-8）をそのまま使い、UTF-16互換層は作りません。**

当初はUTF-16互換層が必要だと考えていましたが、現行のTypeScript版を調べたところ、
`core/src/plugin_system_string.mts` の文字列命令は**すでに `Array.from` ベースで
コードポイント単位に揃えられていました**（`String.fromCodePoint` / `codePointAt` も同様）。
つまりサロゲートペアを考慮した実装になっており、**Goのrune基準とそのまま一致します。**

実測で確認した対応状況です（`"𩸽あ"` などで検証）。

| 命令 | TS版 | Go版(rune基準) | |
|---|---|---|---|
| `文字数` / `文字抜出` / `文字左部分` / `文字右部分` | コードポイント単位 | 同じ | ○ 一致 |
| `文字検索` / `何文字目` / `文字挿入` / `文字削除` | コードポイント単位 | 同じ | ○ 一致 |
| `文字列分解` | コードポイント単位 | 同じ | ○ 一致 |
| `置換` / `単置換` / `出現回数` | `split`/`join` ベース | 同じ | ○ 一致（注1） |
| `ASC` / `CHR` | コードポイント値 | 同じ | ○ 一致 |
| `ゼロ埋` / `空白埋` / `カタカナ半角変換` | コードポイント単位 | 同じ | ○ 一致（注2） |
| ZWJ絵文字 `"👨‍👩‍👦"` の `文字数` | 5 | 5 | ○ 一致 |

注1: これらは内部的にはUTF-16のまま部分文字列を突き合わせますが、
部分文字列の一致判定と分割は**コードユニットで数えてもコードポイントで数えても
結果が変わらない**ため、Goの `strings.Replace` / `strings.Count` と一致します。

注2: `ゼロ埋` / `空白埋` は桁数を `v.length`（UTF-16コードユニット数）で、
`カタカナ半角変換` は `s.split('')`（コードユニット分割）で数えていたため、
本来ここはrune基準と食い違う箇所でした。**本家TS版で修正され、fixtureも
再同期済み**です（サロゲートペアのケースが追加されています）。

### 一致しない2点

Goネイティブにすると差が出るのは、**次の2つ**です。
いずれも差分fixtureで `intentionalDiff` を付けて集計から外してあります。

| 箇所 | TS版 | Go版 | 備考 |
|---|---|---|---|
| 文字列の添字アクセス `A[0]` | サロゲートの片割れ | `𩸽` | 単独では文字として成立しない値 |
| 正規表現の `.` など | サロゲートを2文字扱い | 1文字扱い | JSは `u` フラグなしのため |

いずれも**サロゲートペア（BMP外の文字）を含む文字列に限った話**で、
日本語の常用範囲・絵文字の`文字数`では差が出ません。

> **「2つ」は現時点の調査結果であり、証明ではありません。**
> 上の注2のように、TS版の `plugin_system_string.mts` には
> `.length` / `split('')` / `substring` を使った命令が残っていることがあります。
> 新しいUTF-16依存が見つかったら、**まず本家側を修正して**サロゲートペアの
> ケースをfixtureに足し、Go版はrune基準のまま追随する、という順序で進めます。
> Go版に合わせて `intentionalDiff` を増やすのは最後の手段です。

> **文字コード変換（`ASC`/`CHR`）で差が出るのでは、という懸念について。**
> ここは逆に**既に一致しています。** TS版が `codePointAt` / `fromCodePoint` を
> 使っているため、`("𩸽"のASC)` は `171581`（コードポイント値）を返します。
> UTF-16のコードユニット値（`0xD867`）ではありません。

### 2点への対処

1. **添字アクセス**: サロゲートの片割れは単独で文字として成立しないので、
   Go版がrune基準にするほうが自然です。**意図的差異として受け入れます**
2. **正規表現**: Goの `regexp` はrune基準なので、こちらも意図的差異とします。
   JSの `u` フラグ付き正規表現と同じ挙動になります

> かつては `要素数` / `LEN` も差異の一つでした（`"𩸽あ"` に対して
> `文字数` が2、`要素数` が3を返していた）。本家TS版がコードポイント基準に
> 揃えたため（#2449）、この差はなくなっています。

### 実装上の注意

Goの `string` は**UTF-8バイト列**なので、`len(s)` はバイト数、
`s[i]` はバイトを返します。なでしこの命令で使ってよいのは次だけです。

```go
// internal/value/text
func RuneLen(s string) int              // utf8.RuneCountInString
func RuneSlice(s string, i, j int) string
func RuneAt(s string, i int) string
```

**`len()` と `s[i]` を直接使わない**ことを、コードレビューの観点に入れてください。
ここを間違えると、日本語を含むほぼ全ての文字列でバグります。

## 6. IR --- 最重要の設計境界

現行のASTは `meta` に実際のJS関数（`FuncListItem.fn`）を持つため直列化できません。
Go版では**バージョン付きの直列化可能IR**を新規に定義します。

```go
// internal/ir
type Program struct {
    Version   int          // IRのバージョン
    Consts    []Const      // 定数プール
    Funcs     []Func       // 関数（無名関数を含む）
    Main      int          // エントリのFuncインデックス
    Sources   []SourceFile // ソースファイル
    Positions []SourcePos  // 命令から参照するソース位置
}

type Func struct {
    Name    string
    Params  []Param    // 助詞を含む
    NumVars int
    Code    []Inst
    Async   bool       // 効果情報
    Pure    bool
}

type Inst struct {
    Op   Op
    A, B int
    Pos  int   // Positionsのインデックス
}
```

**VMは値スタック方式**にします。式の評価はオペランドスタックで行い、
ローカル変数は `Func.NumVars` 個のスロット配列（`A` に添字）で持ちます。
レジスタ方式にしないのは、12節のGoソース生成でスタックの上げ下げを
そのままGoの一時変数に落とせるからです。

`Inst` のオペランドは `A` / `B` の2つだけなので、**可変長のものは
「個数を `B` に持ち、実体はスタックに積む」**規約で表します。

```text
CallFunc   A=stdlibの関数ID   B=引数の個数   … 引数はスタックに左から積まれている
CallUser   A=Funcのインデックス B=引数の個数
MakeArray  A=未使用           B=要素数
MakeDict   A=未使用           B=キーと値の組数
LoadConst  A=Constsの添字
LoadVar / StoreVar  A=ローカルスロット番号
Jump / JumpIfFalse  A=飛び先のCode添字
```

守ること。

- **命令は名前ではなくIDで参照**する（`stdlib` の関数テーブルの添字）
- `Async` / `Pure` などの**効果情報をIRに持たせる**。VMもGoコード生成も両方使う
- **ソース位置を必ず持つ。** `Inst.Pos` から `SourcePos`、さらに `SourceFile` を
  参照してファイル名・行を復元する。エラー文面の行番号が互換対象なので落とせない。
  列は互換対象ではない（fixtureの `error` は `{type, line, message}` のみ）が、
  `SourcePos` には診断用に持たせてよい
- IRのバージョンを上げたら、古いバイトコードは読めないと明示的に拒否する

**IRからGoソースを生成できる粒度に保つ**ことを、設計レビューの観点に入れます（→ 12節）。

### VMの速さについて

`internal/vm/bench_test.go` に、性質の違う3つの計測があります。

```bash
go test ./internal/vm/ -run XXX -bench . -benchtime 200x
```

- `BenchmarkLoop` --- 命令のディスパッチと演算子。**実行中のアロケーションはゼロ**
- `BenchmarkRecursion` / `BenchmarkCalls` --- 呼び出し1回あたりの費用。
  フレーム・ローカル・セルの確保がここに出る

速くしようとする前に、必ずこれを取ってください。プロファイルを見ずに
直すと、たいてい「速くなったつもり」で終わります（実際、演算子を
`internal/ops` へ切り出したとき、中継メソッドを1段挟んだだけで測れるほど
遅くなりました）。

呼び出しは1回あたり3アロケーション（フレーム・ローカルのスライス・
セルのまとめ確保）まで減らしてあります。これ以上減らすにはフレームの
使い回しが要りますが、クロージャに捕まったセルを使い回すと壊れるので、
`OpMakeFunc` を持たない関数に限る、といった条件が要ります。

ループ側はディスパッチ(約40%)とスタック操作(約23%)で大半が決まります。
ここはスーパー命令を1つ入れて縮めました（`internal/compiler/peephole.go`。
`Load`+`Load`+`Binary` → `OpBinaryAt`）。IRのバージョンを2に上げ、検証器と
gogenにも同じ命令を足してあります。`BenchmarkLoop` が-17%、`BenchmarkCalls`
が-3%。

このとき、被演算子の取得を `operandAt` という小さなメソッドに切り出した版も
測りましたが、**その1段だけで改善分がほぼ消えました**（-17%が-2%になった）。
`internal/vm/run.go` の `OpBinaryAt` を展開したまま置いてあるのはそのためです。
`internal/ops` を切り出したときと同じ現象で、ディスパッチループの中では
中継を1段挟むだけで測れるほど効きます。

---

## 7. Host API

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

## 8. 非同期とイベントキュー

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

## 9. エラー

エラーの**種類・行番号・文面**が互換対象です。

```go
// internal/errs
type Kind int   // Lexer / Syntax / Runtime

type NakoError struct {
    Kind Kind
    File string
    Line int      // 0起点。文面では +1 して「N行目」と出す
    Msg  string
}

func (e *NakoError) Error() string   // "[実行時エラー]main.nako3(1行目): ..."
```

`compat run` の出力する `error.type` は**TS版のクラス名**なので、
`Kind` から次のとおり変換します。文面の接頭辞とは別物なので混同しないこと。

| `Kind` | `error.type` | `Error()` の接頭辞 |
|---|---|---|
| `Lexer` | `NakoLexerError` | `[字句解析エラー]` |
| `Syntax` | `NakoSyntaxError` | `[文法エラー]` |
| `Runtime` | `NakoRuntimeError` | `[実行時エラー]` |

`Msg` は改行を含みうる（`09_error` の「閉じ括弧なし」は2行の文面を返す）ので、
1行だと決め打ちしないこと。

差分fixtureの `09_error` グループが、この文面をそのまま固定しています。
文面の日本語は現行TS版からの**逐語移植**であり、Go版で言い回しを改善しないでください。

---

## 10. 単一ファイル梱包

なでしこ1で好評だった「作ったものをそのまま渡せる」を取り戻す部分です。

```bash
gonako build かんたんゲーム.nako3 --resource ./images --out かんたんゲーム
```

構成はシンプルに、**ランタイム本体の末尾にペイロードを追記**する方式にします。

```text
[ gonako ランタイム本体 (通常のGoバイナリ) ][ ペイロード ][ フッタ(マジック+長さ) ]
```

- ペイロードは `IR + リソース` をまとめたzip
- 起動時に自分自身（`os.Executable()`）の末尾を読み、フッタがあれば同梱モードで動く
- リソースは `internal/bundle` の仮想ファイルシステム越しに見せ、
  **なでしこ側からは通常のファイル読み込み命令と同じ書き方**でアクセスできるようにする
- 開発中（同梱なし）は実ファイルを読むので、書き換えなしで同じコードが動く

クロスコンパイルは `GOOS` / `GOARCH` を指定してランタイム本体を用意しておき、
それにペイロードを追記するだけなので、**ビルド機にGoツールチェインが要りません**。

### ペイロードの中身

ペイロードのzipには `manifest.json` を入れ、**アプリの種類**を持たせます。

| 種類 | 起動時の動作 | 中身 |
|---|---|---|
| `nako3` | 同梱したプログラムを実行する | `program.ir.json` + `resources/` |
| `html` | 同梱した開始ページをWebViewで開く | `resources/`（`entry` が開始ページ） |

マニフェストのない古いバンドルは `nako3` として読みます（後方互換）。

リソースの入れ方は2通りあります。`gonako build --resource ./images` は
**フォルダ名を残して** `resources/images/…` に入れるので、開発中に
`「images/a.png」を開く` と書いたコードがそのまま動きます。GUI版の
「フォルダを実行ファイルに変換」（→ 11節）は逆に**フォルダの中身を平らに**
入れるので、`「data/x.csv」を開く` がそのまま動きます。どちらも
「開発時に書いたパスがそのまま通る」という一点に揃えてあります。

### 署名まわり（実機で確認した結果）

**追記しても署名は壊れません。むしろ `codesign` し直すと壊れます。**

macOSのad-hoc署名が守る範囲は CodeDirectory の `codeLimit` までで、
その後ろに足したバイトは対象外です。Apple Silicon機で、追記しただけの
バイナリがそのまま起動することを確認しています。

一方、追記済みのファイルを `codesign --force --sign -` にかけると、
Mach-Oの末尾に余分なデータがある状態なので
`main executable failed strict validation` で拒否され、書き換えられた
実行ファイルは起動時に SIGKILL されます（exit 137）。

したがって**署名には触らない**のが正解です。Windowsも同様に、PEローダは
末尾の余分なデータを見ません。

---

## 11. GUI版（webview_go）

- 日本語入力（IME）が死活問題なので、OSネイティブのWebView（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を使う軽量な `webview_go` を採用しています。
- `internal/host` や `internal/vm` をCUI版と共有し、バイナリ埋め込み（`//go:embed`）のWebエディタUIからなでしこプログラムを実行できます。
- 開発エディタは、`cmd/gonako-gui/ui/*`にあるリソースから起動します。実体はgo build時点でコンパイル済みバイナリの中にコピーされ、実行時はembed.FS（メモリ上の仮想FS）から読み出されます。そのため、エディタを変更したら、再ビルドが必要です。

### エディタの色分け

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

### フォルダを実行ファイルに変換

ハンバーガーメニューの「このフォルダを実行ファイルに変換」で、開いている
ファイルをメインにして、そのフォルダ以下を1つのアプリに梱包します（→ 10節）。
拡張子で種類が決まり、`.nako3` なら起動と同時にそのプログラムを実行、
`.html` ならそのページをWebViewで開くアプリになります。

ランタイムには **gonako-gui 自身のコピー**（`os.Executable()`）を使うので、
**作る側にも受け取る側にもGoツールチェインが要りません**。

OSごとの後始末が要ります。

| OS | 出力 | 追加でやること |
|---|---|---|
| macOS | `app.app` | `ui/macapp/` のひな形から `.app` を組み立てる。`Info.plist` に名前を埋め、アイコンを入れる。署名には触らない（→ 10節） |
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

### Go言語でビルド（gogen連携）

ハンバーガーメニューの「Go言語でビルド」で、開いている `.nako3` を
`internal/gogen` でGoソースに変換し、そのまま `go build` してネイティブ
実行ファイルにします（→ 12節・`docs/gogen.md`）。「フォルダを実行ファイルに
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

---

## 12. Goコード生成バックエンド

速度が必要な場面のための追加バックエンドです（#2448 参照）。**実装済み**です。

```text
IR ──► internal/gogen ──► Goソース ──► go build ──► ネイティブ実行ファイル
```

JS生成と違い、**標準命令の実装を二重に持つ必要がありません。**
リポジトリ外へ生成したGoコードからはGoの可視性規則により `internal/stdlib` を
直接 `import` できないため、最小限の公開ファサードを `pkg/runtime` に置き、
生成コードはそこを経由して同じ `internal/stdlib`（正確には `internal/vm`）を呼びます。
標準命令の実装は一つのままです。

### 仕組み

`internal/gogen` が生成するのは、実は**制御フローだけ**です。値の演算・添字
アクセス・命令呼び出し・クロージャ生成といった意味論はすべて `internal/vm`
（インタプリタ）が既に持っているコードへ委譲します。`internal/vm/export.go`
がその薄いエクスポート層で、`ConstValue` / `Binary` / `IndexGet` /
`CallStd` / `MakeClosure` などをインタプリタの内部関数そのままの薄いラッパ
として公開します。`pkg/runtime` はこれを型エイリアスと薄い再エクスポート
だけで包み、外部モジュールから見える唯一の依存先になります。

`ir.Func` 1つにつき生成されるGo関数は、バイトコードを `goto` ラベル付きの
直線コードへ機械的に展開したものです（`internal/vm/run.go` の `execute()`
と1対1対応）。オペランドスタックは通常の `[]rt.Value` に、ローカル・
キャプチャは `*value.Cell` の添字配列のまま。したがって**なでしこの意味論を
再実装しているわけではなく**、`internal/vm` と `internal/gogen` が挙動で
食い違うリスクは（新しい命令を追加したとき以外は）構造的にありません。

エラー監視（`OpTry`/`OpEndTry`）を使う関数だけは特別扱いします。Goの
`recover()` はパニックした呼び出しへ戻ることができない（新しい呼び出しで
やり直すしかない）ため、`internal/vm/run.go` の `run()`/`protect()` と同じ
「再試行ループ + 使い捨ての実行フレーム」構成をそのまま生成します
（詳細は `internal/gogen` のパッケージコメント）。

### 最適化

演算子の意味は `internal/ops` の1箇所にあります。インタプリタも、
コンパイラの定数畳み込みも、生成コードも、そこを通ります。「畳み込んだ
結果」と「実行した結果」が食い違わないことを、実装を共有することで
保証するためです（gogenと同じ考え方）。その上で:

| 最適化 | 効く範囲 |
|---|---|
| 定数畳み込み（`internal/compiler/fold.go`） | VM・gogen |
| 定数伝播（同上。『定数』宣言の値を後の式へ差し込む） | VM・gogen |
| 覗き穴最適化とスーパー命令（`internal/compiler/peephole.go`） | VM・gogen |
| 数値どうしの高速経路（`internal/ops`。両辺が数値なら ToPrimitive/ParseFloat を通さない） | VM・gogen |
| 定数を `rt.Number(9)` としてソースへ直接埋め込む | gogen |
| 演算子・変数代入をMachineのメソッド経由にせず、Goがインライン展開できる形で書く | gogen |

**速度のためにJavaScriptの意味論を曲げないこと。** 数値は float64 一本
（→ 4節）で、「この変数は整数だ」と決めつける最適化はしません。上の
高速経路も、実行時に両辺が数値だったときに遠回りを省くだけです。
一般経路と1ビットも違わないこと（NaN・±0を含む）は
`internal/ops/ops_test.go` が総当たりで確かめています。

残っている一番大きなコストは**オペランドスタック**（`push`/`pop`）です。
式を直接Goの式へ組み立てれば更に速くなりますが、「バイトコードの機械的な
変換」という現在の作りを変えることになります（→ `docs/gogen.md` §7）。

### 対応範囲

クロージャ・キャプチャ・入れ子のエラー監視・動的呼び出し（`OpCallValue`）
を含め、インタプリタが実行できるものはほぼそのまま生成できます。唯一
非対応なのは**非同期関数**（`Func.Async`）です。Goのコールスタックを
goroutine外で中断・再開する自然な対応がないため、`Generate` は非同期関数を
検出すると理由付きでエラーを返します（AGENTS.md本節が言う「動的な機能は
制限してよい」の範囲内。必要ならその関数だけVM実行に戻します）。

### レジストリの一致が絶対条件

標準命令のIDは、レジストリに登録されたプラグインの**完全な組み合わせ**
（名前順にソートしたもの）で決まります。生成コードのレジストリが、
`prog` をコンパイルしたときのレジストリと1件でもズレると、コンパイル
エラーにも実行時エラーにもならず、**別の命令が黙って呼ばれます**。
これを構造的に避けるため、`gogen.Options.Plugins`（コンパイル側は
`gogen.BuildRegistry`）で名前のリストを一箇所にまとめ、`gonako gengo`
はコンパイルと生成の両方に同じリストを渡します。

- Goツールチェインが要るのは**作る側だけ**。受け取る側は実行ファイルを動かすだけ
- **差分fixtureを「TS版 / Go VM / Go生成コード」の3系統で回し、結果の一致を保証する**。
  `internal/gogen/compat_test.go` の `TestCompatBatch`（`GOGEN_COMPAT=1`
  で有効化）が「Go VM / Go生成コード」の側を担う。非同期を除く230/230件が一致
- `gonako gengo <file.nako3> [--out out.go] [--plugins ...] [--package NAME]`
  でGoソースを書き出す。ビルド方法（`go.mod` に `replace` を書く手順）は
  実行時のメッセージに出る

---

## 13. 実装手順

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

## 14. 本家リポジトリ（TypeScript版）側でやること

- DocTestや差分fixtureの調査中に、**本家TypeScript版の命令実装ミス・仕様との不一致・
  回帰不具合を見つけた場合は、Go版だけで回避せず、再現コードと期待結果・実際結果を添えて
  [なでしこ3のIssues](https://github.com/kujirahand/nadesiko3/issues) に投稿する**
- 差分fixtureを**育てる**。互換で困ったケースが見つかるたびに `cases/` へ足す
- **UTF-16依存が見つかったらTS版側を直す**（`ゼロ埋` / `空白埋` /
  `カタカナ半角変換` は修正済み）。直したらサロゲートペアのケースを `cases/` に足し、
  Go側は `scripts/sync-compat-fixtures.sh` で**再同期して `SOURCE` を更新する**。
  同期を忘れると、Go版が正しいのに不一致として数えられます
- 期待値を変える変更（挙動の変更）は、**変えた理由をコミットメッセージに残す**。
  Go側は理由が分からないと追随できません
- `plugin_system` に命令を足したら、対応するケースも足す

差分fixtureは、**Go版の計画が途中で止まってもTS版の回帰テストとして残る資産**です。

---

## 15. 未決事項

決め打ちせず、実装しながら判断する項目です。

- **多倍長整数**: 現行TS版は一部でBigIntを扱う。Go版で `math/big` に載せるか、
  そもそも対象外にするか。当面は対象外とし `KindBigInt` を作らない（→ 4節）
- **正規表現エンジン**: 標準 `regexp`(RE2) で始める。後方参照・先読みが必要になったら
  `dlclark/regexp2` を後付けし、失敗したものだけ流す二段構えにする（#2448）
- **`plugin_node` の命令名**: TS版に寄せる範囲をどこまでにするか
- **オフィス処理などのライブラリ選定**: ライセンス（MIT/BSD系を優先）と日本語の扱いで選ぶ
