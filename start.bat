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
    if not exist "%~dp0start.sh" (
        echo -^> Setting up WSL runner scripts...
        (
            echo #!/bin/bash
            echo set -e
            echo DIR="$^( cd "$^( dirname "${BASH_SOURCE[0]}" ^)" ^&^& pwd ^)"
            echo cd "$DIR"
            echo if [ ! -f "$DIR/bin/librocksdb.so.8" ] ^|^| [ ! -f "$DIR/bin/sirius.bc" ]; then
            echo   echo "-^> Downloading precompiled Sirius Linux binaries..."
            echo   mkdir -p "$DIR/bin"
            echo   curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-linux-amd64.tar.gz" ^| tar -xz -C "$DIR"
            echo fi
            echo chmod +x "$DIR/sirius-core" "$DIR"/*.sh 2^>/dev/null ^|^| true
            echo [ -f "$DIR/bin/sirius.bc" ] ^&^& chmod +x "$DIR/bin/sirius.bc" 2^>/dev/null ^|^| true
            echo exec "$DIR/sirius-core" -port 8080 -chainconfig "$DIR/chainconfig"
        ) > "%~dp0start.sh"
        wsl.exe --cd "%~dp0" -u root -- sed -i "s/\r$//" ./start.sh 2>nul
    )
    wsl.exe --cd "%~dp0" -u root -- ./start.sh %*
    exit /b %ERRORLEVEL%
)

echo Error: Neither native sirius.exe nor WSL found!
echo The Sirius Catapult engine runs inside WSL on Windows.
echo Please install WSL with:
echo   wsl --install -d Ubuntu-22.04
pause
exit /b 1
