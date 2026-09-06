@echo off
setlocal
cd /d "%~dp0.."
go run .\cmd\gonako compat run --cases .\testdata\compat\cases --out .\out
