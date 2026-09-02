# gonako VM 設計・実装計画

## 1. 目的と範囲

この文書は、`internal/ir` の直列化可能IRを `internal/value.Value` で実行する
スタックVMの設計を定める。

VMの責務は次のとおり。

- IRの検証と実行
- オペランドスタック、呼び出しフレーム、ローカル変数、グローバル変数の管理
- ユーザー関数、無名関数、クロージャの呼び出し
- 標準命令を安定したIDで呼ぶためのABI
- IRのソース位置を使った実行時エラーの生成
- 単一スレッドのイベントキューによる決定的な非同期実行

次はVMの責務に含めない。

- ソースコード、トークン、ASTの解釈
- ASTからIRへの変換
- `plugin_system` 各命令の具体的な意味
- ファイル、OS、ネットワークなどの実処理
- compat結果JSONへの正規化

パーサーとコンパイラが未完成でもVMを実装・検証できるよう、VMの単体テストでは
手書きの `ir.Program` を使う。コンパイラ担当者は、この文書の命令契約を出力仕様として
利用する。

## 2. 現状と着手前に確定すること

現在のIRは `Nop`、`Const`、`Call`、`Return` の4命令だけで、値モデルの
`value.Func` も関数IDだけを持つ骨格である。このままでは分岐、変数、配列、辞書、
ユーザー関数、クロージャ、例外、非同期を表せない。

最初に次のIR拡張を行い、VMとコンパイラの境界を固定する。

```go
type Program struct {
    Version       int
    StdlibVersion int
    Consts        []Const
    Globals       []Global
    Funcs         []Func
    Main          int
    Sources       []SourceFile
    Positions     []SourcePos
}

type Global struct {
    Name  string
    Const bool
}

type Func struct {
    Name       string
    Params     []Param
    NumVars    int
    ConstVars  []int
    Captures   []Capture
    MaxStack   int
    Code       []Inst
    Async      bool
    Pure       bool
}

type Capture struct {
    From  CaptureFrom // local または外側のcapture
    Index int
}
```

- `Globals` は名前からスロット番号への対応を直列化する。compatの `vars` 取得にも使う。
- `StdlibVersion` は標準命令IDのABIバージョンである。IRバージョンとは別に検証する。
- `MaxStack` は検証時に再計算し、一致を確認する。実行時の領域確保にも使う。
- `Capture` はGoポインタをIRへ入れず、外側のどのセルを捕捉するかだけを記録する。
- `ConstVars` は関数内の定数スロットを示す。グローバル定数は `Global.Const` で示す。
- IR v1を外部配布する前なので、上記変更は現行v1へまとめてよい。配布開始後に形式を
  変える場合は `CurrentVersion` を上げる。

## 3. 実行時データ構造

### 3.1 VM本体

```go
type VM struct {
    program  *ir.Program
    registry *stdlib.Registry
    host     host.Host
    loop     *event.Loop

    globals  []*value.Cell
    specials [ir.SpecialCount]value.Value

    callbacks map[host.CallbackID]queuedCallback
    // ...
}
```

`VM` は1実行につき1個作る。実行中のVMを複数のgoroutineから操作しない。

### 3.2 セルとクロージャ

fixtureの「関数-クロージャ」では、外側の `M` を無名関数が更新し、次回呼び出しにも
更新後の値が残る。そのため値のコピーではなく共有セルを捕捉する。

```go
type Cell struct {
    Value       Value
    Mutable     bool
    Initialized bool
}

type Func struct {
    ID       int
    Captures []*Cell
}
```

`Cell` と `Func` は `internal/value` に置く。Goポインタを使ってよいのはGoプロセス内の
VM境界までであり、IR、bundle、Host APIには出さない。

実装を単純にするため、最初は全ローカル変数を `[]*value.Cell` で持つ。
捕捉されないローカルだけ値配列にする最適化は、互換fixtureが通った後に検討する。

### 3.3 呼び出しフレームと特殊変数

