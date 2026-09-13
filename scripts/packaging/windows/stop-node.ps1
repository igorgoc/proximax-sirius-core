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
Write-Host "  Stopping ProximaX Sirius Native Node Gracefully...     " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

# 1. Preferred Method: Graceful API Shutdown via HTTP POST /api/system/shutdown
# This invokes StopNode() -> stopWSL() with multi-signal forward progress tracking in WSL2
try {
    $shutdownUrl = "http://127.0.0.1:$Port/api/system/shutdown"
    $response = Invoke-RestMethod -Uri $shutdownUrl -Method Post -TimeoutSec 3 -ErrorAction Stop
    if ($response -and $response.status -eq "ok") {
        Write-Host "-> Sent graceful shutdown command to Node Manager API (port $Port)..." -ForegroundColor Green
        Write-Host -NoNewline "-> Waiting for WSL2 blockchain engine & RocksDB flush" -ForegroundColor Gray

        for ($i = 1; $i -le 60; $i++) {
            $mgrRunning = $false
            if (Test-Path $PidFile) {
                $pidVal = (Get-Content $PidFile -ErrorAction SilentlyContinue | Out-String).Trim()
                if ($pidVal -and (Get-Process -Id $pidVal -ErrorAction SilentlyContinue)) {
                    $mgrRunning = $true
                }
            }
            # Also check if sirius.bc is running inside WSL
            $wslRunning = $false
            if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
                $wslOut = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
                if ($wslOut -and $wslOut.Trim().Length -gt 0) {
                    $wslRunning = $true
                }
            }

            if (-not $mgrRunning -and -not $wslRunning) {
                $Stopped = $true
                Write-Host ""
                Write-Host "✓ Node manager and WSL engine terminated cleanly via API." -ForegroundColor Green
                break
            }
            Write-Host -NoNewline "." -ForegroundColor Gray
            Start-Sleep -Seconds 1
        }
        Write-Host ""
    }
} catch {
    # API was offline, fall back to direct process handling
}

# 2. Fallback: If API was offline or processes remain, handle via process signals
if (-not $Stopped) {
    # 2a. Stop manager process via PID
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
    }

    # 2b. Stop any sirius-core manager processes
    Get-Process -Name "sirius-core" -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "-> Stopping manager process (PID: $($_.Id))..." -ForegroundColor Yellow
        Stop-Process -Id $_.Id -ErrorAction SilentlyContinue
        $Stopped = $true
    }

    # 2c. Gracefully signal the C++ Catapult engine inside WSL2
    if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
        $wslOut = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
        if ($wslOut -and $wslOut.Trim().Length -gt 0) {
            Write-Host "-> Sending SIGINT to sirius.bc inside WSL2..." -ForegroundColor Yellow
            wsl.exe -u root -- pkill -INT -f "sirius.bc" 2>$null

            Write-Host -NoNewline "-> Waiting for RocksDB state cache to flush in WSL2" -ForegroundColor Gray
            $wslStopped = $false
            for ($i = 1; $i -le 60; $i++) {
                $check = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
                if (-not $check -or $check.Trim().Length -eq 0) {
                    $wslStopped = $true
                    Write-Host ""
                    Write-Host "✓ sirius.bc engine stopped cleanly inside WSL2 ($i s elapsed)." -ForegroundColor Green
                    break
                }
                Write-Host -NoNewline "." -ForegroundColor Gray
                Start-Sleep -Seconds 1
            }

            if (-not $wslStopped) {
                Write-Host ""
                Write-Host "<warning> sirius.bc did not terminate within 60s. Forcing shutdown..." -ForegroundColor Red
                wsl.exe -u root -- pkill -KILL -f "sirius.bc" 2>$null
            }
            # Flush WSL filesystem buffers
            wsl.exe -u root -- sync 2>$null
            $Stopped = $true
        }
    }

    # 2d. Free port if lingering
    if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
        $connections = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        foreach ($conn in $connections) {
            if ($conn.OwningProcess -gt 0) {
                Write-Host "-> Freeing port $Port (PID: $($conn.OwningProcess))..." -ForegroundColor Yellow
                Stop-Process -Id $conn.OwningProcess -Force -ErrorAction SilentlyContinue
                $Stopped = $true
            }
        }
    }
}

# Remove PID file
Remove-Item -Force $PidFile -ErrorAction SilentlyContinue

# 3. Detect data directory and clean all stale lock files ONLY when engine is confirmed stopped
$isEngineRunning = $false
if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
    $check = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
    if ($check -and $check.Trim().Length -gt 0) {
        $isEngineRunning = $true
    }
}

if (-not $isEngineRunning) {
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
} else {
    Write-Warning "sirius.bc process is still active. Lock files preserved."
}

Write-Host "=========================================================" -ForegroundColor Cyan
if ($Stopped) {
    Write-Host "✓ ProximaX Sirius Core node stopped cleanly." -ForegroundColor Green
} else {
    Write-Host "ℹ No running Sirius Core processes detected." -ForegroundColor Gray
}
Write-Host "=========================================================" -ForegroundColor Cyan
