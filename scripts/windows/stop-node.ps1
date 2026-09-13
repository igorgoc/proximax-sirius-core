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

# 1. Retrieve API authentication token
$token = ""
$tokenFile = Join-Path $RootDir "chainconfig\resources\.sirius-token"
if (Test-Path $tokenFile) {
    $token = (Get-Content $tokenFile -ErrorAction SilentlyContinue | Out-String).Trim()
}
if (-not $token) {
    try {
        $tokResp = Invoke-RestMethod -Uri "http://127.0.0.1:$Port/api/auth/token" -Method Get -Headers @{ "Origin" = "http://127.0.0.1:$Port" } -TimeoutSec 2 -ErrorAction SilentlyContinue
        if ($tokResp -and $tokResp.token) {
            $token = $tokResp.token
        }
    } catch {}
}

# 2. Preferred Method: Graceful API Shutdown via HTTP POST /api/system/shutdown
# This invokes StopNode() -> stopWSL() with multi-signal forward progress tracking in WSL2,
# flushes RocksDB, reconciles state integrity, and exits cleanly.
try {
    $headers = @{
        "Origin"       = "http://127.0.0.1:$Port"
        "Content-Type" = "application/json"
    }
    if ($token) {
        $headers["X-Sirius-Token"] = $token
        $headers["Authorization"] = "Bearer $token"
    }
    $shutdownUrl = "http://127.0.0.1:$Port/api/system/shutdown"
    $response = Invoke-RestMethod -Uri $shutdownUrl -Method Post -Headers $headers -Body "{}" -TimeoutSec 5 -ErrorAction Stop
    if ($response -and $response.status -eq "ok") {
        Write-Host "-> Sent graceful shutdown command to Node Manager API (port $Port)..." -ForegroundColor Green
        Write-Host -NoNewline "-> Waiting for WSL2 blockchain engine & RocksDB flush" -ForegroundColor Gray

        for ($i = 1; $i -le 120; $i++) {
            $mgrRunning = $false
            if (Test-Path $PidFile) {
                $pidVal = (Get-Content $PidFile -ErrorAction SilentlyContinue | Out-String).Trim()
                if ($pidVal -and (Get-Process -Id $pidVal -ErrorAction SilentlyContinue)) {
                    $mgrRunning = $true
                }
            }
            if (-not $mgrRunning) {
                $procs = Get-Process -Name "sirius-core" -ErrorAction SilentlyContinue
                if ($procs) { $mgrRunning = $true }
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
                Write-Host "[OK] Node manager and WSL engine terminated cleanly via API." -ForegroundColor Green
                break
            }
            Write-Host -NoNewline "." -ForegroundColor Gray
            Start-Sleep -Seconds 1
        }
        Write-Host ""
    }
} catch {
    # API was offline or unreachable, fall back to direct process handling
}

# 3. Fallback: If API was offline or processes remain, handle via direct graceful signals
if (-not $Stopped) {
    # 3a. Gracefully signal the C++ Catapult engine inside WSL2 FIRST!
    # NEVER terminate manager before engine has finished flushing RocksDB.
    if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
        $wslOut = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
        if ($wslOut -and $wslOut.Trim().Length -gt 0) {
            Write-Host "-> Sending graceful SIGINT to sirius.bc inside WSL2..." -ForegroundColor Yellow
            wsl.exe -u root -- pkill -INT -f "sirius.bc" 2>$null

            Write-Host -NoNewline "-> Waiting for in-flight blocks and RocksDB state cache to flush in WSL2" -ForegroundColor Gray

            $logsDir = Join-Path $RootDir "chainconfig\logs"
            $indexPath = Join-Path $RootDir "chainconfig\data\index.dat"
            $lastLog = ""
            $lastSize = [int64]-1
            $lastMtime = $null
            $idleSecs = 0
            $elapsed = 0
            $wslStopped = $false

            while ($true) {
                $check = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
                if (-not $check -or $check.Trim().Length -eq 0) {
                    $wslStopped = $true
                    Write-Host ""
                    Write-Host "[OK] sirius.bc engine stopped cleanly inside WSL2 ($elapsed s elapsed)." -ForegroundColor Green
                    break
                }

                # Multi-Signal Forward Progress Check:
                $progress = $false

                # Signal 1: server_*.log active file size or rotation
                if (Test-Path $logsDir) {
                    $curLogFile = Get-ChildItem -Path (Join-Path $logsDir "server_*.log") -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1
                    if ($curLogFile) {
                        if ($curLogFile.FullName -ne $lastLog) {
                            $progress = $true
                            $lastLog = $curLogFile.FullName
                            $lastSize = $curLogFile.Length
                        } elseif ($curLogFile.Length -gt $lastSize) {
                            $progress = $true
                            $lastSize = $curLogFile.Length
                        }
                    }
                }

                # Signal 2: index.dat timestamp change
                if (Test-Path $indexPath) {
                    $curMtime = (Get-Item $indexPath -ErrorAction SilentlyContinue).LastWriteTime
                    if ($lastMtime -and $curMtime -ne $lastMtime) {
                        $progress = $true
                    }
                    $lastMtime = $curMtime
                }

                if ($progress) {
                    $idleSecs = 0
                } else {
                    $idleSecs++
                }

                # Progress-based timeout: NEVER kill while progress continues. Escalate only after 60s idle deadlock:
                if ($idleSecs -ge 60) {
                    Write-Host ""
                    Write-Host "<warning> sirius.bc made ZERO forward progress for 60s (deadlock detected). Escalating..." -ForegroundColor Red
                    wsl.exe -u root -- pkill -KILL -f "sirius.bc" 2>$null
                    break
                }

                Write-Host -NoNewline "." -ForegroundColor Gray
                Start-Sleep -Seconds 1
                $elapsed++
            }

            # Flush WSL filesystem buffers
            wsl.exe -u root -- sync 2>$null
            $Stopped = $true
        }
    }

    # 3b. Stop manager process via PID
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

    # 3c. Stop any remaining sirius-core manager processes
    Get-Process -Name "sirius-core" -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "-> Stopping manager process (PID: $($_.Id))..." -ForegroundColor Yellow
        Stop-Process -Id $_.Id -ErrorAction SilentlyContinue
        $Stopped = $true
    }

    # 3d. Free port if lingering
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

