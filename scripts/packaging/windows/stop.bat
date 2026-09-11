@echo off
setlocal
cd /d "%~dp0"

if exist "%~dp0stop-node.ps1" (
    set "PS_SCRIPT=%~dp0stop-node.ps1"
) else if exist "%~dp0scripts\packaging\windows\stop-node.ps1" (
    set "PS_SCRIPT=%~dp0scripts\packaging\windows\stop-node.ps1"
) else (
    echo Error: stop-node.ps1 not found!
    pause
    exit /b 1
)

powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%PS_SCRIPT%" %*
timeout /t 3 >nul 2>&1
