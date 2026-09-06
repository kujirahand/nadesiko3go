@echo off
setlocal
cd /d "%~dp0.."
go build -o bin\gonako.exe .\cmd\gonako
