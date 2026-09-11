@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM 1. If native Windows engine (bin\sirius.exe) is present, launch natively via start-node.ps1
if exist "%~dp0bin\sirius.exe" (
    set "PS_SCRIPT="
    if exist "%~dp0start-node.ps1" (
        set "PS_SCRIPT=%~dp0start-node.ps1"
    ) else if exist "%~dp0scripts\packaging\windows\start-node.ps1" (
        set "PS_SCRIPT=%~dp0scripts\packaging\windows\start-node.ps1"
    )
    if defined PS_SCRIPT (
        powershell.exe -ExecutionPolicy Bypass -NoProfile -File "!PS_SCRIPT!" %*
        if !ERRORLEVEL! NEQ 0 (
            echo.
            echo Node manager encountered an error.
            pause
            exit /b !ERRORLEVEL!
        ) else (
            timeout /t 5 >nul 2>&1 || ping -n 6 127.0.0.1 >nul 2>&1
            exit /b 0
        )
    )
)

REM 2. For Windows without native C++ engine: run Catapult engine via WSL
where wsl.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo =========================================================
    echo   ProximaX Sirius Core (Windows Host - Engine in WSL)
    echo   Dashboard: http://localhost:8080
    echo =========================================================
    wsl.exe --cd "%~dp0" -u root -- ./start.sh %*
    exit /b %ERRORLEVEL%
)

echo Error: Neither native sirius.exe nor WSL found!
echo The Sirius Catapult engine runs inside WSL on Windows.
echo Please install WSL with:
echo   wsl --install -d Ubuntu-22.04
pause
exit /b 1
