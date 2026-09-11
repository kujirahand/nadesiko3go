# nadesiko3go (gonako & gonako-gui) インストーラー (Windows / PowerShell 用)
# 使い方:
#   irm https://nadesi.com/install/gonako | iex
# または:
#   irm https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "kujirahand/nadesiko3go"
$defaultVersion = "3.8.4"

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
$targetPath = Join-Path $installDir "gonako.exe"

function Download-And-Extract-CLI ($ver) {
    Write-Host "===> [1/2] なでしこ3 CLI版 (gonako v$ver) をダウンロード中..." -ForegroundColor Cyan
    $cliZipName = "gonako-$ver-windows-amd64.zip"
    $cliUrl = "https://github.com/$repo/releases/download/$ver/$cliZipName"
    $tmpCliZip = Join-Path $env:TEMP "gonako-cli.zip"
    try {
        Invoke-WebRequest -Uri $cliUrl -OutFile $tmpCliZip -UseBasicParsing
        Expand-Archive -Path $tmpCliZip -DestinationPath $installDir -Force
        Remove-Item $tmpCliZip -Force -ErrorAction SilentlyContinue
        Write-Host "  -> CLI版の保存完了: $targetPath" -ForegroundColor Green
        return $true
    } catch {
        Remove-Item $tmpCliZip -Force -ErrorAction SilentlyContinue
        return $false
    }
}

$cliSuccess = Download-And-Extract-CLI $version
if (-not $cliSuccess) {
    if ($version -ne $defaultVersion) {
        Write-Host "  [再試行] v$version のダウンロードに失敗したため、安定版 v$defaultVersion を試みます..." -ForegroundColor Yellow
        $cliSuccess = Download-And-Extract-CLI $defaultVersion
        if ($cliSuccess) {
            $version = $defaultVersion
        } else {
            Write-Error "CLI版のダウンロードに失敗しました"
            exit 1
        }
    } else {
        Write-Error "CLI版のダウンロードに失敗しました"
        exit 1
    }
}

# ----------------------------------------------------
# 2. GUI版 (gonako-gui.exe) のインストール
# ----------------------------------------------------
function Download-And-Extract-GUI ($ver) {
    Write-Host "===> [2/2] なでしこ3 GUI版 (gonako-gui v$ver) をダウンロード中..." -ForegroundColor Cyan
    $guiZipName = "gonako-gui-$ver-windows-amd64.zip"
    $guiUrl = "https://github.com/$repo/releases/download/$ver/$guiZipName"
    $tmpZip = Join-Path $env:TEMP "gonako-gui.zip"

    try {
        Invoke-WebRequest -Uri $guiUrl -OutFile $tmpZip -UseBasicParsing
        Expand-Archive -Path $tmpZip -DestinationPath $installDir -Force
        Remove-Item $tmpZip -Force -ErrorAction SilentlyContinue
        $guiTargetPath = Join-Path $installDir "gonako-gui.exe"
        Write-Host "  -> GUI版の保存完了: $guiTargetPath" -ForegroundColor Green

        # デスクトップにショートカットを作成
        $wshShell = New-Object -ComObject WScript.Shell
        $desktopDir = [Environment]::GetFolderPath("Desktop")
        $shortcutPath = Join-Path $desktopDir "なでしこ3 - gonako.lnk"
        $shortcut = $wshShell.CreateShortcut($shortcutPath)
        $shortcut.TargetPath = $guiTargetPath
        $shortcut.Description = "なでしこ3 GUIエディタ"
        $shortcut.Save()
        Write-Host "  -> デスクトップにショートカットを作成しました: $shortcutPath" -ForegroundColor Green
        return $true
    } catch {
        Remove-Item $tmpZip -Force -ErrorAction SilentlyContinue
        return $false
    }
}

$guiSuccess = Download-And-Extract-GUI $version
if (-not $guiSuccess) {
    if ($version -ne $defaultVersion) {
        Write-Host "  [再試行] GUI版 v$version のダウンロードに失敗したため、安定版 v$defaultVersion を試みます..." -ForegroundColor Yellow
        $guiSuccess = Download-And-Extract-GUI $defaultVersion
        if (-not $guiSuccess) {
            Write-Warning "GUI版のダウンロードに失敗しました"
        }
    } else {
        Write-Warning "GUI版のダウンロードに失敗しました"
    }
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
