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
  - **「📁 ファイル」タブ**: ディレクトリを探索できる簡易ファイルブラウザ。
- **画面右側・上部（テキストエディタ）**:
  - 行番号表示、Tabキーインデント、リアルタイム文字数・行数カウントに対応。
  - macOSでは`Cmd+C`でコピー、`Cmd+V`で貼り付け、`Cmd+X`で切り取り、`Cmd+A`ですべて選択できます。
- **画面右側・下部（実行結果コンソール）**:
  - なでしこプログラムの標準出力やエラーメッセージを色分けして表示。
  - 実行時間の計測やワンクリック結果コピーに対応。
- **スプリッター**:
  - 各領域の境界線（縦・横）をドラッグしてサイズを自由に調整できます。

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
  "output": "10 + 20 = 30\n",
  "error": ""
}
```

### `window.getAppInfo()`
アプリケーションのバージョンや実行環境（OS/Arch）情報を取得します。

```javascript
const infoJson = await window.getAppInfo();
const info = JSON.parse(infoJson);
console.log(`Version: ${info.version}, OS: ${info.os}, Arch: ${info.arch}`);
```

---

## 5. コマンドラインオプション一覧

```bash
gonako-gui [オプション] [HTMLフォルダまたはファイル]
```

| オプション | 説明 | 既定値 |
|---|---|---|
| `-dir <パス>` | 配信・表示するHTMLフォルダのパス | なし（内蔵UI） |
| `-url <URL>` | 直接開くURL（`http://` または `https://`） | なし |
| `-title <文字列>` | ウィンドウのタイトルバー文字列 | `"なでしこ3 (gonako-gui)"` |
| `-width <数値>` | ウィンドウの横幅（ピクセル） | `980` |
| `-height <数値>` | ウィンドウの高さ（ピクセル） | `680` |
| `-debug` | 開発者ツール（Developer Tools / インスペクタ）を有効化 | 無効 |
| `-help`, `--help` | ヘルプメッセージを表示 | - |

---

## 6. 開発時の便利な使い方 (Tips)

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
