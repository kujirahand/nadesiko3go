# gonako/gonako-guiをWindowsでビルドする手順

## Go言語のインストール


## justのインストール

- `winget install Casey.Just` を試す。
- `where.exe just` で確認すると、次の2つが見つかった。
  - `C:\ProgramData\chocolatey\bin\just.exe`
  - `C:\Users\kujirahand\AppData\Local\Microsoft\WinGet\Links\just.exe`
- PATH 上で Chocolatey 版が先に見つかっていたため、WinGet 版を使える状態に整理した。
- その結果、`just --version` が正常に動作し、`just 1.58.0` と表示されるようになった。
- 次に `just` を実行すると、以下のエラーが出た。
  ```text
  error: recipe `default` could not be run because just could not find the shell `sh`: program not found
  ```
- 原因は、`justfile` が Unix 系の `sh` を前提としている一方、PowerShell からは `sh.exe` が見つからなかったため。
- Git for Windows がすでにインストールされていることを確認した。
- `C:\Program Files\Git\cmd\` には `git.exe` はあったが、`sh.exe` はなかった。
- Git for Windows の `sh.exe` がある `C:\Program Files\Git\bin` を PATH に追加した。
  ```powershell
  $env:PATH += ";C:\Program Files\Git\bin"
  ```
- `where.exe sh` などで `sh` が使える状態を確認。
- 最終的に、
  ```powershell
  just build
  ```
  が正常に実行できるようになった。

要するに、今回の問題は **「just の実行ファイルの競合」→「sh が PATH にない」** という2段階の問題でした。