# DocTest 仕様と使い方 (gonako)

マニュアル（`manual/{プラグイン名}/{命令名}.txt`）やテストデータ（`testdata/doctest/**/*.txt`）に書かれたコードサンプルを抽出し、記述されている表示結果のとおり動作するかを自動検証する仕組みです。

---

## 使い方

### 1. gonako コマンドで実行

`gonako doctest` サブコマンドを使用します。

```bash
# デフォルト対象（manual/plugin_system、manual/gonako、testdata/doctest）を実行
gonako doctest

# ディレクトリを指定して実行
gonako doctest testdata/doctest

# 単一のテキストファイルを指定して実行
gonako doctest testdata/doctest/core/plugin_system.txt

# 複数のファイルやディレクトリを混在して指定
gonako doctest manual/plugin_system/01-文字列.txt testdata/doctest/core/

# 失敗詳細の表示件数を変更（デフォルト: 10件、0で全件表示）
gonako doctest -max 0 testdata/doctest/core/plugin_system.txt

# 外部ランタイムで実行する（一時ファイルのパスを第1引数に渡す）
gonako doctest --runtime=lnako --label=L表示結果 testdata/doctest

# lnako のようにサブコマンドが必要な場合
gonako doctest --runtime=lnako --subcommand=run --label=L表示結果 testdata/doctest

# 本家cnako3などで「表示結果」だけを回す
gonako doctest --runtime=/path/to/cnako3 --label=表示結果 manual/plugin_system
```

### なでしこ版ユーティリティで実行

既存のGo版とは別に、なでしこで実装したDocTestも利用できます。

```sh
just doctest-nako testdata/doctest/core/plugin_system.txt
bin/gonako-doctest --runtime=lnako --subcommand=run --label=L表示結果 testdata/doctest
```

`bin/gonako-doctest` は同じフォルダのgonakoを起動します。Windowsでは `bin/gonako-doctest.ps1` を使います。
実装は `gonako-package/doctest/index.nako3` にあり、Go版と同じ引数で対象・ラベル・サンプル用ランタイムを指定できます。
詳しくは [gonako-packageの仕様](gonako-package.md) を参照してください。

### 2. just で実行

```bash
# デフォルト対象を実行
just doctest

# 特定のファイルやディレクトリに絞って実行
just doctest testdata/doctest/core/plugin_system.txt
```

---

## サンプルコードの記述フォーマット

テキストファイル（`.txt`）内に `{{{#nako3` と `}}}` のブロックを記述し、その中に `### 表示結果:` を含めることで DocTest の対象となります。

### 単一行の表示結果

```text
{{{#nako3
10 + 5を表示。
### 表示結果: 15
}}}
```

### 複数行の表示結果

2行目以降は `### ` に続けて記述します。

```text
{{{#nako3
「あ{改行}い」と表示。
### 表示結果: あ
### い
}}}
```

### WEB表示結果 (WNako専用)

ブラウザ環境専用のサンプルは `### WEB表示結果:` と記述します。`gonako doctest`（CUI版）では自動的に省略（スキップ）としてカウントされます。

```text
{{{#nako3
L＝「こんにちは」のラベル作成。
Lのテキスト取得して表示。
### WEB表示結果: こんにちは
}}}
```

### GO表示結果 (gonako独自命令)

本家TypeScript版には存在しない、gonako独自の命令（`manual/gonako/` 以下）のサンプルは
`### GO表示結果:` と記述します。`### 表示結果:` と同じくCNako（CUI版）として実際に実行・検証されます。
本家の命令ではないことを明示しつつ、`gonako doctest` の対象にするための書き分けです。

`--label` を省略した既定の対象は `表示結果` と `GO表示結果` です。

```text
{{{#nako3
「12345」をハッシュ値計算して表示。
### GO表示結果: 5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5
}}}
```

### 任意ラベル（他ランタイム向け）

`### L表示結果:` のように、英数字の接頭辞を付けたラベルも同じ形式で書けます。
`--label` でそのラベルだけを対象にできます。

```text
{{{#nako3
「こんにちは」と表示。
### L表示結果: こんにちは
}}}
```

```bash
gonako doctest --runtime=lnako --subcommand=run --label=L表示結果 フォルダパス
```

これは `lnako run <一時ファイル.nako3>` を実行します。`--subcommand` を省略すると、ファイルパスだけを渡します。

ランタイム側の約束は次のとおりです。

- ソースファイル（`.nako3`）のパスを受け取る（`--subcommand` 指定時はその直後）
- `表示` の内容を標準出力に出す（末尾空白は比較前に取り除く）
- 失敗時は終了コードを非0にし、できれば標準エラーに理由を書く

外部ランタイムはサンプルごとに最大10秒で打ち切ります。期限を超えたサンプルは失敗として報告し、後続のサンプルを続けて実行します。

`--runtime` を指定したときは、内蔵VM向けの省略（`JS実行` など）は適用しません。
相手ランタイムが対応していればそのまま実行します。`--runtime` は実行ファイル1つです。
`lnako run ファイル` のようにサブコマンドが必要な場合は `--subcommand=run` を付けます。
`node src/cnako3.mjs` のように実行ファイル以外の引数がさらに必要な場合は、ラッパースクリプトを渡してください。

---

## 仕様・注意点

- **ファイル拡張子**: 対象ファイルは拡張子が **`.txt`** のものに限られます。
- **対象ブロックの抽出**: `### 表示結果:` や `### WEB表示結果:`、`### GO表示結果:`、`### L表示結果:` のように「表示結果」で終わる見出しがない `{{{#nako3 ... }}}` ブロックはテスト対象外（説明用コード）としてスキップされます。
- **外部ランタイム**: `--runtime` で実行ファイルを差し替え、`--label` で照合する見出しを選べます。`--subcommand` を付けると、ファイルの前にその引数を挿入します（`lnako run ファイル`）。省略時は内蔵VMで `表示結果` と `GO表示結果` を実行します。
