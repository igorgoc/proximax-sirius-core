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
echo   Stopping ProximaX Sirius Core...
echo =========================================================

REM 1. Stop native Windows processes and WSL engine gracefully via PowerShell stopper
set "PS_SCRIPT="
if exist "%~dp0stop-node.ps1" (
    set "PS_SCRIPT=%~dp0stop-node.ps1"
) else if exist "!ROOT_DIR!\scripts\windows\stop-node.ps1" (
    set "PS_SCRIPT=!ROOT_DIR!\scripts\windows\stop-node.ps1"
)
if defined PS_SCRIPT (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "!PS_SCRIPT!" %*
    set "EXIT_CODE=!ERRORLEVEL!"
    exit /b !EXIT_CODE!
)

REM 2. Fallback only if PowerShell was unavailable: Gracefully signal WSL engine with SIGINT
where wsl.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Sending graceful SIGINT to Sirius engine in WSL...
    wsl.exe -u root -- pkill -INT -f sirius.bc >nul 2>&1
    wsl.exe -u root -- sync >nul 2>&1
)

echo ProximaX Sirius Node is stopped.
exit /b 0
