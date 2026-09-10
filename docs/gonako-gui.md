# gonako-gui とは

`gonako-gui` は、日本語プログラミング言語「なでしこ3」のGo言語実装（gonako）におけるデスクトップGUI環境です。

OS標準のWebViewエンジン（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を採用しており、Node.jsやElectronなどの外部依存なしに、単一バイナリで軽量・高速に動作し、日本語入力（IME）にも完全対応しています。

主な役割として以下の2点があります。
- **内蔵Webエディタ（開発環境）**: なでしこプログラムの作成・編集・実行、標準命令一覧やひな形の検索、ファイル管理、実行可能ファイルへの変換が行える統合環境。
- **WebViewアプリケーション実行ランタイム**: ユーザーが作成したHTML/CSS/JavaScriptアプリや、なでしこスクリプトを同梱（バンドル）した単一実行ファイルの実行基盤。

## gonako-gui のパラメータと指定例

### コマンドライン書式
```bash
gonako-gui [オプション] [HTMLフォルダ / HTMLファイル / なでしこファイル / URL]
```

### オプション一覧
| オプション | 型 | 既定値 | 説明 |
|---|---|---|---|
| `-dir <パス>` | 文字列 | なし | 配信・表示するHTMLフォルダのパス |
| `-url <URL>` | 文字列 | なし | 直接開くURL（`http://` または `https://`） |
| `-title <文字列>` | 文字列 | `"なでしこ3 (gonako-gui)"` | ウィンドウのタイトルバー文字列 |
| `-width <数値>` | 整数 | `1080` | ウィンドウの横幅（ピクセル） |
| `-height <数値>` | 整数 | `720` | ウィンドウの高さ（ピクセル） |
| `-debug` | 真偽値 | 無効（`false`） | 開発者ツール（デバッグモード）を有効化（環境変数 `GONAKO_DEBUG=1` でも可） |
| `-help`, `--help` | - | - | ヘルプメッセージを表示 |

### パラメータ指定例
- **引数なしで内蔵エディタを起動**
  ```bash
  gonako-gui
  ```
- **なでしこプログラムファイルを指定して内蔵エディタで開く**
  ```bash
  gonako-gui ./main.nako3
  ```
- **自作のHTMLフォルダを指定して起動**
  ```bash
  # フォルダを指定（フォルダ内の index.html が自動表示されます）
  gonako-gui ./my-app/

  # -dir オプションで指定し、ウィンドウタイトルやサイズを指定
  gonako-gui -dir ./my-app/ -title "マイアプリ" -width 1280 -height 800
  ```
- **特定のHTMLファイルを直接指定して起動**
  ```bash
  gonako-gui ./my-app/custom_page.html
  ```
- **外部URLやローカル開発サーバー（Viteなど）を開く**
  ```bash
  # 位置引数で直接URLを指定
  gonako-gui https://example.com

  # -url オプションと -debug を指定してローカルサーバーを開く
  gonako-gui -url http://localhost:5173 -debug
  ```

## gonako-gui の起動処理順(箇条書きで)

1. **同梱アプリ（バンドルバイナリ）の判定・実行 (`runBundledApp`)**:
   - 実行ファイル自身の末尾にパッケージングされたペイロード（ZIPアーカイブ・フッタ）が存在するか検査（`bundle.Open`）。
   - **同梱HTMLアプリの場合**: 同梱ZIPリソースを読み込み、ローカルHTTPサーバーで配信してWebViewウィンドウを開いて終了。
   - **同梱なでしこプログラムの場合**: バイトコード（IR）をロードしてGUIセッションを開始し、画面部品やダイアログを表示するWebViewウィンドウを開いて終了。
   - ペイロードが同梱されていない通常の `gonako-gui` の場合は次の処理へ進む。
