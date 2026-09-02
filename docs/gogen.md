# Goコード生成バックエンド（gogen）の使い方

なでしこプログラムをGoソースに変換し、`go build` でネイティブ実行ファイルにする機能です（AGENTS.md §10・§12）。バイトコードVMでの実行と違い、インタプリタのディスパッチループを介さず、コンパイル済みのGoコードとして動きます。

作られたGoソースを**ビルドする側**にはGoツールチェインが必要ですが、できあがった実行ファイルを**受け取って動かす側**には何も要りません（単一ファイル梱包 `gonako build` と同じ配布モデルです。→ `docs/bundle-resources.md`）。

---

## 1. 基本的な使い方

### コマンド構文

```bash
gonako gengo <プログラム.nako3> [オプション]
```

### 最小の手順

```bash
# 1. なでしこプログラムをGoソースに変換する
gonako gengo hello.nako3 --out hello.go

# 2. 案内どおりに go.mod を用意する（後述）
# 3. ビルドして実行する
go build -o hello hello.go
./hello
```

`gonako gengo` を実行すると、次のように出力先とビルド手順が案内されます。

```text
hello.go を作りました
ビルドするには、hello.go と同じ場所に go.mod を用意し、以下を requireとreplaceに書いてください:
  require github.com/kujirahand/nadesiko3go v0.0.0
  replace github.com/kujirahand/nadesiko3go => /path/to/nadesiko3go
```

### オプション一覧

| オプション | 既定値 | 説明 |
|---|---|---|
| `--out` | `<ファイル名>.go` | 出力するGoソースファイル名 |
| `--package` | `main` | 生成するGoソースのパッケージ名 |
| `--plugins` | `nodelib,csvlib,mathlib,sqlitelib,officelib,pdflib,imagelib` | 生成コードのレジストリに含めるプラグイン（カンマ区切り）。空文字を渡すと標準命令（plugin_system）だけになる |

---

## 2. go.mod の用意（ビルドする側の準備）

生成されたGoソースは `github.com/kujirahand/nadesiko3go/pkg/runtime` だけに依存します（内部の `internal/*` パッケージは外部から直接 `import` できないので、公開ファサードの `pkg/runtime` を経由します。→ 4節）。

このリポジトリがまだ公開バージョンとして `go.mod` の `require` に書ける状態でない間は、`replace` でローカルのチェックアウトを指す必要があります。

```bash
mkdir myapp && cd myapp
mv ../hello.go .
cat > go.mod <<'EOF'
module myapp

go 1.23

require github.com/kujirahand/nadesiko3go v0.0.0

replace github.com/kujirahand/nadesiko3go => /path/to/nadesiko3go
EOF
go mod tidy
go build -o myapp .
```

`/path/to/nadesiko3go` は、このリポジトリを `git clone` した場所の絶対パスに置き換えてください。`gonako gengo` の出力メッセージには、実行時のカレントディレクトリを使った参考パスが表示されますが、これは**あくまで参考**です。実際のチェックアウト場所に合わせて書き換えてください。

このリポジトリが将来タグ付きバージョンとして公開されれば、`replace` は不要になり、`require github.com/kujirahand/nadesiko3go v3.6.0` のような通常の依存として書けるようになります。

---

## 3. プラグインを使う場合

`ファイル読込` や `CSV取得` のようなplugin_system以外の命令（`nodelib`・`csvlib`など）を使うプログラムを変換するときは、`--plugins` で必要なプラグインを指定します。既定値（`nodelib,csvlib,mathlib,sqlitelib,officelib,pdflib,imagelib`）は `gonako` 本体が使っているのと同じ組み合わせなので、通常のなでしこプログラムはそのままで動きます。

```bash
# CSVの命令を使うプログラムを変換する（既定のままでよい）
gonako gengo report.nako3 --out report.go

# 標準命令（plugin_system）だけに絞りたいとき
gonako gengo pure.nako3 --out pure.go --plugins ""

# 一部のプラグインだけを使いたいとき
gonako gengo image_only.nako3 --out image_only.go --plugins imagelib
```

### なぜプラグインの指定が重要か

標準命令のID（内部の識別番号）は、レジストリに登録したプラグインの**完全な組み合わせ**（名前順にソートしたもの）で決まります。`gonako gengo` は、プログラムをコンパイルするときに使ったレジストリと、生成したGoコードが実行時に組み立てるレジストリの両方に**同じ**プラグイン一覧を使うので、この点は自動的に正しくなります。

ただし、生成済みのGoソースを手で書き換えて別のプラグイン構成に差し替えるようなことをすると、コンパイルエラーにも実行時エラーにもならず、**まったく別の命令が黙って呼ばれる**ことがあります。プラグイン構成を変えたいときは、`--plugins` を変えて `gonako gengo` を再実行してください。

---

## 4. 対応している機能・対応していない機能

