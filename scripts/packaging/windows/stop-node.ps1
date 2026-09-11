<#
.SYNOPSIS
    ProximaX Sirius Mainnet Peer Node Stopper for Windows
#>

[CmdletBinding()]
param(
    [int]$Port = 8080
)

$ErrorActionPreference = "SilentlyContinue"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

# Automatically detect root directory
if (Test-Path (Join-Path $ScriptDir "..\..\..\chainconfig\resources")) {
    $RootDir = (Resolve-Path (Join-Path $ScriptDir "..\..\..")).Path
} elseif (Test-Path (Join-Path $ScriptDir "..\..\chainconfig\resources")) {
    $RootDir = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
} elseif (Test-Path (Join-Path $ScriptDir "chainconfig\resources")) {
    $RootDir = (Resolve-Path $ScriptDir).Path
} else {
    $RootDir = $ScriptDir
}
Set-Location $RootDir

$PidFile = Join-Path $RootDir ".sirius-core.pid"
$Stopped = $false

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  Stopping ProximaX Sirius Native Node...                 " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

# 1. Stop via PID if file exists
if (Test-Path $PidFile) {
    $PidNum = (Get-Content $PidFile -ErrorAction SilentlyContinue | Out-String).Trim()
    if ($PidNum) {
        $Proc = Get-Process -Id $PidNum -ErrorAction SilentlyContinue
        if ($Proc) {
            Write-Host "-> Stopping Sirius Core Native Manager (PID: $PidNum)..." -ForegroundColor Yellow
            Stop-Process -Id $PidNum -ErrorAction SilentlyContinue
            $Stopped = $true
        }
    }
    Remove-Item -Force $PidFile -ErrorAction SilentlyContinue
}

# 2. Terminate any sirius-core manager processes
Get-Process -Name "sirius-core" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "-> Stopping manager process (PID: $($_.Id))..." -ForegroundColor Yellow
    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
    $Stopped = $true
}

# 3. Gracefully terminate blockchain engine processes
$EngineNames = @("sirius", "sirius.bc", "catapult.recovery")
foreach ($name in $EngineNames) {
    Get-Process -Name $name -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "-> Terminating engine process $name (PID: $($_.Id))..." -ForegroundColor Yellow
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        $Stopped = $true
    }
}

# 4. Check if any process is still holding port 8080 or 3080
if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
    foreach ($p in @($Port, 8080, 3080)) {
        $connections = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue
        foreach ($conn in $connections) {
            if ($conn.OwningProcess -gt 0) {
                Write-Host "-> Freeing port $p (PID: $($conn.OwningProcess))..." -ForegroundColor Yellow
                Stop-Process -Id $conn.OwningProcess -Force -ErrorAction SilentlyContinue
                $Stopped = $true
            }
        }
    }
}

# 5. Detect data directory and clean all stale lock files
$DataDir = Join-Path $RootDir "chainconfig\data"
$UserProps = Join-Path $RootDir "chainconfig\resources\config-user.properties"
if (Test-Path $UserProps) {
    $match = Select-String -Path $UserProps -Pattern "^\s*data\.path\s*=\s*(.+)$"
    if ($match) {
        $customData = $match.Matches[0].Groups[1].Value.Trim().Trim('"').Trim("'")
        if ($customData) {
            if ([System.IO.Path]::IsPathRooted($customData)) {
                $DataDir = $customData
            } else {
                $DataDir = Join-Path $RootDir $customData
            }
        }
    }
}

if (Test-Path $DataDir) {
    Get-ChildItem -Path $DataDir -Filter "*.lock" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    $StateDbDir = Join-Path $DataDir "statedb"
    if (Test-Path $StateDbDir) {
        Get-ChildItem -Path $StateDbDir -Filter "LOCK" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    }
    Write-Host "-> Cleared blockchain lock files in $DataDir" -ForegroundColor Gray
}

if ($Stopped) {
    Write-Host "✓ ProximaX Sirius Core node processes stopped cleanly." -ForegroundColor Green
} else {
    Write-Host "ℹ No running Sirius Core processes detected." -ForegroundColor Gray
}