```go
type Frame struct {
    Func     *ir.Func
    Locals   []*value.Cell
    Captures []*value.Cell
    Stack    []value.Value
    Specials [ir.SpecialCount]value.Value // それ、対象、対象キー、回数、エラーメッセージ等
    Handlers []Handler
}
```

- 引数は `Locals[0:len(Params)]` に左から格納する。
- 未初期化のスロットは必ず `undefined` とし、Goのゼロ値を言語値として扱わない。
- 定数セルは `InitLocal` / `InitGlobal` で一度だけ初期化でき、その後の書き込みを拒否する。
- 関数の明示的な戻り値がなければ、そのフレームの `それ` を返す。
- 呼び出し結果は呼び出し元の `それ` にも設定する。
- **特殊変数はフレーム（ローカル変数）単位で保持**:
  - `それ`、`対象`、`対象キー`、`回数`、`エラーメッセージ` は、TypeScript版の `__setSysVar`（グローバル共有）の挙動とは異なり、呼び出しフレームごとに独立したローカル変数として管理される。
  - 子フレームは呼び出し元の特殊変数を値コピーして開始する。ただし `それ` は空文字列で初期化し、タイマーコールバックの `対象` は実行中のタイマーIDで上書きする。
  - これにより、ループ処理内から呼び出された別関数が `対象` や `回数` を変更しても、呼び出し元の特殊変数は保護される。入れ子ループでの一時的な保存・復元は、コンパイラがローカルスロットへ退避・復元するIRを生成して行う。

## 4. オペランドスタックの規約

- 式は必ず値を1個スタックへ積む。
- 文として結果を使わない式は、コンパイラが `Pop` を出す。
- 二項演算は左辺、右辺の順に積み、右辺、左辺の順に取り出す。
- 関数引数はソース上の左から右へ評価して積む。
- 呼び出し命令は `B` 個を取り出して、元の順番の引数スライスへ戻す。
- 関数呼び出しは戻り値がない命令でも `undefined` を1個積む。
- `JumpIfFalse` は条件値を1個取り出す。
- 命令実行後にスタック効果が契約と違った場合は、ユーザーエラーではなく
  `InvalidIR` として停止する。

共有スタックを使い、フレーム開始時の長さを `StackBase` に保存する。関数が戻るときは
必ず `stack[:StackBase]` へ戻してから戻り値を1個積む。これにより、関数内の一時値が
呼び出し元へ漏れない。

## 5. IR命令集合

`Inst` は `Op, A, B, C, Pos` の固定長とする。可変長データは定数プール、
関数表、またはスタックで表す。`C` は覗き穴最適化が作るスーパー命令
（→ 5.3 `BinaryAt`）だけが使い、それ以外の命令では0である。

### 5.1 基本・スタック

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `Nop` | - | `0` | 何もしない |
| `LoadConst` | `A=Consts index` | `+1` | 定数を積む |
| `Pop` | - | `-1` | 未使用値を捨てる |
| `Dup` | - | `+1` | 先頭値を複製する |

`Const` は名前が他の型名と紛らわしいので `LoadConst` に改名する。

### 5.2 変数と特殊変数

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `LoadLocal` | `A=slot` | `+1` | ローカルセルを読む |
| `InitLocal` | `A=slot` | `-1` | ローカル定数セルを一度だけ初期化する |
| `StoreLocal` | `A=slot` | `-1` | ローカルセルへ書く |
| `LoadCapture` | `A=slot` | `+1` | 捕捉セルを読む |
| `StoreCapture` | `A=slot` | `-1` | 捕捉セルへ書く |
| `LoadGlobal` | `A=slot` | `+1` | グローバルセルを読む |
| `InitGlobal` | `A=slot` | `-1` | グローバル定数セルを一度だけ初期化する |
| `StoreGlobal` | `A=slot` | `-1` | グローバルセルへ書く |
| `LoadSpecial` | `A=SpecialID` | `+1` | `それ` などを読む |
| `StoreSpecial` | `A=SpecialID` | `-1` | `それ` などへ書く |

