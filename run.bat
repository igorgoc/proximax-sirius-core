@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo =========================================================
echo   ProximaX Sirius Mainnet Peer Node (Windows Native)
echo =========================================================

REM 1. If in a pre-built release package (no frontend/backend source directories), delegate to start.bat
if not exist "%~dp0frontend" (
    if exist "%~dp0start.bat" (
        call "%~dp0start.bat" %*
        exit /b %ERRORLEVEL%
    )
)

REM 2. Source repo build workflow: build React UI if not built
if not exist "%~dp0backend\dist\index.html" (
    if exist "%~dp0frontend" (
        echo -> Building React UI...
        cd /d "%~dp0frontend"
        call npm run build
        if %ERRORLEVEL% NEQ 0 (
            echo Failed to build React UI!
            pause
            exit /b 1
        )
        cd /d "%~dp0"
    )
)

REM 3. Compile Go backend for Windows
echo -> Compiling native Go backend (sirius-core.exe)...
cd /d "%~dp0backend"
go build -ldflags="-s -w" -o "%~dp0sirius-core.exe" .
if %ERRORLEVEL% NEQ 0 (
    echo Failed to build sirius-core.exe!
    pause
    exit /b 1
)
cd /d "%~dp0"

REM 4. Launch Node Manager
echo -> Launching Node Manager...
if exist "%~dp0start.bat" (
    call "%~dp0start.bat" %*
) else if exist "%~dp0scripts\packaging\windows\start-node.ps1" (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%~dp0scripts\packaging\windows\start-node.ps1" %*
)
