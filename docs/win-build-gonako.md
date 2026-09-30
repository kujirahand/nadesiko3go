# gonako/gonako-guiをWindowsでビルドする手順

Windows実機でgonako（CUI版）およびgonako-gui（GUI版）をビルドする際の手順をまとめたものです。

CUI版はcgoを使わないので、macOS/Linuxから `GOOS=windows` のクロスビルドでも作れます。
一方、GUI版はcgo（WebView2）を使うため、クロスビルドするにはMinGWのクロスコンパイラ
（`x86_64-w64-mingw32-gcc` など）が必要です。**Windows実機でビルドするのが最も手間がありません**。
このドキュメントはWindows実機でのビルドを前提にしています。

---

## 前提条件

- Windows 10（1809以降）または Windows 11
- PowerShell（管理者権限不要、一般ユーザーで可）

---

## 1. Go言語のインストール

wingetを使ってGoをインストールします。

```powershell
winget install GoLang.Go
```

インストールが完了したら、**ターミナルを一度閉じて開き直す**（PATHの反映のため）か、次のコマンドでPATHを更新します。

```powershell
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
```

インストールを確認します。

```powershell
go version
```

次のように表示されればOKです（バージョン番号は環境によって異なります）。

```text
go version go1.26.x windows/amd64
```

> **注意**: `go.mod` は `go 1.26.0` 以上を要求しています。古いバージョンがインストールされている場合は `winget upgrade GoLang.Go` でアップグレードしてください。

---

## 2. Git for Windows のインストール

リポジトリのクローンと、後の `just` の動作に必要な `sh.exe` を得るために Git for Windows をインストールします。

```powershell
winget install Git.Git
```

