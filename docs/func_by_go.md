# Go言語によるなでしこ命令の追加手順

gonako（なでしこ3 Go版）でGo言語を使って命令（関数）を追加する方法のまとめです。

---

## 1. 概要と違い

### 方式は同じですか？
**基本的な実装方法（関数の型シグネチャ、助詞の定義方法、値の扱い方）はまったく同じです。**

ただし、**「システム命令」**と**「拡張プラグイン命令（SQLite3など）」**では、**登録場所と責務のスコープ**が分かれています。

| 区分 | システム命令 (`stdlib`) | プラグイン命令 (`sqlitelib` など) |
|---|---|---|
| **対象** | なでしこ3の `plugin_system` 相当（文字列・配列・日時など） | 外部機能・OS機能（SQLite3, CSV, ファイル, OS等） |
| **互換性保証** | 公式TypeScript版との**完全互換が保証される** | 保証対象外（Goらしく設計・拡張してよい） |
| **定義場所** | `internal/stdlib/registry.go` + `internal/stdlib/*.go` | 独立したパッケージ（`internal/sqlitelib/` など） |
| **登録方法** | `stdlib` コアのレジストリに直接組み込む | `stdlib.Plugin` インターフェースを実装して注入 |

---

## 2. 共通の基本知識

### (1) 実装関数の型 (`stdlib.Impl`)
すべての命令の実装は以下のシグネチャを持ちます。

```go
type Impl func(ctx stdlib.Context, args []value.Value) (value.Value, error)
```

- `args`: 引数が左から順番に渡されます（0番目が第一引数）。
- 戻り値: `value.Value` と `error`。エラーを返すと実行時エラー（`NakoRuntimeError`）になります。
- `ctx`: 出力（`ctx.Print()`）、タイマー操作、システム変数の読み書きなどにアクセスできます。

### (2) 引数と戻り値の変換 (`value.Value`)
```go
// 引数の取り出し
n := value.ToNumber(args[0])  // float64
s := value.ToString(args[1])  // string (UTF-8)
arr, ok := args[0].Array()    // *value.Array
dict, ok := args[0].Dict()    // *value.Dict

// 戻り値の作成
return value.Number(123), nil
return value.String("結果"), nil
return value.Bool(true), nil
return value.Undefined(), nil // 戻り値なしの場合
```

### (3) メタデータ・助詞の定義 (`lexer.FuncItem`)
```go
&lexer.FuncItem{
    Name:       "命令名",
    Type:       "func",
    Josi:       [][]string{{"を", "から"}, {"に", "へ"}}, // 第1引数の助詞, 第2引数の助詞
    Pure:       true,   // 副作用がない場合true
    ReturnNone: false,  // 戻り値がない（表示系など）場合true
}
```

---

## 3. 手順A: システム命令を追加する場合

`plugin_system` 相当の標準命令（文字列、配列、日時、計算など）を追加する手順です。

### ステップ1: `internal/stdlib/registry.go` にシグネチャを登録
構文解析（パーサー）で認識できるように `ParserFuncList()` に追加します。

```go
// internal/stdlib/registry.go
addFunc("命令名", [][]string{{"を"}, {"で"}})
```

### ステップ2: `internal/stdlib/*.go` に実装を追加
対応するカテゴリのファイル（例: `string.go`, `math.go`, `datetime.go` 等）に処理を記述します。

```go
// 例: internal/stdlib/string.go
m["命令名"] = func(ctx Context, args []value.Value) (value.Value, error) {
    if len(args) < 2 {
        return value.Undefined(), nil
    }
    src := value.ToString(args[0])
    opt := value.ToString(args[1])
    
    result := doSomething(src, opt)
    return value.String(result), nil
}
```

---

## 4. 手順B: プラグイン命令を追加する場合 (SQLite3など)

SQLite3やCSV、独自の外部ライブラリなど、独立した拡張機能を追加する手順です。

### ステップ1: プラグイン用のパッケージを作成
`internal/<プラグイン名>/` ディレクトリを作成し、`stdlib.Plugin` インターフェースを満たす構造体を定義します。

```go
package mylib

import (
    "github.com/kujirahand/nadesiko3go/internal/lexer"
    "github.com/kujirahand/nadesiko3go/internal/stdlib"
    "github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct {
    // 必要に応じて状態（DB接続ハンドルなど）を保持
}

func New() *Plugin {
    return &Plugin{}
}

// 1. 命令一覧と助詞（パーサー用）
func (p *Plugin) FuncList() lexer.FuncList {
    list := lexer.FuncList{}
    list["カスタム処理"] = &lexer.FuncItem{
        Name: "カスタム処理",
        Type: "func",
        Josi: [][]string{{"を"}, {"で"}},
    }
    return list
}

// 2. 実装関数一覧（実行時用）
func (p *Plugin) Impls() map[string]stdlib.Impl {
    return map[string]stdlib.Impl{
        "カスタム処理": func(ctx stdlib.Context, args []value.Value) (value.Value, error) {
            // 処理を記述
            return value.String("OK"), nil
        },
    }
}
```

### ステップ2: ランタイムにプラグインを登録
`internal/vm/cui.go` などでレジストリ生成時にプラグインを渡します。

```go
// internal/vm/cui.go
return stdlib.NewRegistry(
    nodelib.New(),
    csvlib.New(),
    mathlib.New(),
    sqlitelib.New(),
    mylib.New(), // ★追加
)
```

---

## 5. まとめ

- **処理を書く関数シグネチャや `value.Value` の使い方は共通**です。
- **なでしこ公式の標準命令**なら `internal/stdlib/` 直下を編集します。
- **外部拡張・独自機能**なら `stdlib.Plugin` (`FuncList()` + `Impls()`) を満たすパッケージを独立して作成し、`stdlib.NewRegistry(...)` に渡します。
