# WebAssembly版（ブラウザでコア機能を動かす）

なでしこ3のコア機能だけを、標準Goの `GOOS=js GOARCH=wasm` でWebAssemblyに
出力し、ブラウザ（やNode.js）で動かします。

## ビルドと確認

```sh
just wasm         # bin/wasm/ に gonako.wasm・wasm_exec.js・index.html を出力
just wasm-test    # Node.jsで読み込んで動作を確かめる（scripts/wasm-smoke.mjs）
just wasm-serve   # http://localhost:8080/ でサンプルページを開く
```

`wasm_exec.js` はGo本体に付属するもの（`$(go env GOROOT)/lib/wasm/wasm_exec.js`）を
コピーします。**ビルドに使ったGoと同じバージョンのものでないと動きません。**
`fetch` で読み込むので、`file://` ではなくHTTPサーバー経由で開いてください。

## 配布時は圧縮する

標準Goが出力する `gonako.wasm` は約7MBありますが、テキストに近い圧縮率の
良いバイナリなので、圧縮して配ればかなり小さくなります。

| | サイズ |
| --- | --- |
| 無圧縮 | 約6.9MB |
| gzip -9 | 約1.8MB |
| brotli -q 11 | 約1.3MB |

`just wasm` は `gonako.wasm` と一緒に `gonako.wasm.gz` を必ず、
`brotli` コマンドが入っていれば `gonako.wasm.br` も作ります。これらは
nginxの `gzip_static`／`brotli_static` や、Cloudflare Pages・Netlifyなど
「同名+拡張子」の事前圧縮ファイルを自動で配信するホスティングにそのまま
置けます。そうした仕組みが無いサーバーでは、`gonako.wasm` を要求された時に
サーバー側で `Content-Encoding: gzip`（または `br`）を付けて
`gonako.wasm.gz`（`.br`）の中身を返すよう設定してください。ブラウザの
`fetch("gonako.wasm.gz")` が中身を自動で展開してくれるわけではありません。

## 含まれる機能

| 含む | 含まない |
| --- | --- |
| `stdlib`（plugin_system 相当） | `nodelib`（ファイル・OS・プロセス・ネットワーク、`尋ねる`・`終了` など） |
| `mathlib`（plugin_math 相当） | `bundle`（単一ファイル梱包） |
| `csvlib`（plugin_csv 相当） | `sqlitelib`・`officelib`・`pdflib`・`imagelib`・`guilib` |

命令の組み合わせは `internal/wasmrt.Registry()` で決めています。

`言う` はブラウザの `alert` を使います。`alert` が無い環境（Node.jsなど）では
`表示` に切り替わります。`秒待機` は実時間で待ちます（実行はgoroutineで行い、
JSのイベントループを止めません）。

## 構成

```text
cmd/gonako-wasm/main.go      syscall/js でJSへAPIを公開する（//go:build js && wasm）
cmd/gonako-wasm/web/         サンプルページ
internal/wasmrt/             Host実装と命令の組み合わせ。syscall/js 非依存なのでネイティブでテストできる
internal/vm/registry_run.go  レジストリを渡して実行する経路（wasmでも使える）
internal/vm/cui.go           CUI用Hostと既定プラグイン。js/wasm ではビルドから外す
```

`internal/vm/cui.go` に `//go:build !(js && wasm)` を付けているのが要点です。
これで vm から nodelib・bundle・clipboard などへの依存が切れ、
wasmのサイズは約16MB（`gonako-cui` をそのまま出力した場合）から約7MBになります。
CUI版の `vm.NewCUIHost` や `pkg/runtime` は js/wasm ではビルドできません。

## JavaScript API

`gonako.wasm` を起動すると、グローバルに `gonako` を公開し、
`globalThis.gonakoReady` が関数ならそれを呼びます。

```html
<script src="wasm_exec.js"></script>
<script>
globalThis.gonakoReady = async (gonako) => {
  const result = await gonako.run("「こんにちは」を表示。", {
    onPrint: (line) => console.log(line),
  });
  console.log(result); // {ok: true, output: "こんにちは\n", error: null}
};
const go = new Go();
WebAssembly.instantiateStreaming(fetch("gonako.wasm"), go.importObject)
  .then(({ instance }) => go.run(instance));
</script>
```

- `gonako.version` … バージョン文字列
- `gonako.run(code, options)` … `Promise<{ok, output, error}>` を返す。失敗してもrejectせず、
  `error` に `{kind, file, line, message}` を入れる（`line` は1始まり）
  - `filename` エラー表示に使うファイル名（既定: `main.nako3`）
  - `args` プログラムに渡す引数（文字列の配列）
  - `onPrint(line)` `表示` のたびに1行を受け取る
  - `onWrite(s)` 改行しない出力を受け取る
  - `onDialog(kind, message)` `alert`/`confirm`/`prompt` の代わりに呼ばれる

同時に `run` を呼んだ場合は、呼び出し順に1本ずつ実行します。

## 今後

- **TinyGoでの出力は当面見送り**：wasm出力のサイズは標準Goの約1/4（gzip後
  約0.57MB）になり魅力的だが、TinyGoのwasmターゲットは `recover()` を
  サポートしていない（`tinygo_longjmp` がネイティブ向けにしか実装されて
  いない）。gonakoはパーサー・コンパイラ・VMの内部エラー伝達を
  panic/recoverに頼っているため、構文エラーや実行時エラーが起きるたびに
  `RuntimeError: unreachable` でwasm全体が落ちる。エラー伝達をすべて戻り値
  方式に書き換えるのは影響範囲が大きく、wasm対応のためだけにやるのは
  見合わない。主要ブラウザはwasmの例外処理提案に対応済みなので、TinyGoが
  それを使ってrecoverを実装したら移行を検討する。
- `尋ねる` などブラウザ向け命令（plugin_browser 相当）の追加
