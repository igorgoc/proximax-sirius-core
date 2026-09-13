@echo off
setlocal
cd /d "%~dp0"

echo =========================================================
echo   Stopping ProximaX Sirius Core...
echo =========================================================

REM 1. Stop native Windows processes and WSL engine gracefully via PowerShell stopper
set "PS_SCRIPT="
if exist "%~dp0stop-node.ps1" (
    set "PS_SCRIPT=%~dp0stop-node.ps1"
) else if exist "%~dp0scripts\packaging\windows\stop-node.ps1" (
    set "PS_SCRIPT=%~dp0scripts\packaging\windows\stop-node.ps1"
)
if defined PS_SCRIPT (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%PS_SCRIPT%" %*
    set "EXIT_CODE=%ERRORLEVEL%"
    exit /b %EXIT_CODE%
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
