# 命令・マニュアルの検索（`gonako doc`）

`gonako doc KEYWORD` は、なでしこの命令やマニュアルを検索するサブコマンドです（#51）。
ターミナルから使う人だけでなく、AIエージェントが命令の使い方を調べるための入り口も兼ねています。

```bash
gonako doc 秒待               # 命令一覧から検索する（既定）
gonako doc ファイル 読む       # 複数キーワードのAND検索
gonako doc 秒待 --web         # Webのマニュアルも検索する
gonako doc 秒待 --json        # 結果をJSONで出力する
gonako doc 文字列 --limit 0   # 全件表示する
```

| オプション | 説明 |
|---|---|
| `--command`, `-c` | 命令一覧(JSON)から検索する（既定の動作なので、普段は省略できる） |
| `--web`, `-w` | Webのマニュアル（<https://nadesi.com/v3/doc/>）も検索する |
| `--json` | 結果をJSONで出力する |
| `--limit N` | 表示する件数の上限（既定: 20、`0`で全件） |

## 命令一覧はどこから来るのか

検索の対象は `internal/commanddoc/command-list.json` です。これは
`just gen-command-list`（`scripts/gen-command-list.go`）が、Go版のレジストリと
本家TypeScript版のプラグインのコメントから生成したものです。生成時に、
GUIエディタが読む `cmd/gonako-gui/ui/command-list.json` と同じ内容を2か所へ書き出しています。

`internal/commanddoc` はこのJSONを `//go:embed` でバイナリに埋め込むので、
**ネットにつながっていなくても検索できます**。命令を追加・変更したら
`just gen-command-list` を実行して、この一覧を更新してください。

軽量CUI版（`gonako-cui`）はバイナリサイズを優先しているため、`doc` を持ちません。

## 一致度の付け方

キーワードごとに次の点数を付け、合計の高い順（同点なら命令名順）に並べます。
0点（＝どこにも一致しない）の命令は結果に含みません。キーワードを複数与えた場合は、
そのすべてに一致した命令だけが残ります（AND検索）。

| 一致した場所 | 点数 |
|---|---|
| 命令名と完全一致 | 100 |
| 命令名の前方一致 | 80 |
| 命令名の部分一致 | 60 |
| 読みがな | 40 |
| 説明 | 20 |
| 分類・プラグイン名 | 10 |

点数は `--json` の出力にも `score` として入ります。並び順の理由が分かるようにするためです。

## Web検索の仕組み

マニュアルはkona3というWikiで動いています。`--web` は検索フォームと同じ
`POST https://nadesi.com/v3/doc/index.php?FrontPage&search`（`a_mode=search`, `a_key=キーワード`）を
送り、返ってきたHTMLの `<!-- result -->` 以降にあるリンクの一覧を取り出します。
メニューのリンクを拾わないよう、結果欄の範囲に絞ってから解析しています。

Webへつながらないことは普通にあるので、失敗しても命令の検索結果は表示し、
エラーは標準エラー出力（`--json` では `web_error`）へ回すだけにしています。

なお、個々の命令の解説ページのURLは `https://nadesi.com/v3/doc/index.php?gonako/命令名` です。
Web検索をしなくても、検索結果の各命令に `マニュアル:`（JSONでは `doc_url`）として表示されます。