2. **コマンドライン引数とフラグの解析**:
   - `flag.FlagSet` で `-dir`, `-url`, `-title`, `-width`, `-height`, `-debug` などを解析（環境変数 `GONAKO_DEBUG` も反映）。
   - 位置引数（第1引数）がある場合、その種別を判定：
     - `http://` / `https://` → 表示対象URL（`targetURL`）に設定。
     - ディレクトリ → 表示対象フォルダ（`targetDir`）に設定（開始ページは `index.html`）。
     - `.html` / `.htm` ファイル → 親ディレクトリを `targetDir`、ファイル名を `startPage` に設定。
     - `.nako3` やテキストファイル → 内蔵エディタで開く初期ファイル（`initialFile`）に設定。
3. **HTTPコンテンツ配信サーバーの準備と起動**:
   - **外部URL指定時**: ローカルサーバーは起動せず、指定されたURLをそのままナビゲーション先とする。
   - **HTMLフォルダ／ファイル指定時**: 対象フォルダをルートとする `http.FileServer` を生成。
   - **指定なし／ファイル指定時（内蔵エディタ）**: Goの `//go:embed ui/*` から組み込みWebエディタのアセット群を配信する `http.FileServer` を生成。
   - ループバックアドレスの空きポート（`127.0.0.1:0`）をリスンし、バックグラウンドgoroutineでHTTPサーバーを起動してアクセスURL（`http://127.0.0.1:<ポート>/<開始ページ>`）を決定。
4. **WebViewインスタンスの生成とウィンドウ設定**:
   - `webview.New(*debugFlag)` により、OS標準のWebViewインスタンス（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を生成。
   - ウィンドウタイトル（`-title`）およびサイズ（`-width`, `-height`）を設定。
5. **Go ↔ JavaScript双方向バインディング関数の登録 (`w.Bind`)**:
   - JavaScript側から呼び出せる各種APIをWebViewに登録：
     - **なでしこプログラム実行・制御**: `runNakoCode`, `startNakoCode`, `runNakoFile`, `startNakoFile`, `pollNakoRun`, `resolveNakoDialog`
     - **画面部品（GUI）イベント連携**: `startNakoEvent`（イベントも非同期に実行し、`pollNakoRun` / `resolveNakoDialog` で進める。同期実行するとハンドラ内の『言う』でウィンドウが固まる → #59）
     - **システム・エディタ情報取得**: `getAppInfo`（OS/Arch/バージョン/初期ファイル等）、`getCommandList`（命令一覧）、`getTemplateList`（ひな形一覧）
     - **ファイル管理・OS連携**: `listFiles`, `readFile`, `saveFile`, `createNewFile`, `revealInFinder`（Finder/Explorer表示）
     - **OS標準ダイアログ連携**: `showOpenFileDialog`, `showSaveFileDialog`
     - **ビルド・パッケージング機能**: `buildAppFromFolder`（フォルダから実行ファイル変換）、`buildWithGo`（Goソース出力・ビルド）
6. **ページ読み込みとイベントループ開始**:
   - `w.Navigate(finalURL)` で対象URL（または内蔵エディタURL）の読み込みを開始。
   - `w.Run()` を呼び出してGUIのメインイベントループを開始し、ウィンドウを表示（ウィンドウが閉じられるまでブロック）。
   - 終了時に `defer w.Destroy()` や `listener.Close()` によりリソースを解放。

---

# gonako-gui 使い方ガイド

`gonako-gui` は、なでしこ3 Go言語版のデスクトップGUI環境です。

OS標準のWebViewエンジン（macOS: WKWebView, Windows: WebView2, Linux: WebKitGTK）を採用しており、軽量・高速で日本語入力（IME）に完全対応しています。

標準で内蔵されているWebエディタを起動できるほか、**自作のHTML/CSS/JavaScriptフォルダを指定して独自のデスクトップアプリとして起動**することも可能です。

---

## 1. 基本的な起動方法

### 内蔵エディタで起動する
引数なしで起動すると、内蔵のなでしこ3Webエディタが立ち上がります。また、なでしこプログラムのファイルを引数に指定して直接エディタで開くことも可能です。

