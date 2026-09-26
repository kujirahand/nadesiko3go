# gonako-package と取込パス

なでしこで書いたライブラリは `gonako-package/<package-name>.nako3` に配置します。

```nako3
!「doctest.nako3」を取り込む
```

入口は `gonako-package/xxx.nako3`、関連するソース・データは `gonako-package/xxx/` 以下に配置します。
入口から関連ソースを取り込む場合は `!「./xxx/helper.nako3」を取り込む` と書きます。
名前空間はディレクトリを除いたファイル名から作るため、入口の名前はパッケージ間で一意にしてください。
複数パッケージの入口を `index.nako3` に揃える配置は使用しません。
関連ソースも同名ファイルが別パッケージにあると名前空間が衝突するため、現状は `xxx-helper.nako3` のように一意の名前を付けてください。
大規模開発でフォルダを含むパッケージをどのように配置・識別するかは未決定です（[Issue #165](https://github.com/kujirahand/nadesiko3go/issues/165)）。

## パスの解決順

- `貯蔵庫:xxx.nako3`（全角コロンも可）は `https://n3s.nadesi.com/plain/xxx.nako3` から取得します。貯蔵庫の表示ページ `/show/` ではなくソース用の `/plain/` を使用します。
- `https://.../xxx.nako3` または `http://.../xxx.nako3` はURLから取得します。
- `./xxx.nako3`、`../xxx.nako3` は取込元ファイルのディレクトリを基準に解決します。ファイル名のない実行ではカレントディレクトリが基準です。絶対パスも指定できます。
- 接頭辞のない `xxx.nako3`、`package-name.nako3` は、次の順に探索します。
  1. 環境変数 `GONAKO_PACKAGE_PATH` の各ディレクトリ
  2. ランタイム実行ファイルと同じディレクトリの `gonako-package/`
  3. ランタイム実行ファイルの一つ上のディレクトリの `gonako-package/`（`bin/` 配置用）
  4. Goのビルド時に埋め込まれた `gonako-package/`

`GONAKO_PACKAGE_PATH` はmacOS/Linuxでは `:`、Windowsでは `;` で複数ディレクトリを区切れます。
ランタイムの位置はシンボリックリンクの実体を基準にします。
裸のファイル名でローカルファイルを取り込んでいたコードは `./` を付けてください。
拡張子は `.nako3` と `.nako` に対応します。パッケージ名だけを指定する `!「doctest」を取り込む` は `doctest.nako3` に展開します。

URL内の `./`、`../` は取込元URLを基準に、埋め込みパッケージ内では取込元の仮想ディレクトリを基準に解決します。
相対パスを明示した場合は、その場所に存在しなければエラーとなり、パッケージ探索には移りません。
同じ解決済みパスのファイルは一度だけ取り込み、循環取込も防ぎます。
URL取得には15秒のタイムアウトと8MiBのサイズ上限があります。

## 配置と埋め込み

```text
<root>/
├── bin/gonako
├── bin/gonako-doctest   # なでしこ版DocTestの起動用ファイル
└── gonako-package/
    ├── doctest.nako3    # 入口
    └── doctest/         # 関連ファイル（必要な場合）
```

リポジトリ直下の `gonako-package/` は `packages.go` の `go:embed` で埋め込みます。
パッケージを追加・更新したらランタイムを再ビルドしてください。
埋め込みは通常のGoビルドに含まれ、CUI・GUI・Goコード生成時の共通パーサーから利用されます。
単一ファイル梱包では取込をビルド時に解決してIRへ組み込むため、実行時のソース取込は不要です。

## なでしこ版DocTest

`gonako-package/doctest.nako3` が抽出・選択・照合・集計を実装します。
実行ファイル（起動用ファイル）のパスは `bin/gonako-doctest` とし、パッケージ名は `doctest` のままです。
既存の `gonako doctest`（Go版）はそのまま利用できます。

```sh
bin/gonako-doctest -max 0 testdata/doctest/core/plugin_system.txt
bin/gonako-doctest --runtime=cnako3 --label=表示結果 manual/plugin_system
bin/gonako-doctest --runtime=lnako --subcommand=run --label=L表示結果 testdata/doctest
```

Windowsでは `bin/gonako-doctest.ps1` を使います。起動用ファイルは同じフォルダの `gonako` を優先し、なければPATHから探します。
`GONAKO_RUNTIME` で起動するgonakoを指定できます。サンプル用の既定ランタイムも同じ実行ファイルになります。
ソースを直接起動する場合は `GONAKO_DOCTEST_RUNTIME` でサンプル用ランタイムを指定できます（既定はPATH上の `gonako`）。

Go版と同じ `-max`、`--runtime`、`--subcommand`、`--label` と対象パスに対応します。
ラベル省略時は `表示結果` と `GO表示結果`、対象省略時は存在する `manual/plugin_system`、`manual/gonako`、`testdata/doctest` を選びます。
期待出力は複数行・CRLF・全角コロンに対応し、末尾空白を除去して比較します。
既定実行ではGo版の省略命令を省略し、外部ランタイム指定時は省略しません。
サンプルは別プロセスで10秒を上限に実行します。成功は終了コード0、テスト失敗は1、引数や対象読込のエラーは2です。
なでしこ版の起動にはgonakoを使用し、サンプルの実行先は任意のCUIランタイムへ変更できます。

### コマンド実行待機の辞書指定（Go版）

従来のシェル文字列指定に加えて、次の辞書を指定できます。引数はシェルを介さず渡し、標準出力と標準エラーはUTF-8として取得します。

```nako3
設定={"実行ファイル":"gonako","引数":["sample.nako3"],"秒":10}
結果=設定をコマンド実行待機
結果["標準出力"]を表示
```

戻り値のキーは `標準出力`、`標準エラー`、`終了コード`、`時間切れ`、`エラー` です。
プロセスの失敗は戻り値で報告し、後続のテストを実行できます。起動できない場合の終了コードは-1です。
`秒` は既定10、0より大きく3600以下を指定します。
