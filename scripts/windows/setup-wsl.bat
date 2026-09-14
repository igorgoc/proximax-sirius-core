@echo off
setlocal
cd /d "%~dp0"
echo =========================================================================
echo   ProximaX Sirius Mainnet - Windows Subsystem for Linux (WSL2) Setup
echo =========================================================================
powershell.exe -NoExit -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup-wsl.ps1" -Action All
pause
