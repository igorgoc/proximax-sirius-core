@echo off
setlocal
cd /d "%~dp0"

echo =========================================================
echo   Restarting ProximaX Sirius Core...
echo =========================================================

REM 1. If native Windows engine is present, restart natively
if exist "%~dp0bin\sirius.exe" (
    set "PS_SCRIPT="
    if exist "%~dp0restart-node.ps1" (
        set "PS_SCRIPT=%~dp0restart-node.ps1"
    ) else if exist "%~dp0scripts\packaging\windows\restart-node.ps1" (
        set "PS_SCRIPT=%~dp0scripts\packaging\windows\restart-node.ps1"
    )
    if defined PS_SCRIPT (
        powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%PS_SCRIPT%" %*
        set "EXIT_CODE=%ERRORLEVEL%"
        timeout /t 5 >nul 2>&1 || ping -n 6 127.0.0.1 >nul 2>&1
        exit /b %EXIT_CODE%
    )
)

REM 2. Restart via WSL
where wsl.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    wsl.exe --cd "%~dp0" -u root -- ./restart.sh %*
    exit /b %ERRORLEVEL%
)

echo Error: Neither native sirius.exe nor WSL found!
exit /b 1