定数への再代入はコンパイラで拒否する。IR検証は定数セルへの通常の `Store` と、変数セルへの
`Init` を拒否する。実行時にも二重初期化を検査し、壊れたIRを安全に停止する。定数セルを
クロージャが捕捉した場合も、セルの `Mutable` がfalseなので `StoreCapture` は失敗する。

### 5.3 演算

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `Unary` | `A=UnaryOp` | `0` | 否定、符号反転など |
| `Binary` | `A=BinaryOp` | `-1` | 二項演算 |
| `BinaryAt` | `A=BinaryOp+取得元`, `B`,`C`=添字 | `+1` | 両辺をスタックを経ずに読んで二項演算 |

`BinaryAt` はコンパイラの覗き穴最適化（`internal/compiler/peephole.go`）が
`Load;Load;Binary` の3命令をまとめたもので、意味は `Binary` と同じである。
`A` に演算子と両辺の取得元（定数・ローカル・捕捉・グローバル）を詰め、
`B` と `C` がその添字になる。詰め方は `ir.EncodeBinaryAt` / `ir.DecodeBinaryAt`
の2つだけが知っていればよい。IR検証は取得元と添字を、対応する `Load` 命令と
同じ基準で範囲検査する。

演算子ごとの命令を増やさず、`UnaryOp` / `BinaryOp` の列挙値を使う。
暗黙の型変換、等価比較、真偽判定は `internal/value` の純粋関数へまとめ、VM、標準命令、
将来のGoコード生成が同じ実装を使う。VM内へ変換規則を散らさない。

`BinaryOp` は少なくとも加算、減算、乗算、除算、整数除算、剰余、累乗、文字列連結、
等価、不等価、大小比較、AND、OR、ビット演算、シフトを持つ。

### 5.4 配列・辞書・添字

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `MakeArray` | `B=要素数` | `1-B` | 左から積まれた要素で配列を作る |
| `MakeDict` | `B=組数` | `1-2B` | key, valueの組から挿入順辞書を作る |
| `GetIndex` | - | `-1` | container, indexから値を読む |
| `SetIndex` | - | `-2` | container, index, valueを書き、valueを積む |

`SetIndex` の正味効果は `-2` である。代入式を文として使う場合は後続の `Pop` が値を捨てる。

- 配列の範囲外読み出しは `undefined`。
- 配列を範囲外へ書くと中間を `undefined` で埋める。
- 辞書キーは共通の文字列変換関数で文字列化する。
- 文字列添字はGo版の決定どおりrune単位とする。
- 多次元添字は `GetIndex` / `SetIndex` の連鎖へコンパイルする。

### 5.5 制御フロー

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `Jump` | `A=code index` | `0` | 無条件分岐 |
| `JumpIfFalse` | `A=code index` | `-1` | 偽なら分岐 |
| `JumpIfTrue` | `A=code index` | `-1` | 真なら分岐 |

`もし`、`回`、`間`、`反復`、`条件分岐`、`抜ける`、`続ける` は専用VM命令を作らず、
ローカル変数、比較、分岐へ落とす。これにより将来のGoコード生成も同じIRを扱える。

### 5.6 関数とクロージャ

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `MakeClosure` | `A=Funcs index` | `+1` | 現フレームから捕捉セルを集め関数値を作る |
| `CallStd` | `A=command ID, B=argc` | `1-B` | 標準命令を呼ぶ |
| `CallUser` | `A=Funcs index, B=argc` | `1-B` | 既知のユーザー関数を直接呼ぶ |
| `CallValue` | `B=argc` | `-B` | calleeと引数を取り出して関数値を呼ぶ |
| `Return` | - | `-1` | 値を返す |
| `ReturnSore` | - | `0` | 現フレームの `それ` を返す |

`CallValue` の入力は `callee, arg0, ... argN`、出力は戻り値1個なので、正味効果は
`-B` となる。直接呼び出しと関数値呼び出しを分けることで、通常の命令呼び出しを軽くし、
無名関数と高階関数も扱える。

