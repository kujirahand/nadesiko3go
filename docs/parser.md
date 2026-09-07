# AST・IR・パーサーの設計

構文解析からIR（直列化可能なバイトコード）までの設計をまとめたドキュメントです。
設計の基準はリポジトリ直下の `AGENTS.md` で、本ドキュメントはそこからIRに
関する節を独立させたものです。

パイプライン全体は次のパッケージで構成されます（→ AGENTS.md 3節）。

```text
prepare（前処理） → lexer（字句解析） → indent/dncl（構文変換） → parser（構文解析、ASTを作る） → compiler（AST→IR） → vm（IR実行）
```

---

## 1. IR --- 最重要の設計境界

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
レジスタ方式にしないのは、Goソース生成（→ `docs/gogen.md`）でスタックの
上げ下げをそのままGoの一時変数に落とせるからです。

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
  参照してファイル名・行を復元する。エラー文面の行番号が互換対象なので落とせない
  （→ `docs/compat.md`）。列は互換対象ではない（fixtureの `error` は
  `{type, line, message}` のみ）が、`SourcePos` には診断用に持たせてよい
- IRのバージョンを上げたら、古いバイトコードは読めないと明示的に拒否する

**IRからGoソースを生成できる粒度に保つ**ことを、設計レビューの観点に入れます
（→ `docs/gogen.md`）。

---

## 2. VMの速さについて

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
