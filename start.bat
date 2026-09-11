@echo off
setlocal
cd /d "%~dp0"

if exist "%~dp0start-node.ps1" (
    set "PS_SCRIPT=%~dp0start-node.ps1"
) else if exist "%~dp0scripts\packaging\windows\start-node.ps1" (
    set "PS_SCRIPT=%~dp0scripts\packaging\windows\start-node.ps1"
) else (
    echo Error: start-node.ps1 not found!
    pause
    exit /b 1
)

powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%PS_SCRIPT%" %*
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Node manager encountered an error.
    pause
) else (
    timeout /t 5 >nul 2>&1
)
