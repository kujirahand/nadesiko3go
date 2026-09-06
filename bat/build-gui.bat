@echo off
setlocal
cd /d "%~dp0.."
go build -o bin\gonako-gui.exe .\cmd\gonako-gui
