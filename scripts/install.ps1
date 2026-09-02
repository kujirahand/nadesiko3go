# nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)
# 使い方:
#   irm https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "kujirahand/nadesiko3go"
$defaultVersion = "3.8.1"

# バージョンの決定
$version = $env:GONAKO_VERSION
if (-not $version) {
    try {
        $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -TimeoutSec 5
        if ($latest.tag_name) {
            $version = $latest.tag_name.TrimStart("v")
        }
    } catch {
        $version = $defaultVersion
    }
}
if (-not $version) {
    $version = $defaultVersion
}

# インストール先フォルダ
$installDir = $env:GONAKO_INSTALL_DIR
if (-not $installDir) {
    $installDir = Join-Path $HOME ".gonako\bin"
}

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# ----------------------------------------------------
# 1. CLI版 (gonako.exe) のインストール
# ----------------------------------------------------
Write-Host "===> [1/2] なでしこ3 CLI版 (gonako v$version) をダウンロード中..." -ForegroundColor Cyan
$binName = "gonako-$version-windows-amd64.exe"
$url = "https://github.com/$repo/releases/download/$version/$binName"
$targetPath = Join-Path $installDir "gonako.exe"

try {
    Invoke-WebRequest -Uri $url -OutFile $targetPath
    Write-Host "  -> CLI版の保存完了: $targetPath" -ForegroundColor Green
} catch {
    Write-Warning "CLI版のダウンロードに失敗しました: $_"
}

# ----------------------------------------------------
# 2. GUI版 (gonako-gui.exe) のインストール
# ----------------------------------------------------
Write-Host "===> [2/2] なでしこ3 GUI版 (gonako-gui v$version) をダウンロード中..." -ForegroundColor Cyan
$guiZipName = "gonako-gui-$version-windows-amd64.zip"
$guiUrl = "https://github.com/$repo/releases/download/$version/$guiZipName"
$tmpZip = Join-Path $env:TEMP "gonako-gui.zip"

try {
    Invoke-WebRequest -Uri $guiUrl -OutFile $tmpZip
    Expand-Archive -Path $tmpZip -DestinationPath $installDir -Force
    Remove-Item $tmpZip -Force -ErrorAction SilentlyContinue
    $guiTargetPath = Join-Path $installDir "gonako-gui.exe"
    Write-Host "  -> GUI版の保存完了: $guiTargetPath" -ForegroundColor Green

    # デスクトップにショートカットを作成
    $wshShell = New-Object -ComObject WScript.Shell
    $desktopDir = [Environment]::GetFolderPath("Desktop")
    $shortcutPath = Join-Path $desktopDir "なでしこ3 (gonako-gui).lnk"
    $shortcut = $wshShell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $guiTargetPath
    $shortcut.Description = "なでしこ3 GUIエディタ"
    $shortcut.Save()
    Write-Host "  -> デスクトップにショートカットを作成しました: $shortcutPath" -ForegroundColor Green
} catch {
    Write-Warning "GUI版のダウンロードに失敗しました: $_"
}

# ----------------------------------------------------
# ユーザー環境変数 PATH の確認と追加
# ----------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    $newPath = "$installDir;$userPath"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$installDir;$env:Path"
    Write-Host "===> ユーザー環境変数 PATH に $installDir を追加しました。" -ForegroundColor Green
}

Write-Host "===> インストールが完了しました！" -ForegroundColor Green
Write-Host "===> 動作確認:" -ForegroundColor Cyan
if (Test-Path $targetPath) {
    & "$targetPath" -e '「CLI版 (gonako): こんにちは！」と表示。'
}