```bash
gonako-gui                   # 新規プログラムでエディタを起動
gonako-gui ./main.nako3       # 指定したファイルをエディタで開いて起動
```

### 内蔵エディタの画面構成
- **ツールバー（画面上部）**:
  - **「📄 新規」ボタン**: 新しいプログラム編集を開始（`Ctrl+N` / `Cmd+N`）。
  - **「📂 開く」ボタン**: OS標準のファイル選択ダイアログを開き、ファイルを選択して読み込み（`Ctrl+O` / `Cmd+O`）。
  - **「💾 保存」ボタン**: ファイルを保存（`Ctrl+S` / `Cmd+S`）。新規ファイルの場合はOS標準の「名前を付けて保存」ダイアログが表示されます。`Ctrl+Shift+S` / `Cmd+Shift+S` で名前を付けて保存。
  - **「▶ 実行」ボタン**: 編集中のプログラムを実行（`F5` / `Ctrl+R` / `Cmd+R` / `Ctrl+Enter`）。
  - **「種類」選択**: 「ウィンドウ」または「コマンドライン」アプリの実行モードを切り替え。
  - **「📖」ボタン**: 画面左側のツールパネル（命令・ひな形・ファイル）の表示/非表示を切り替え（`Ctrl+B` / `Cmd+B`）。
  - **「☰」メニュー**: 実行ファイルへの変換、Go言語ビルド、ショートカットキー一覧などを表示。
- **画面左側（ツールパネル）**:
  - **「📖 命令」タブ**: 全標準命令の一覧を表示。リアルタイム検索で命令名や助詞を絞り込めます。
  - **「📄 ひな形」タブ**: 各種サンプルのなでしこプログラム一覧。ワンクリックでエディタに読み込めます。
  - **「📁 ファイル」タブ**: ディレクトリを探索できる簡易ファイルブラウザ。一覧の更新・上のフォルダへの移動に加え、下部の「📂 フォルダ開く」で現在のフォルダをFinder／Explorerに表示し、「新規フォルダ」でフォルダを作成できます。ツールバーの「📂 開く」でファイルを選んだ場合も、その親フォルダへ自動的に移動します。
- **画面右側・上部（テキストエディタ）**:
  - 行番号表示、Tabキーインデント、リアルタイム文字数・行数カウントに対応。
  - macOSでは`Cmd+C`でコピー、`Cmd+V`で貼り付け、`Cmd+X`で切り取り、`Cmd+A`ですべて選択できます。
  - UTF-8以外のファイルは読み込み前に確認します。Shift_JISとして読み込んだCSVなどは文字コードを記憶し、保存時もShift_JIS形式を維持します。
- **画面右側・下部（実行結果コンソール）**:
  - なでしこプログラムの標準出力やエラーメッセージを色分けして表示。
  - 実行時間の計測やワンクリック結果コピーに対応。
- **スプリッター**:
  - 各領域の境界線（縦・横）をドラッグしてサイズを自由に調整できます。

### なでしこからOS標準ダイアログを使う

`gonako-gui` で実行するなでしこプログラムでは、ファイルを開く場所、保存先、フォルダをOS標準ダイアログで選択できます。選択結果は絶対パスの文字列です。キャンセルした場合は空文字列を返します。

```nako3
# .txtファイルを選択
「.txt」のファイル選択
それを表示

# .txtファイルの保存先を選択
「.txt」の保存ファイル選択
それを表示

# 母艦パスを開始位置にしてフォルダを選択
母艦パスでフォルダ選択
それを表示
```

拡張子には `.txt`、`txt`、`*.txt` のいずれの形式も指定できます。`*.*` または空文字列を指定すると、すべてのファイルが対象になります。

---

## 2. 自作のHTMLフォルダを指定して起動する

HTMLファイルやWebアプリのフォルダパスを引数に指定することで、独自のUIを持ったデスクトップアプリを起動できます。