インストール先はデフォルトの `C:\Program Files\Git\` です。このパスに `bin\sh.exe` が含まれています。

---

## 3. just のインストール

`justfile` のタスクを実行するために `just` をインストールします。

```powershell
winget install Casey.Just
```

インストール後、ターミナルを開き直して確認します。

```powershell
just --version
```

```text
just 1.58.0
```

### 3.1 justのPATH競合に注意

Chocolateyで `just` をインストール済みだと次の2つの `just.exe` が見つかります。

```powershell
where.exe just
```

- `C:\ProgramData\chocolatey\bin\just.exe`
- `C:\Users\<USERNAME>\AppData\Local\Microsoft\WinGet\Links\just.exe`

PATHのどちらか一方だけが使える状態に整理してください。通常はChocolatey版をアンインストールするか、PATHの順序でWinGet版を先にします。

### 3.2 PATHの一時追加

Git for Windowsの `bin` がPATHに入っていない場合、手動で追加します。

```powershell
$env:PATH += ";C:\Program Files\Git\bin"
```

---

## 4. MSYS2（GCC）のインストール

GUI版（`gonako-gui`）はcgo経由でWebView2を使うため、CコンパイラとしてGCCが必要です。MSYS2のMINGW64環境からインストールします。

```powershell
winget install MSYS2.MSYS2
```

インストール後、`MSYS2 UCRT64` ショートカットを開いて、GCCをインストールします。

```bash
pacman -S --needed mingw-w64-ucrt-x86_64-gcc
```

インストールされたGCCをPATHに通します。次のいずれかの方法で設定します。

### 方法A: システムのPATHに追加

**コントロールパネル → システム → 詳細設定 → 環境変数** で、`Path` に次を追加します。

```text
C:\msys64\ucrt64\bin
```

### 方法B: PowerShellのプロファイルに追加

```powershell
notepad $PROFILE
```

末尾に次を追加して保存します。

```powershell
$env:PATH += ";C:\msys64\ucrt64\bin"
```

### GCCのインストールを確認

```powershell
where.exe gcc
```

```text
C:\msys64\ucrt64\bin\gcc.exe
```

### GoがGCCを見付ける場所

```powershell
go env CC
```

```text
gcc
```

でOKです（PATH上に `gcc` が見つかればよい）。

---

## 5. CUI版（`gonako`）のビルド

CUI版はcgoに依存しないため、GCCなしでもビルドできます。

```powershell
git clone https://github.com/kujirahand/nadesiko3go.git
cd nadesiko3go
just cmd
```

ビルド成果物は `bin\gonako.exe` です。

---

## 6. GUI版（`gonako-gui`）のビルド

GUI版はWebView2を使うためMSYS2のGCCがPATHに通っている必要があります。

```powershell
just gui
```

ビルド成果物は `bin\gonako-gui.exe` です。

### 6.1 WebView2ランタイム

WebView2（Edge）ランタイムはWindows 11以降であれば同梱されています。
Windows 10では、[Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) をインストールするか、Edgeブラウザが起動できる状態にしてください。

---

## 7. すべてのビルド（CUI + GUI）

```powershell
just build
```

`just build` は `just cmd` と `just gui` の両方を実行します。

---

## 8. よくあるエラーと対処法

### 8.1 `sh` が見つからない

```text
error: recipe `default` could not be run because just could not find the shell `sh`: program not found
```

**原因**: Git for Windowsの `sh.exe` がPATHに入っていない。

**対処**:

```powershell
$env:PATH += ";C:\Program Files\Git\bin"
```

`where.exe sh` で見つかることを確認してください。

### 8.2 cgoでビルドエラー

```text
# runtime/cgo
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%
```

**原因**: MSYS2のGCCがPATHにない。

**対処**:

```powershell
$env:PATH += ";C:\msys64\ucrt64\bin"
```

### 8.3 認証エラー（プロキシ環境）

社内プロキシ環境ではGoがモジュールのダウンロードに失敗することがあります。

```powershell
go env -w HTTP_PROXY=http://<プロキシ>:<ポート>
go env -w HTTPS_PROXY=http://<プロキシ>:<ポート>
go env -w GOPROXY=https://proxy.golang.org,direct
```

### 8.4 長時間のビルドタイム（初回）

初回ビルドはGoの依存パッケージをダウンロードするため数分かかります。以降は高速になります。

### 8.5 ビルド後のexeが実行できない（GUI版）

WebView2ランタイムが無いWindows 10環境では、GUI版が起動できないことがあります。ランタイムのインストールを確認してください。

### 8.6 互換テストを実行したい

```powershell
just test
```

差分fixtureによる互換性チェックは本家リポジトリが必要になります。`nadesiko3/` を `/nadesiko3go` 直下にcloneして、次のコマンドを実行してください。

```powershell
just sync-compat
just compat-run
```

---

## 9. 配布用アーカイブを作る / 開発中のGUIをすぐ試す

Windows実機で配布用（`release/` にzipを作る）には次を使います。

```powershell
just release-windows
```

ビルドせずにGUI版を起動して確認するには:

```powershell
just run-gui
```

---

## 10. 補足：macOS/LinuxからWindows向けCUI版をクロスビルドする

CUI版はcgoに依存しないので、Windows実機が無くても作れます。

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/gonako-windows-amd64.exe ./cmd/gonako
```

GUI版を他のOSからクロスビルドする場合は、MinGWのクロスコンパイラ
（`x86_64-w64-mingw32-gcc` / `aarch64-w64-mingw32-g++`）が必要です。
`scripts/build-release.go` はクロスコンパイラが見つからなければGUIをスキップします。
macOSでは `brew install mingw-w64` で入ります。

---

## 11. 検証環境

このドキュメントの手順は、Windows 11 の実機で次の状態を確認しています。

- just 1.58.0（WinGet版。Chocolatey版と競合していたため整理）
- Git for Windows（`sh.exe` は `C:\Program Files\Git\bin` に存在）
- `just build` が正常に完了することを確認

Go・MSYS2・GCCのバージョンは環境によります。実際のバージョンは次で確認してください。

```powershell
go version; gcc --version; git --version; just --version
```

> **注**: `justfile` はUnix系の `sh` を前提としています。Windowsで使う場合、Git for Windowsの `sh` とMSYS2のGCCがPATHに通っていることが必要です。

---

## 参考リンク

- [Go公式ダウンロード](https://go.dev/dl/)
- [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)
- [just チートシート](https://github.com/casey/just)