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
```

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

```text
{{{#nako3
「12345」をハッシュ値計算して表示。
### GO表示結果: 5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5
}}}
```

---

## 仕様・注意点

- **ファイル拡張子**: 対象ファイルは拡張子が **`.txt`** のものに限られます。
- **対象ブロックの抽出**: `### 表示結果:` または `### WEB表示結果:` の記述がない `{{{#nako3 ... }}}` ブロックはテスト対象外（説明用コード）としてスキップされます。
