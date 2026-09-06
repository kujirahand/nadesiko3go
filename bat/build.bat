@echo off
setlocal
cd /d "%~dp0.."
go build -o bin\gonako.exe .\cmd\gonako || exit /b 1
go build -o bin\gonako-cui.exe .\cmd\gonako-cui || exit /b 1
go build -o bin\gonako-gui.exe .\cmd\gonako-gui || exit /b 1