### ① フォルダを指定して起動（最もおすすめ）
指定したフォルダ内の `index.html` が自動的に読み込まれます。

```bash
# 相対パスで指定
gonako-gui ./my-app/

# -dir オプションで指定
gonako-gui -dir ./my-app/ -title "マイアプリ"
```

### ② 特定のHTMLファイルを指定して起動
`index.html` 以外のHTMLファイルを直接指定して開くことも可能です。

```bash
gonako-gui ./my-app/custom_page.html
```

### ③ 開発用ローカルサーバー（ViteやWebpackなど）を開く
`-url` オプションを使用すると、ローカル開発サーバーや指定のURLを直接WebViewで開くことができます。

```bash
gonako-gui -url http://localhost:5173 -title "開発プレビュー"
```

---

## 3. HTMLフォルダの構成例

フォルダ内には通常のWebサイトと同様に HTML / CSS / JavaScript / 画像 などを自由に配置できます。

### 構成例:
```text
my-app/
├── index.html      # 最初に読み込まれる画面
├── style.css       # スタイルシート
├── app.js          # JavaScript処理
└── images/
    └── logo.png    # 画像などの静的ファイル
```

`index.html` 内では、通常の相対パス（`<link rel="stylesheet" href="style.css">` や `<img src="images/logo.png">`）でリソースを読み込めます。

---

## 4. JavaScriptから「なでしこ3」を実行する連携API

`gonako-gui` 上で動作するHTML/JavaScriptからは、Go側で提供される以下のバインディング関数を呼び出すことができます。

### `window.runNakoCode(code)`
なでしこ3のプログラム文字列をGoの実行エンジン（`internal/vm`）で実行し、結果をJSON形式の文字列で返します。

#### JavaScript側の実装例:
```javascript
async function executeNadesiko() {
  const code = `
A = 10
B = 20
「{A} + {B} = {A + B}」と表示
`;

  try {
    // なでしこプログラムを実行
    const resultJson = await window.runNakoCode(code);
    const result = JSON.parse(resultJson);

    if (result.ok) {
      console.log("実行結果:\n" + result.output);
      document.getElementById("output").textContent = result.output;
    } else {
      console.error("エラー:\n" + result.error);
      alert("実行エラー: " + result.error);
    }
  } catch (err) {
    console.error("通信エラー:", err);
  }
}
```

#### 戻り値のJSON構造 (`RunResult`):
```json
{
  "ok": true,
  "runId": 1,
  "output": "10 + 20 = 30\n"
}
```

`runId`と、画面操作時だけ返る`operations`は内蔵GUIが画面部品とイベントを管理するための値です。
通常のJavaScript連携では`ok`、`output`、`error`だけを利用できます。

ダイアログ命令を含むプログラムを独自HTMLから実行する場合は、UIを止めない
`window.startNakoCode(code)`を使い、`window.pollNakoRun(runId)`で進捗を取得します。
`dialog`が返ったらHTMLで回答画面を表示し、
`window.resolveNakoDialog(runId, dialog.id, text, accepted)`で回答を返してください。
内蔵エディタと変換済みアプリは、この非同期APIを自動的に使用します。

### `window.getAppInfo()`
アプリケーションのバージョンや実行環境（OS/Arch）情報を取得します。

```javascript
const infoJson = await window.getAppInfo();
const info = JSON.parse(infoJson);
console.log(`Version: ${info.version}, OS: ${info.os}, Arch: ${info.arch}`);
```

---

## 5. なでしこから画面部品を操作する

内蔵エディタで種類を「ウィンドウ」にして実行するか、`.nako3`を実行ファイルに
変換すると、なでしこの命令だけでラベル、入力欄、ボタン、フォームを作れます。
画面部品を作る命令は、後続の命令で使う数値ハンドルを返します。

```nako3
「<h2>簡単なフォーム</h2>」をHTML表示
「名前:」のラベル作成
名前入力=「太郎」のエディタ作成
ボタン=「挨拶する」のボタン作成
結果=「」のラベル作成

ボタンをクリックした時には
　名前=名前入力のテキスト取得
　結果に「こんにちは、{名前}さん！」をテキスト設定
ここまで
```

