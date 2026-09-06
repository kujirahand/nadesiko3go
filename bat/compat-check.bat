@echo off
setlocal
cd /d "%~dp0.."
if not exist nadesiko3 (
    echo .\nadesiko3 が見つかりません。本家リポジトリを clone してください。
    exit /b 1
)
pushd nadesiko3
call npm run compat:check -- ..\out
popd
