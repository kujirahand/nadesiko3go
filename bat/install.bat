@echo off
setlocal
cd /d "%~dp0.."
go install .\cmd\gonako || exit /b 1
go install .\cmd\gonako-cui
