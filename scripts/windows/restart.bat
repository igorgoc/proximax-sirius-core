@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM 0. Detect project/package root directory
set "ROOT_DIR=%~dp0"
if exist "%~dp0..\..\chainconfig\resources" (
    pushd "%~dp0..\.."
    set "ROOT_DIR=!CD!"
    popd
)

echo =========================================================
echo   Restarting ProximaX Sirius Core...
echo =========================================================

REM 1. Launch native Windows supervisor via PowerShell launcher
set "PS_SCRIPT="
if exist "%~dp0restart-node.ps1" (
    set "PS_SCRIPT=%~dp0restart-node.ps1"
) else if exist "!ROOT_DIR!\scripts\windows\restart-node.ps1" (
    set "PS_SCRIPT=!ROOT_DIR!\scripts\windows\restart-node.ps1"
)

if defined PS_SCRIPT (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "!PS_SCRIPT!" %*
    set "EXIT_CODE=!ERRORLEVEL!"
    timeout /t 5 >nul 2>&1 || ping -n 6 127.0.0.1 >nul 2>&1
    exit /b !EXIT_CODE!
)

REM 2. Restart via WSL
where wsl.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    wsl.exe --cd "!ROOT_DIR!" -u root -- ./scripts/linux/restart.sh %*
    exit /b !ERRORLEVEL!
)

echo Error: Neither native sirius.exe nor WSL found!
exit /b 1
