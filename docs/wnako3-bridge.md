# wnako3連携とタートル（設計）

gonako-gui に本家のブラウザ版なでしこ（`wnako3.js`）を同梱し、WebView上の
wnako3からGo側（gonako）の命令を呼べるようにする仕組みです（#63）。
教育用途のタートルグラフィックスは、本家の `plugin_turtle.js` をそのまま使います。

## 1. wnako3.js の取り込み

`scripts/copy-nadesiko3.sh`（`just copy-nadesiko3`）で `cmd/gonako-gui/ui/wnako3/` に取り込みます。

1. ローカルの本家リポジトリ（既定は `./nadesiko3`）に `release/*.js` があればコピーする
2. なければ jsDelivr から `nadesiko3@<Nadesiko>/release/*.js` を取得する。
   `<Nadesiko>` は `internal/version` の `Nadesiko` 定数。その版がnpmに無ければ `latest` を使う

取り込んだ版と取得元（`local` / `cdn`）は `ui/wnako3/VERSION` に記録され、
`getAppInfo()` の `wnako3Version`、なでしこの `WNAKOバージョン` で確認できます。
ローカルの本家とgonakoの言語バージョンが食い違うときは警告を出します。
取り込んだファイルは `//go:embed` でgonako-guiに入るので、コミットしてください。

## 2. 配信と自動インポート

gonako-gui のローカルHTTPサーバー（HTMLフォルダ、内蔵エディタ、HTMLを梱包したアプリ）は
`newSiteHandler`（`cmd/gonako-gui/wnako3.go`）で次を配信します。

| URL | 内容 |
| --- | --- |
| `/__gonako/wnako3/<名前>.js` | 同梱したwnako3のファイル |
| `…/wnako3.js`、`…/plugin_turtle.js` など | フォルダに実物が無ければ、どの階層でも同梱版を返す |

後者により、本家と同じ `<script src="wnako3.js">` や
`!「plugin_turtle.js」を取り込む` がそのまま動きます。フォルダに実物があればそちらが優先です。

WebViewには `w.Init` で `ui/gonako-loader.js` を全ページの先頭に注入します。
ローダーは `DOMContentLoaded` で次のように振る舞います。

- ページ自身が `wnako3.js` を読み込んでいれば、プラグイン `PluginGonako` を登録するだけ
- `<script type="なでしこ">` がある、または `index.json` に `"wnako3": true` があれば、
  `/__gonako/wnako3/wnako3.js` を読み込み、登録してから `runNakoScript()` で実行する
- どちらでもないページ（既存のHTMLアプリ）には何もしない

`-url` で外部のページを開いたときは注入しません。

```html
<canvas id="turtle_cv" width="640" height="400"></canvas>
<script type="なでしこ">
!「plugin_turtle.js」を取り込む
カメ作成
100だけカメ進む
「{WNAKOバージョン}で動いています」を表示
</script>
```

## 3. ブリッジ（webview Bind）

HTTPやWebSocketは使わず、既存の webview Bind に揃えています。
BindはUIスレッドで呼ばれるため、命令はgoroutineで実行し、結果はEvalで返します。

```text
JS  window.startNakoCommand(命令名, 引数JSON) ──► Go  呼び出しIDを即座に返す
                                                   │  goroutineで常駐VMの命令を実行
JS  window.__gonakoCommandDone(ID, 結果JSON) ◄──── Go  w.Dispatch → w.Eval
```

- 命令は1つの常駐VM（`commandBridge`、`cmd/gonako-gui/bridge.go`）で実行する。
  SQLiteのハンドルなど、命令の状態が呼び出しをまたいで保たれる
- 値はJSONで受け渡す（`stdlib.EncodeJSON` / `DecodeJSON`、JSON.stringifyと同じ規則）
- 結果は `{ok, value, output, error}`。`output` は命令が `表示` した文字列

### JavaScriptから

```javascript
const n = await gonako.call('文字数', 'あいう');        // 3
const out = await gonako.run('「Goから」を表示');       // "Goから\n"
console.log(gonako.version, gonako.wnako3Version);
```

`gonako.run` は既存の `startNakoCode` / `pollNakoRun` を包んだもので、
ダイアログは `prompt` / `confirm` / `alert` で応答します。

### wnako3（なでしこ）から

| 命令 | 書式 | 説明 |
| --- | --- | --- |
| `GONAKO呼出` | `「命令名」を[引数…]でGONAKO呼出` | Go側の命令を呼んで戻り値を返す |
| `GONAKO実行` | `「コード」をGONAKO実行` | Go側でなでしこを実行し、表示内容を返す |
| `GONAKOバージョン` | 定数 | gonakoのバージョン |

```nako3
N=「文字数」を[「あいう」]でGONAKO呼出
「保存」を[「こんにちは」,「hello.txt」]でGONAKO呼出
```

## 4. エディタの「ブラウザ(wnako3)」実行モード

`docs/gonako-gui-editor.md` の6節を参照してください。子プロセスが
`ui/wnako3run.html` の実行画面で、タートル付きのwnako3としてプログラムを動かします。

## 5. セキュリティ

- **信頼モデル**: gonako-gui が配信するHTML（`-dir` のフォルダ、梱包アプリ、エディタで実行する
  プログラム）は、利用者が実行するアプリ本体として信頼する。ネイティブアプリと同じく、
  利用者の権限でGo側の全命令（ファイル・プロセスなど）を使える。これはブリッジ以前からある
  `runNakoCode` / `startNakoCode` と同じ前提で、ブリッジで権限が広がるわけではない。
  信頼できないスクリプト（出所の分からないCDNのJSなど）をHTMLに読み込まないこと
- **外部ページ**: `-url` で開いた外部ページには、ローダーを注入しない
- **DNS rebinding対策**: ローカルHTTPサーバーは `loopbackOnly`（`cmd/gonako-gui/wnako3.go`）で
  Hostヘッダーが `127.0.0.1` / `localhost` / `::1` の要求だけを受け付ける。ふつうのブラウザで開いた
  悪意あるサイトが、自分のドメインを127.0.0.1へ向け直してフォルダのファイルを読むのを防ぐ。
  別オリジンからの直接の `fetch` は、CORSヘッダーを返さないので中身を読めない

## 6. 制約

- wnako3で動くのは本家のブラウザ版の言語処理系で、gonakoの互換保証（`plugin_system`）の対象外
- wnako3からGo側の関数値（コールバック）は渡せない。JSONにできる値だけを受け渡す
- `ウィンドウ作成` などGUI部品の命令は、ブリッジの常駐VMでは画面に反映されない
