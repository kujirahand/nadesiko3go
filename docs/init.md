# 開発環境の初期化

このリポジトリを新しいディレクトリへcloneしたときは、Git管理外の参照用リポジトリと
マニュアルへのシンボリックリンクを別途用意します。

## 公式TypeScript版を配置する

公式実装との互換性確認に使う `nadesiko3` を、このリポジトリの直下へcloneします。

```sh
git clone https://github.com/kujirahand/nadesiko3.git nadesiko3
```

`nadesiko3/` は参照用の別リポジトリであり、`nadesiko3go` のGit管理には含めません。

## マニュアルを配置する

このリポジトリと同じ親ディレクトリへ `nadesiko3doc` をcloneし、その `data` ディレクトリを
`./manual` から参照する相対シンボリックリンクを作成します。

```sh
git clone https://github.com/kujirahand/nadesiko3doc.git ../nadesiko3doc
ln -s ../nadesiko3doc/data manual
```

`manual` フォルダ（または有効なシンボリックリンク）がない状態で `just doctest` を実行すると、
次の警告を表示します。警告後も、リポジトリ内の固定サンプルを対象にDocTestを継続します。

```text
[警告] manualフォルダ(nadesiko3doc/data)を./manualとしてシンボリックリンクを作成してください
```
