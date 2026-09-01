# DocTest 仕様と使い方 (gonako)

マニュアル（`manual/{プラグイン名}/{命令名}.txt`）やテストデータ（`testdata/doctest/**/*.txt`）に書かれたコードサンプルを抽出し、記述されている表示結果のとおり動作するかを自動検証する仕組みです。

---

## 使い方

### 1. gonako コマンドで実行

`gonako doctest` サブコマンドを使用します。

```bash
# デフォルト対象（manual/plugin_system および testdata/doctest）を実行
gonako doctest

# ディレクトリを指定して実行
gonako doctest testdata/doctest

# 単一のテキストファイルを指定して実行
gonako doctest testdata/doctest/core/plugin_system.txt

# 複数のファイルやディレクトリを混在して指定
gonako doctest manual/plugin_system/01-文字列.txt testdata/doctest/core/

# 失敗詳細の表示件数を変更（デフォルト: 10件、0で全件表示）
gonako doctest -max 0 testdata/doctest/core/plugin_system.txt
```

### 2. Makefile で実行

```bash
# デフォルト対象を実行
make doctest

# 特定のファイルやディレクトリに絞って実行
make doctest DOCTEST_ARGS="testdata/doctest/core/plugin_system.txt"
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

---

## 仕様・注意点

- **ファイル拡張子**: 対象ファイルは拡張子が **`.txt`** のものに限られます。
- **対象ブロックの抽出**: `### 表示結果:` または `### WEB表示結果:` の記述がない `{{{#nako3 ... }}}` ブロックはテスト対象外（説明用コード）としてスキップされます。
