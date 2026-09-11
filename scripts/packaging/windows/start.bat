@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM 1. Launch native Windows supervisor via PowerShell launcher
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
        timeout /t 3 >nul 2>&1 || ping -n 4 127.0.0.1 >nul 2>&1
        exit /b 0
    )
)

REM 2. Fallback: Launch native Windows binary sirius-core.exe directly
if exist "%~dp0sirius-core.exe" (
    echo Starting Sirius Core Native on Windows...
    start "" "%~dp0sirius-core.exe" -port 8080 -chainconfig "%~dp0chainconfig"
    echo Dashboard: http://localhost:8080
    exit /b 0
)

echo Error: sirius-core.exe not found!
echo Please run build or download the Windows release package.
pause
exit /b 1
