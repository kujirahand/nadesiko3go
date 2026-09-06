@echo off
setlocal
cd /d "%~dp0.."
where bash >nul 2>nul
if errorlevel 1 (
    echo bash が見つかりません。Git for Windows などに含まれる bash を PATH に通してください。
    exit /b 1
)
bash ./scripts/sync-compat-fixtures.sh
