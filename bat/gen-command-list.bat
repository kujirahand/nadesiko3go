@echo off
setlocal
cd /d "%~dp0.."
go run .\scripts\gen-command-list.go