### 対応している

VM（バイトコードインタプリタ）が実行できるものは、ほぼそのままGoコードとして生成できます。

- 変数・配列・辞書・演算子（AGENTS.md §4の値モデルをそのまま使用）
- 制御構文（もし・繰り返し・条件分岐など）
- ユーザー定義関数、再帰呼び出し
- クロージャ・変数のキャプチャ（`関数()...ここまで` の入れ子）
- エラー監視（`エラー監視..エラーならば..ここまで`）。入れ子も可
- 動的な関数呼び出し（値として渡された関数を呼ぶ）
- plugin_system（標準命令）と、`--plugins` で指定したプラグインの命令すべて

### 対応していない

- **非同期関数**（`Func.Async`）。Goのコールスタックをgoroutine外で中断・再開する自然な対応がないため、`gonako gengo` は非同期関数を検出すると、理由付きのエラーで変換を止めます。そのプログラムは、VMバックエンド（`gonako run` / `gonako <ファイル>`）で実行してください

```text
$ gonako gengo async_example.nako3
gogenは非同期関数に対応していません（VM実行にフォールバックしてください）: [非同期テスト]
```

---

## 5. 仕組み（開発者向け）

`internal/gogen` が生成するのは、実は**制御フローだけ**です。値の演算・添字アクセス・命令呼び出し・クロージャ生成といった意味論はすべて、既存のバイトコードインタプリタ（`internal/vm`）が持つ実装にそのまま委譲します。

```text
IR ──► internal/gogen ──► Goソース ──► go build ──► ネイティブ実行ファイル
```

- `internal/vm/export.go`: インタプリタの内部関数（`binary`・`indexGet`・`callStd`・`makeClosure` など）を薄くエクスポートしたもの
- `pkg/runtime`: 上記を型エイリアスで包んだ公開ファサード。生成コードが依存する唯一のパッケージ
- `internal/gogen`: `ir.Func` 1つにつき1つのGo関数を生成する。バイトコードを `goto` ラベル付きの直線コードへ機械的に展開したもので、`internal/vm/run.go` の `execute()` と1対1対応する

この設計により、標準命令の実装を二重に持つ必要がなく（AGENTS.md §12）、VMとgogenが挙動で食い違うリスクも構造的に小さくなっています。

### エラー監視の実装

エラー監視（`OpTry`/`OpEndTry`）を使う関数だけは、通常と違う形で生成されます。Goの `recover()` はパニックした呼び出しへ戻ることができない（呼び出し元に戻って終わるだけ）ため、`internal/vm/run.go` の `run()`/`protect()` と同じ「再試行ループ + 使い捨ての実行フレーム」構成をそのまま生成します。エラーを捕まえるたびに、フレームを作り直して途中から再開する、という動きです。

---

## 6. 検証方法

### 単体テスト

```bash
go test ./internal/gogen/
```

代表的なプログラム（四則演算・文字列・配列・辞書・再帰関数・クロージャ・エラー監視・プラグイン使用）について、実際に `go mod tidy` → `go build` → 実行まで行い、VMバックエンドの出力と一致することを確かめます。

### 差分fixtureとの3系統比較

AGENTS.md §1の差分fixture（TypeScript版 / Go VM / Go生成コード）のうち、「Go VM / Go生成コード」の一致は次のコマンドで確かめられます。

```bash
GOGEN_COMPAT=1 go test ./internal/gogen/ -run TestCompatBatch -v
```

差分fixtureの各ケースを、なでしこVMとgogenの両方でコンパイル・実行し、出力を突き合わせます。1ケースにつき `go build` 相当の処理をするため数分かかるので、通常の `go test ./...` には含めていません（gogenを触った後の手動確認用です）。

```text
合計: 230/230 通過（skip 11）
```

skipになるのは、非同期関数を使うケース（`10_async` グループ）と、実行時エラーで終わる意図のケース（ログを比較できないため）です。

---

## 7. 制限・既知の注意点

- **速度**: 生成されたコードは、なでしこの値（`value.Value`）をそのまま操作する点はVMと同じです。バイトコードのディスパッチ（`switch`文での命令分岐）が無くなる分は速くなりますが、V8並みのJITコンパイルのような最適化は行っていません
- **デバッグのしやすさ**: 生成されたGoソースは人間にも読める形（`go/format` で整形済み）ですが、`goto` を多用した機械的な出力なので、手で保守する用途は想定していません。プログラムを直したいときは、なでしこのソースを直して `gonako gengo` をやり直してください
- **配布**: 単一の実行ファイルとして配布したい場合、gogenよりも `gonako build`（→ `docs/bundle-resources.md`）の方が、Goツールチェインなしでクロスプラットフォームに配布できる点で手軽です。gogenが向いているのは、**速度が必要**で、かつビルド環境を用意できる場合です
