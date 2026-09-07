<#
.SYNOPSIS
    ProximaX Sirius Mainnet Peer Node Launcher for Windows (Native)
.DESCRIPTION
    Starts the native node supervisor and cockpit backend on port 3080.
    Enforces Windows NTFS ACL hardening (0600 equivalent) on sensitive keys.
#>

[CmdletBinding()]
param(
    [switch]$Foreground,
    [string]$DataPath = ""
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $ScriptDir

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  ProximaX Sirius Mainnet Peer Node (Windows Native)      " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

# 1. Add bin/ to PATH for native DLL loading
$BinDir = Join-Path $ScriptDir "bin"
if (Test-Path $BinDir) {
    $env:PATH = "$BinDir;$env:PATH"
    Write-Host "-> Configured DLL search path: $BinDir" -ForegroundColor Green
}

# 2. NTFS ACL Security Hardening (POSIX 0600 Equivalent)
# Removes inherited permissions and restricts read/write to current user and SYSTEM
$ResourcesDir = Join-Path $ScriptDir "chainconfig\resources"
$SensitiveFiles = @(
    (Join-Path $ResourcesDir "config-harvesting.properties"),
    (Join-Path $ResourcesDir "config-user.properties"),
    (Join-Path $ResourcesDir ".sirius-token")
)

foreach ($file in $SensitiveFiles) {
    if (Test-Path $file) {
        try {
            icacls "$file" /inheritance:r /grant:r "$($env:USERNAME):(R,W)" /grant:r "SYSTEM:(R,W)" | Out-Null
            Write-Host "-> Secured NTFS ACL (0600-equivalent): $(Split-Path -Leaf $file)" -ForegroundColor Gray
        } catch {
            Write-Warning "Could not apply NTFS ACL to $file"
        }
    }
}

# 3. Clean up stale lock files
$LockFile = Join-Path $ScriptDir "chainconfig\data\server.lock"
if (Test-Path $LockFile) {
    Remove-Item -Force $LockFile -ErrorAction SilentlyContinue
    Write-Host "-> Cleared stale server.lock" -ForegroundColor Gray
}

# 4. Check if already running
$PidFile = Join-Path $ScriptDir ".sirius-core.pid"
if (Test-Path $PidFile) {
    $ExistingPid = (Get-Content $PidFile -ErrorAction SilentlyContinue | Out-String).Trim()
    if ($ExistingPid) {
        $Proc = Get-Process -Id $ExistingPid -ErrorAction SilentlyContinue
        if ($Proc) {
            Write-Host "Sirius Core Native is already running (PID: $ExistingPid)." -ForegroundColor Yellow
            Write-Host "Dashboard: http://localhost:3080" -ForegroundColor Cyan
            exit 0
        }
    }
}

# 5. Launch Node Manager
$BackendExe = Join-Path $ScriptDir "sirius-core.exe"
if (-not (Test-Path $BackendExe)) {
    Write-Error "Binary not found: $BackendExe"
    exit 1
}

$LogDir = Join-Path $ScriptDir "chainconfig\logs"
if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir | Out-Null }
$LogFile = Join-Path $LogDir "manager.log"

if ($Foreground) {
    Write-Host "-> Starting node manager in foreground..." -ForegroundColor Green
    & $BackendExe
} else {
    Write-Host "-> Starting node manager in background..." -ForegroundColor Green
    $Process = Start-Process -FilePath $BackendExe -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile -PassThru -WindowStyle Hidden
    $Process.Id | Out-File -FilePath $PidFile -Encoding ascii
    Start-Sleep -Seconds 2

    Write-Host ""
    Write-Host "=========================================================" -ForegroundColor Green
    Write-Host "  ProximaX Sirius Native Node is ONLINE (PID: $($Process.Id))!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Access GUI Cockpit Dashboard:" -ForegroundColor White
    Write-Host "    >>> http://localhost:3080 <<<" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Logs: Get-Content -Wait $LogFile" -ForegroundColor Gray
    Write-Host "  Stop: .\stop-node.ps1" -ForegroundColor Gray
    Write-Host "=========================================================" -ForegroundColor Green
}
