@echo off
setlocal
cd /d "%~dp0.."
go build -o bin\gonako.exe .\cmd\gonako || exit /b 1
go run .\benchmark\runner.go