命令実行ループは命令を取得した直後に `IP` を次へ進め、それからcallを実行する。
stdlibの `Context.Invoke` が別のユーザー関数を同期的に呼んでも、呼び出し元のcall命令を
再実行しないためである。`Context.Invoke` は対象フレームを積み、そのフレームが戻るまで
VMを進めて戻り値を受け取る。

`Captures` が空でない関数を `CallUser` で直接呼ぶIRは不正とする。その関数は必ず
`MakeClosure` と `CallValue` を経由し、定義時の環境を伴って呼ぶ。

再帰は `CallUser` で同じ `FuncIndex` を呼べばよい。初期実装では再帰深度上限を
`Options.MaxFrames` で設定し、超過時はソース位置付き実行時エラーにする。

### 5.7 例外

| 命令 | オペランド | スタック効果 | 意味 |
|---|---:|---:|---|
| `Try` | `A=catch先` | `0` | 現在のスタック深さとcatch先を保存 |
| `EndTry` | - | `0` | 直近のハンドラを外す |
| `Throw` | - | `-1` | 値を実行時エラーとして送出 |

標準命令が返したエラーも `Throw` と同じ経路で処理する。エラー発生時は呼び出しフレームを
順に巻き戻し、最も近い `Try` へ移動する。その際、保存したスタック深さへ戻し、
`エラーメッセージ` を設定する。ハンドラがなければ `Run` の戻り値として返す。

Goのpanicは言語の例外に使わない。VM公開境界では防御的にrecoverし、発生した場合は
内部不具合を示す別エラーとして扱う。

## 6. 標準命令ABI

標準命令をGoのmap反復順で採番してはいけない。明示的なIDを持つ配列を正本とする。

```go
type Entry struct {
    ID         int
    Name       string
    Args       [][]string
    Async      bool
    Pure       bool
    ReturnNone bool
    Handler    Handler
}

type Handler func(Context, []value.Value) (value.Value, error)
```

- IDは一度割り当てたら変更・再利用しない。削除命令のIDは欠番として残す。
- 新しい命令は末尾へ追加する。
- 名前検索mapは明示ID配列から生成する。
- `Program.StdlibVersion` とregistryのバージョンが違えば実行前に拒否する。
- 引数数はVMが検査し、可変長命令だけregistryの指定で許可する。
- `ReturnNone` の命令もVM上は `undefined` を返す。

`Context` は標準命令へスタックやフレームを公開せず、次の必要最小限だけを提供する。

- `Host()`
- `Now()`
- `Invoke(function, args)`
- `Schedule(function, at, interval)`
- `CancelTimer(id)`
- 必要になった時点で、限定されたシステム変数アクセス

標準命令は `Context` 経由でコールバックを登録し、goroutineを直接起動しない。

## 7. エラーとソース位置

VMは実行中の各命令について `Inst.Pos` を `Program.Positions`、`Program.Sources` の順に
解決する。標準命令のエラーにも呼び出し元の `CallStd` の位置を付ける。

エラーは次の2系統に分ける。

1. `errs.NakoError{Kind: Runtime}`: 利用者のプログラムによるエラー
2. `InvalidIR` / `InternalVMError`: 壊れたIRまたはVM実装不具合

`NakoError.Line` は0起点のまま保持し、`Error()` でのみ1起点表示にする。TS版と同じ文面が
必要な箇所は、VMで言い換えずfixtureの期待値を逐語的に実装する。

## 8. 非同期とイベントループ

### 8.1 原則

- 観測可能な実行順にgoroutineを使わない。
- イベントは実行予定時刻、同時刻内の登録順で並べる。
- 時刻は実時間を待たず、次のイベント時刻へ進める。
- 同一VMのコールバックは同じグローバルセルを共有する。

### 8.2 `event` と `host.Timer` の境界

