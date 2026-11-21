# gonako-gui をビルドする

`gonako-gui` は、なでしこ3 Go版のデスクトップGUIです。OS標準のWebViewをcgoでリンクします。

| OS | WebView | ビルドに追加で要るもの |
|---|---|---|
| Linux | WebKitGTK（GTK 3） | `gcc` / `g++` / `pkg-config`、`gtk+-3.0`、`webkit2gtk-4.0` **または** `webkit2gtk-4.1` |
| macOS | WKWebView（システムのWebKit） | Xcodeコマンドラインツール（`clang` と `WebKit.framework`）。追加のライブラリは不要 |
| Windows | WebView2 | MSYS2のGCC（`gcc` と `g++`）。WebView2のヘッダはリポジトリに同梱。実行時にWebView2ランタイム |

CUI版（`cmd/gonako`）はcgoを使わないので、上のライブラリが無くてもビルドできます。GUI版だけがこのページの対象です。

Goの版は `go.mod` の指定に従います。Go 1.21以降で `GOTOOLCHAIN=auto`（既定）なら、足りないツールチェインはビルド時に取得されます。

---

## どのOSでも共通のコマンド

リポジトリのルートで実行します。タスクランナーは [just](https://github.com/casey/just) です。

```bash
just gui
```

成果物は `bin/gonako-gui`（Windowsでは `bin/gonako-gui.exe`）です。

`just` が無いときは、同じ処理を行うスクリプトを直接呼べます。Linuxでは、入っているWebKitGTKが 4.0 か 4.1 かをこのスクリプトが選びます。

```bash
./scripts/go-gui.sh build -trimpath -ldflags="-s -w" -o bin/gonako-gui ./cmd/gonako-gui
```

`make` の `build` もGUI版を含み、同じスクリプトを使います。

```bash
make build
```

GUIパッケージのテスト（ウィンドウは開きません）:

```bash
just test-gui
# または
./scripts/go-gui.sh test ./cmd/gonako-gui ./internal/guilib
```

表示のない環境でもビルドとテストは通ります。ウィンドウを目で確認するには、画面のあるマシンで `./bin/gonako-gui` を起動してください。

---

## Linux

### Ubuntu / Debian

```bash
sudo ./scripts/install-gui-deps.sh
just gui
```

`install-gui-deps.sh` は `libwebkit2gtk-4.1-dev` を優先し、パッケージ一覧に無ければ `libwebkit2gtk-4.0-dev` を入れます。手元で入れる場合の対応は次のとおりです。

| ディストリビューション | パッケージ |
|---|---|
| Ubuntu 24.04 / Debian 13 | `gcc` `g++` `pkg-config` `libgtk-3-dev` `libwebkit2gtk-4.1-dev` |
| Ubuntu 22.04 / Debian 12 | 上に同じ。`libwebkit2gtk-4.0-dev` でも `libwebkit2gtk-4.1-dev` でもよい |

Ubuntu 24.04 には WebKitGTK 4.0 の開発パッケージがありません。`webview_go` は既定で `webkit2gtk-4.0` を要求しますが、4.1 しか無いときは `scripts/go-gui.sh` と `just gui` が `-tags webkit2gtk_4_1` を付けます。システムに `webkit2gtk-4.0.pc` のエイリアスを手で置く必要はありません。

4.0 と 4.1 の両方が入っているときは 4.0 を使います（タグなし）。これは従来のビルドと同じです。

確認:

```bash
tag=$(./scripts/webkit-gtk-tag.sh)
echo "build tag: ${tag:-（なし。webkit2gtk-4.0 を使います）}"
pkg-config --modversion gtk+-3.0 webkit2gtk-4.1 || pkg-config --modversion gtk+-3.0 webkit2gtk-4.0
```

4.1 だけの環境では `build tag: webkit2gtk_4_1` と表示されます。

### Fedora

```bash
sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
just gui
```

4.0 系を使う場合は `webkit2gtk4.1-devel` の代わりに `webkit2gtk3-devel`（pkg-config名 `webkit2gtk-4.0`）でも構いません。

### Arch Linux

```bash
sudo pacman -S --needed gcc pkgconf gtk3 webkit2gtk-4.1
just gui
```

### 実行時のライブラリ

ビルドしたバイナリはGTKとWebKitGTKへ動的リンクします。別のLinuxへコピーして動かすときは、そのマシンに実行用パッケージ（開発用の `-dev` ではないもの）が必要です。

| ビルドに使った版 | 実行時のパッケージ例（Ubuntu / Debian） |
|---|---|
| 4.1 | `libgtk-3-0` `libwebkit2gtk-4.1-0` |
| 4.0 | `libgtk-3-0` `libwebkit2gtk-4.0-0` |

開発パッケージを入れたマシンには、実行用ライブラリも一緒に入ります。

### Docker

Ubuntu 24.04 のパッケージだけでビルドするイメージです。リポジトリのルートで実行します。

```bash
docker build -f docker/gonako-gui/Dockerfile -t gonako-gui-builder .
docker create --name gonako-gui-out gonako-gui-builder
docker cp gonako-gui-out:/usr/local/bin/gonako-gui ./bin/gonako-gui
docker rm gonako-gui-out
```

イメージの中のバイナリは、そのイメージに入っている WebKitGTK 4.1 に対してリンクされています。

---

## macOS

Xcodeコマンドラインツールがあれば追加のライブラリは要りません。WebKitはOSのフレームワークです。

```bash
xcode-select --install
just gui
```

`just gui` は macOS では `CGO_LDFLAGS` に `-Wl,-no_warn_duplicate_libraries` を足します（`justfile` の先頭）。Xcode 15以降のリンカが、WebKitとcgoの両方が `-lobjc` を要求することについて出す警告を消すためで、出ても実害はありません。

LinuxやWindowsからmacOS向けの `gonako-gui` はクロスビルドできません。`just release-darwin` も、macOS以外ではGUIをスキップします。AppleのSDKとフレームワークが必要なためです。配布用のmacOS版はmacOS上で作ってください。

Apple Silicon の Mac から Intel（amd64）向けを作るときは、`just release-darwin` が clang に `-arch x86_64` を渡します。逆に Intel の Mac から Apple Silicon 向けを作るときは `-arch arm64` です。ホストと同じアーキテクチャのときは追加のフラグはありません。Intel Mac の GitHub ランナーは無いため、CI では Apple Silicon ランナーから Intel 向けをクロスビルドします。できたバイナリの実行確認は、そのアーキテクチャの Mac で行ってください。

---

## Windows

実機での手順（MSYS2のGCC、PATH、WebView2ランタイム）は [build-gonako-windows.md](build-gonako-windows.md) にあります。短い流れは次のとおりです。

```powershell
winget install GoLang.Go Git.Git Casey.Just MSYS2.MSYS2
```

MSYS2 UCRT64 で `pacman -S --needed mingw-w64-ucrt-x86_64-gcc` を入れ、`C:\msys64\ucrt64\bin` をPATHに通してから:

```powershell
just gui
```

実行には WebView2 ランタイムが必要です。Windows 11 には入っています。Windows 10 では [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) を入れてください。

### LinuxからWindows向けをクロスビルドする

MinGWのクロスコンパイラがあれば、Linux上の `scripts/build-release.go` が Windows向けGUIを作ります。WebKitGTKは使いません。

```bash
sudo apt-get install -y gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64
go run ./scripts/build-release.go -platforms windows/amd64 -skip-cli
```

Ubuntu 24.04 の MinGW には `eventtoken.h`（小文字）しかありません。WebView2のヘッダが取り込む `EventToken.h` は、大文字小文字を区別するファイルシステムでは見つかりません。同梱の `WebView2.h` は MinGW のとき小文字のヘッダを使うので、上のコマンドで `release/gonako-gui-<version>-windows-amd64.zip` ができます。中の実行ファイルは Windows 用の GUI サブシステム（PE32+）です。macOS向けのクロスビルドは、この方法ではできません。

Windows arm64 向けのクロスコンパイラ（`aarch64-w64-mingw32-gcc`）は、Ubuntu 24.04 のパッケージにはありません。MinGW は x86_64 と i686 だけです。`scripts/build-release.go` はコンパイラが見つかれば arm64 もビルドしますが、標準の apt だけでは入りません。

`x86_64-w64-mingw32-gcc` が見つからなければ、そのGUIはスキップされます（失敗にはしません）。arm64 Windows向けは `aarch64-w64-mingw32-gcc` が必要です。

macOSでは `brew install mingw-w64` で同じクロスコンパイラを入れられます。

---

## 配布用アーカイブ

実行中のOS向けのCLIとGUIを `release/` に作るコマンドです。GUIの有無は上の依存関係に従います。

```bash
just release
```

OSを指定する場合は `just release-linux` / `just release-windows` / `just release-darwin` です。Linuxで別アーキテクチャのGUI（amd64ホストからarm64など）を作るには、そのアーキテクチャのCコンパイラに加えて、同じアーキテクチャのGTKとWebKitGTKが必要です。コンパイラが無い場合、そのGUIはスキップされます。

---

## CI

`.github/workflows/gui.yml` が次を実行します。

- Ubuntu 24.04（amd64）で WebKitGTK 4.1 を使い、ビルド・テスト・成果物のアップロード
- Ubuntu 22.04（amd64）で WebKitGTK 4.0 を使い、ビルドとテスト（タグなしの従来経路）
- Ubuntu 24.04 の ARM ランナー（`ubuntu-24.04-arm`）で Linux arm64 をネイティブビルド
- Ubuntu 24.04 から MinGW で Windows amd64 向けをクロスビルドし、zipをアップロード
- macOSランナー（`macos-latest`、Apple Silicon）で arm64 をネイティブビルドし、同じランナーから Intel（amd64）向けをクロスビルド

Linux arm64 を amd64 のマシンからクロスビルドするには、そのアーキテクチャの GTK と WebKitGTK が別途必要です。CI は ARM ランナーでネイティブに作ります。Windows arm64 は、上のとおり Ubuntu 24.04 の MinGW ではコンパイラが無いため CI には入れていません。

通常のテスト（`.github/workflows/test.yml`）は、GTKが無い環境でも通るよう `cmd/gonako-gui` と `internal/guilib` を除外したままです。GUIのテストは `gui.yml` と `just test-gui` が担当します。

---

## うまくビルドできないとき

`Package webkit2gtk-4.0 was not found` と出る場合は、`go build ./cmd/gonako-gui` を直接実行していて、4.1 用のタグが付いていません。`just gui` または `./scripts/go-gui.sh build ...` を使ってください。自分でタグを付ける場合は次です。

```bash
go build -tags webkit2gtk_4_1 -o bin/gonako-gui ./cmd/gonako-gui
```

`gcc` が見つからない、または `CGO_ENABLED=0` のときはcgoが使えません。`gcc` と `g++` をPATHに入れ、`go env CGO_ENABLED` が `1` であることを確認してください。

GTKの非推奨APIについての警告（`gdk_screen_get_monitor_at_window is deprecated` など）は、ビルドを止めません。

---

## 参考

- [build-gonako-windows.md](build-gonako-windows.md): Windows実機の詳しい手順
- [gonako-gui.md](gonako-gui.md): 起動方法と画面の説明
- [release-scripts.md](release-scripts.md): 配布アーカイブの作り方
