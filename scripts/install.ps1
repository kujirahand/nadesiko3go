# nadesiko3go (gonako) インストーラー (Windows / PowerShell 用)
# 使い方:
#   irm https://raw.githubusercontent.com/kujirahand/nadesiko3go/main/scripts/install.ps1 | iex

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

$binName = "gonako-$version-windows-amd64.exe"
$url = "https://github.com/$repo/releases/download/$version/$binName"
$targetPath = Join-Path $installDir "gonako.exe"

Write-Host "===> なでしこ3 (gonako v$version) をダウンロード中..." -ForegroundColor Cyan
Write-Host "  URL: $url"
Write-Host "  保存先: $targetPath"

Invoke-WebRequest -Uri $url -OutFile $targetPath

# ユーザー環境変数 PATH の確認と追加
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    $newPath = "$installDir;$userPath"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$installDir;$env:Path"
    Write-Host "===> ユーザー環境変数 PATH に $installDir を追加しました。" -ForegroundColor Green
}

Write-Host "===> インストールが完了しました！" -ForegroundColor Green
Write-Host "===> 動作確認:" -ForegroundColor Cyan
& "$targetPath" -e '「なでしこ3のインストールに成功しました！」と表示。'
