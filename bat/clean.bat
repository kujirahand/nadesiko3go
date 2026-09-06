@echo off
setlocal
cd /d "%~dp0.."
if exist bin rmdir /s /q bin
if exist out rmdir /s /q out
if exist benchmark\build rmdir /s /q benchmark\build