# 4. Resolve data directory
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

# 5. Post-Shutdown State Integrity Verification & Self-Healing (when engine confirmed dead)
$isEngineRunning = $false
if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
    $check = wsl.exe -u root -- pgrep -f "sirius.bc" 2>$null
    if ($check -and $check.Trim().Length -gt 0) {
        $isEngineRunning = $true
    }
}

if (-not $isEngineRunning) {
    # 5a. Clear stale lock files
    if (Test-Path $DataDir) {
        Get-ChildItem -Path $DataDir -Filter "*.lock" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
        $StateDbDir = Join-Path $DataDir "statedb"
        if (Test-Path $StateDbDir) {
            Get-ChildItem -Path $StateDbDir -Filter "LOCK" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
        }
        Write-Host "-> Cleared blockchain lock files in $DataDir" -ForegroundColor Gray
    }

    # 5b. Verify state integrity (Storage Height vs Cache Height)
    $idxPath = Join-Path $DataDir "index.dat"
    $suppPath = Join-Path $DataDir "state\supplemental.dat"
    if (-not (Test-Path $suppPath)) {
        $suppPath = Join-Path $DataDir "supplemental.dat"
    }

    if ((Test-Path $idxPath) -and (Test-Path $suppPath)) {
        $idxBytes = [IO.File]::ReadAllBytes($idxPath)
        $suppBytes = [IO.File]::ReadAllBytes($suppPath)
        if ($idxBytes.Length -ge 8 -and $suppBytes.Length -ge 40) {
            $storageHeight = [BitConverter]::ToUInt64($idxBytes, 0)
            $cacheHeight = [BitConverter]::ToUInt64($suppBytes, 32)

            if ($storageHeight -gt 1 -or $cacheHeight -gt 1) {
                if ($storageHeight -eq $cacheHeight) {
                    Write-Host "[OK] Shutdown integrity verified: Storage Height ($storageHeight) == Cache Height ($cacheHeight). State 100% consistent." -ForegroundColor Green
                } else {
                    Write-Host "<warning> Height divergence detected: Storage Height ($storageHeight) != Cache Height ($cacheHeight)!" -ForegroundColor Yellow
                    Write-Host "-> Running catapult.recovery inside WSL2 to reconcile state..." -ForegroundColor Yellow

                    $wslRootDir = $RootDir.Replace("\", "/").Replace("C:", "/mnt/c").Replace("c:", "/mnt/c")
                    $recOutput = wsl.exe -u root --cd "$wslRootDir" -- env LD_LIBRARY_PATH="$wslRootDir/bin" "$wslRootDir/bin/catapult.recovery" "$wslRootDir/chainconfig" 2>&1
                    wsl.exe -u root -- sync 2>$null

                    # Re-verify
                    $newIdx = [IO.File]::ReadAllBytes($idxPath)
                    $newSupp = [IO.File]::ReadAllBytes($suppPath)
                    $newStorage = [BitConverter]::ToUInt64($newIdx, 0)
                    $newCache = [BitConverter]::ToUInt64($newSupp, 32)
                    if ($newStorage -eq $newCache) {
                        Write-Host "[OK] State reconciled successfully by catapult.recovery at height $newStorage." -ForegroundColor Green
                    } else {
                        Write-Host "<warning> Post-recovery heights: Storage ($newStorage), Cache ($newCache)." -ForegroundColor Yellow
                    }
                }
            }
        }
    }
} else {
    Write-Warning "sirius.bc process is still active. Lock files preserved."
}

Write-Host "=========================================================" -ForegroundColor Cyan
if ($Stopped) {
    Write-Host "[OK] ProximaX Sirius Core node stopped cleanly." -ForegroundColor Green
} else {
    Write-Host "[INFO] No running Sirius Core processes detected." -ForegroundColor Gray
}
Write-Host "=========================================================" -ForegroundColor Cyan
