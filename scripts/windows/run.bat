@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM 0. Detect project root directory
set "ROOT_DIR=%~dp0"
if exist "%~dp0..\..\chainconfig\resources" (
    pushd "%~dp0..\.."
    set "ROOT_DIR=!CD!"
    popd
)

echo =========================================================
echo   ProximaX Sirius Mainnet Peer Node (Windows Native)
echo =========================================================

REM Auto-detect standard Go and Node.js install paths if not yet in current session PATH
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    if exist "%ProgramFiles%\Go\bin\go.exe" (
        set "PATH=%ProgramFiles%\Go\bin;!PATH!"
    ) else if exist "C:\Go\bin\go.exe" (
        set "PATH=C:\Go\bin;!PATH!"
    )
)

where node >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    if exist "%ProgramFiles%\nodejs\node.exe" (
        set "PATH=%ProgramFiles%\nodejs;!PATH!"
    )
)

REM 1. If in a pre-built release package (no frontend/backend source directories), delegate to start.bat
if not exist "!ROOT_DIR!\frontend" (
    if exist "%~dp0start.bat" (
        call "%~dp0start.bat" %*
        exit /b %ERRORLEVEL%
    ) else if exist "!ROOT_DIR!\start.bat" (
        call "!ROOT_DIR!\start.bat" %*
        exit /b %ERRORLEVEL%
    )
)

REM 2. Source repo build workflow: build React UI if not built
if not exist "!ROOT_DIR!\backend\dist\index.html" (
    if exist "!ROOT_DIR!\frontend" (
        if not exist "!ROOT_DIR!\frontend\node_modules" (
            echo -^> Installing frontend dependencies...
            cd /d "!ROOT_DIR!\frontend"
            call npm ci
            cd /d "%~dp0"
        )
        echo -^> Building React UI...
        cd /d "!ROOT_DIR!\frontend"
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
echo -^> Compiling native Go backend (sirius-core.exe)...
cd /d "!ROOT_DIR!\backend"
go build -ldflags="-s -w" -o "!ROOT_DIR!\sirius-core.exe" .
if %ERRORLEVEL% NEQ 0 (
    echo Failed to build sirius-core.exe!
    pause
    exit /b 1
)
cd /d "%~dp0"

REM 4. Launch Node Manager
echo -^> Launching Node Manager...
if exist "%~dp0start.bat" (
    call "%~dp0start.bat" %*
) else if exist "%~dp0start-node.ps1" (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "%~dp0start-node.ps1" %*
) else if exist "!ROOT_DIR!\scripts\windows\start-node.ps1" (
    powershell.exe -ExecutionPolicy Bypass -NoProfile -File "!ROOT_DIR!\scripts\windows\start-node.ps1" %*
)