| 分類 | 命令 |
|---|---|
| 表示 | `表示`、`HTML表示` |
| 作成 | `ラベル作成`、`エディタ作成`、`ボタン作成`、`フォーム作成`、`送信ボタン作成` |
| 値 | `テキスト設定`、`テキスト取得` |
| DOM検索 | `DOM要素取得`、`DOM要素ID取得`、`DOM要素全取得` |
| DOM内容 | `DOMテキスト変更`、`DOMテキスト取得`、`HTML変更`、`HTML取得` |
| 外観 | `DOMスタイル一括設定`、`DOM属性設定`、`DOM属性一括設定` |
| フォーカス | `DOM注目`（公式名の`注目`も利用可能） |
| イベント | `クリック時`、`変更時`、`フォーム送信時` |

`表示`は文字を安全なテキストとして追加します。HTMLタグを画面に配置する場合だけ
`HTML表示`を使用してください。

DOM検索命令は画面部品作成命令と`HTML表示`で追加した要素を対象にし、タグ名、`#id`、
`.class`、`tag#id`、`tag.class`、属性セレクターを利用できます。取得結果はほかの
画面部品作成命令と同じ数値ハンドルで、`DOM要素全取得`はハンドルの配列を返します。
`DOM注目`には数値ハンドルのほか、`「#名前」をDOM注目`のようにセレクターも直接指定できます。

フォームのイベントが発生すると、画面上の入力欄の最新値がGo側へ送られます。
個別の入力欄は`テキスト取得`で読めます。`フォーム作成`が生成した名前付き入力欄は、
コールバック内で`フォーム値["項目名"]`としても参照できます。

---

## 6. HTMLダイアログを使う

ウィンドウモードでは、`言う`、`尋ねる`、`二択`をWebView内のHTMLダイアログとして
表示します。macOSのWebKitでネイティブダイアログAPIに依存しないよう、プログラムを
非同期で実行し、回答が返ってから次の命令へ進みます。

```nako3
「メッセージ」と言う
名前=「名前を入力」と尋ねる
もし、「続けますか？」で二択ならば
　「続けます」と言う
ここまで
```

- `言う`はOKボタン付きのメッセージを表示します。
- `尋ねる`は入力欄を表示します。数値らしい入力は数値になり、キャンセル時は空文字になります。
- `二択`はOKで真、キャンセルで偽を返します。

---

## 7. コマンドラインオプション一覧

```bash
gonako-gui [オプション] [HTMLフォルダまたはファイル]
```

| オプション | 説明 | 既定値 |
|---|---|---|
| `-dir <パス>` | 配信・表示するHTMLフォルダのパス | なし（内蔵UI） |
| `-url <URL>` | 直接開くURL（`http://` または `https://`） | なし |
| `-title <文字列>` | ウィンドウのタイトルバー文字列 | `"なでしこ3 (gonako-gui)"` |
| `-width <数値>` | ウィンドウの横幅（ピクセル） | `1080` |
| `-height <数値>` | ウィンドウの高さ（ピクセル） | `720` |
| `-debug` | 開発者ツール（Developer Tools / インスペクタ）を有効化 | 無効 |
| `-help`, `--help` | ヘルプメッセージを表示 | - |

---

## 8. 開発時の便利な使い方 (Tips)

### 開発者ツール（デバッグモード）を使う
`-debug` オプションを付けて起動すると、右クリックメニューから「要素を調査（Inspect）」を選択してChrome DevToolsやSafari Web Inspectorを利用できます。

```bash
gonako-gui -dir ./my-app/ -debug
```

### 環境変数でのデバッグモード指定
環境変数 `GONAKO_DEBUG=1` を設定することでもデバッグモードを有効化できます。

```bash
GONAKO_DEBUG=1 gonako-gui ./my-app/
```