`event.Loop` は `host.Timer` の `Post` / `Cancel` を実装する。一方、VMにはキューを進める
ための追加インターフェースを渡す。

```go
type Driver interface {
    host.Timer
    Now() time.Time
    RunNext(dispatch func(host.CallbackID) error) (bool, error)
    RunUntil(until time.Time, dispatch func(host.CallbackID) error) error
    RunUntilIdle(dispatch func(host.CallbackID) error) error
}
```

具体Hostの `Now()` と `Timer()` は、VMへ渡した同じ `event.Loop` へ委譲する。これにより、
標準命令が見る時刻とVMが進める時刻がずれない。

### 8.3 タイマー動作

- `秒後`: 関数値をcallback表へ登録し、指定時刻に1回実行する。
- `秒毎`: callback実行後、キャンセルされていなければ「前回の予定時刻 + 間隔」へ再登録する。
- `タイマー停止`: queueとcallback表の両方を無効化する。
- `秒待`: 現在時刻から指定時間後まで `RunUntil` し、その間のcallbackを順に実行してから
  停止していたフレームを再開する。
- main関数終了後は `RunUntilIdle` し、残っている単発callbackを実行する。
- 0以下の間隔、無限に残る繰り返し、callback総数にはOptionsで上限を設ける。

callback呼び出し時は `Special.Target` にタイマーIDを設定する。これによりcallback内の
`対象のタイマー停止` が同じタイマーを停止できる。

## 9. 実行API

```go
type Options struct {
    MaxFrames       int
    MaxInstructions uint64
    MaxCallbacks    uint64
}

type Result struct {
    Value   value.Value
    Globals []value.Value
}

func New(program *ir.Program, registry *stdlib.Registry,
    host host.Host, loop event.Driver, options Options) (*VM, error)

func (m *VM) Run() (Result, error)
```

- `New` はIR、stdlib ABI、HostとLoopの整合性を検証する。
- `Run` はmain関数を呼び、その後イベントキューを空になるまで処理する。
- VMインスタンスは1回の `Run` にだけ使う。再実行は新しいVMを作る。
- `Result.Globals` の並びは `Program.Globals` と同じにし、compat層が名前で取り出す。
- 命令数上限は壊れたIRや無限ループでcompat実行全体が止まるのを防ぐ。

## 10. IR検証

`Program.Validate` を構造検証と制御フロー検証に分ける。

### 構造検証

- IRとstdlibのバージョン
- Main、Const、Func、Global、Position、Sourceの各index
- ローカル、capture、特殊変数IDの範囲
- jump先が同じ関数の命令範囲内
- 引数数、`NumVars >= len(Params)`
- `ConstVars` が重複せず `NumVars` の範囲内であること
- capture元とindexの妥当性
- captureを必要とする関数をCallUserで直接呼んでいないこと
- const local/globalへのStoreと、変数local/globalへのInitを禁止
- 各関数に到達可能なReturnがあること

### 制御フロー検証

worklistで命令ごとの入力スタック深さを計算する。

- スタックunderflowがない
- 分岐合流点のスタック深さが一致する
- `Return` 時の必要値がある
- `Try` / `EndTry` が正しく対応する
- 計算した最大深さが `Func.MaxStack` と一致する

VM実行ループ内でも境界チェックは残す。Validate済みという前提だけでsliceを直接indexしない。

## 11. テスト方針

### 11.1 VM単体テスト

コンパイラを使わず、手書きIRで次を1件ずつ固定する。

1. 定数、Pop、Return、スタック残量
2. unary/binary演算と型変換
3. local/global/captureの読み書き
4. if、while相当のjump
5. ユーザー関数、再帰、引数、暗黙の `それ` 戻り値
6. mutable captureを持つカウンタ
7. 配列の範囲外代入と辞書の挿入順
8. 標準命令ID、引数順、戻り値なし命令
9. runtime errorのファイル名、行、文面
10. try/catchとフレームをまたぐ巻き戻し
11. 同時刻callbackの登録順、短い待ち時間優先、タイマー停止
12. 壊れたindex、jump、stack効果、ABIバージョンの拒否

