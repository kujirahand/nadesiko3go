@echo off
setlocal
cd /d "%~dp0.."
set VERSION=%1
if "%VERSION%"=="" set VERSION=dev
set PLATFORMS=darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64
go run .\scripts\build-release.go -version %VERSION% -platforms "%PLATFORMS%" -skip-cli
