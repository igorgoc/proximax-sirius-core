@echo off
setlocal
cd /d "%~dp0"

echo =========================================================
echo   Stopping ProximaX Sirius Core...
echo =========================================================

REM 1. Stop native Windows processes if any
set "PS_SCRIPT="
if exist "%~dp0stop-node.ps1" (
    set "PS_SCRIPT=%~dp0stop-node.ps1"
) else if exist "%~dp0scripts\packaging\windows\stop-node.ps1" (
    set "PS_SCRIPT=%~dp0scripts\packaging\windows\stop-node.ps1"
)
if defined PS_SCRIPT (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%PS_SCRIPT%" %* >nul 2>&1
)

REM 2. Stop WSL processes if WSL is active
where wsl.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    wsl.exe --cd "%~dp0" -u root -- ./stop.sh %* >nul 2>&1
)

echo ProximaX Sirius Node is stopped.
exit /b 0