テストHostは出力文字列を収集し、固定時刻とメモリ内Envを返す。時間を実際に待つテストは
作らない。

### 11.2 統合テスト

コンパイラが利用可能になったら、次の順でcompatを接続する。

- `01_literal`
- `02_operator`
- `03_type_convert`
- `07_flow`
- `08_func`
- `05_array` / `06_dict`
- `09_error`
- `10_async`

文字列、日時、JSON、正規表現などの通過率は主にstdlib実装の進捗であり、VMの命令追加で
個別対応しない。

## 12. 実装順序

### VM-0: IRとABIの固定

- IR命令、Globals、Capture、MaxStack、StdlibVersionを定義
- `value.Cell` とcapture付き `value.Func` を定義
- 明示IDのstdlib registry骨格を作る
- Validateの構造検証とstack効果表を作る

完了条件: 不正IRの表駆動テストが通り、手書きIRをJSON往復できる。

### VM-1: 最小実行ループ

- VM、Frame、共有stack、local/globalを実装
- LoadConst、Pop、変数、Unary、Binary、Returnを実装
- ソース位置付きエラーと命令数上限を実装

完了条件: 手書きIRでリテラル、代入、四則演算、比較が動く。

### VM-2: 分岐と標準命令呼び出し

- Jump系を実装
- CallStdとstdlib Contextを実装
- `表示` の最小実装を接続

完了条件: 手書きIRで `01`〜`03` と `07` の代表例を再現できる。

### VM-3: ユーザー関数とクロージャ

- CallUser、CallValue、MakeClosureを実装
- セル共有、再帰、戻り値、`それ` を実装
- frame上限を実装

完了条件: `08_func` の基本、再帰、クロージャ相当の手書きIRテストが通る。

### VM-4: コレクションと例外

- MakeArray、MakeDict、GetIndex、SetIndexを実装
- Try、EndTry、Throw、フレーム巻き戻しを実装

完了条件: 疎な配列、辞書順序、エラー監視の手書きIRテストが通る。

### VM-5: 決定的イベントループ

- event.Driverと仮想時計を実装
- callback表、秒後、秒毎、秒待、停止を接続
- callback数上限と繰り返し安全策を実装

完了条件: `10_async` と同じ順序を、実時間待機なしのテストで再現できる。

### VM-6: コンパイラ・compat接続

- コンパイラ出力をValidateしてVMへ渡す
- VMのResultをcompatのlog/vars/errorへ変換する
- 各グループの通過率を記録する

完了条件: 段階2の対象である `01`〜`03`、`07` が通り、既に通ったケースが後退しない。

## 13. レビュー観点

- VMがASTやトークン型へ依存していないか
- IRにGo関数、Goポインタ、mapを直列化しようとしていないか
- 命令IDと標準命令IDがmap順に依存していないか
- 全ての命令にstack効果とindex検証があるか
- 関数return時にStackBaseへ必ず戻しているか
- 未初期化値をGoゼロ値ではなく `undefined` にしているか
- closureが値コピーではなくセルを共有しているか
- エラー位置が現在命令の `Inst.Pos` から復元されているか
- 非同期の観測順がgoroutine schedulingへ依存していないか
- 文字列添字や文字列演算でbyte indexを使っていないか
- VM専用の処理を増やす前に、同じIRをGoコード生成できるか確認したか

## 14. 最初の作業単位

最初のPRはVM-0だけに限定する。

1. IR拡張と命令列挙
2. 命令ごとのstack効果関数
3. 構造・制御フローValidate
4. `value.Cell` とcapture付き関数値
5. stdlibの明示ID registry骨格
6. JSON往復、不正index、stack underflow、合流不一致、capture記述のテスト

このPRではVM実行ループや標準命令の具体実装を入れない。先にコンパイラ担当者と共有できる
IR契約をテストで固定し、その次のPRから手書きIRを使ってVM本体を実装する。
