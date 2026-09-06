@echo off
setlocal
cd /d "%~dp0.."
go test ./...
