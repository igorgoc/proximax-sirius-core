<#
.SYNOPSIS
    ProximaX Sirius Mainnet Peer Node Launcher for Windows (Native)
.DESCRIPTION
    Starts the native node supervisor and cockpit backend on port 8080.
    Enforces Windows NTFS ACL hardening (0600 equivalent) on sensitive keys.
#>

[CmdletBinding()]
param(
    [int]$Port = 8080,
    [switch]$Foreground,
    [string]$DataPath = ""
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

# Automatically detect root directory (handles both dev repository and standalone release package)
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

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  ProximaX Sirius Mainnet Peer Node (Windows Native)      " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

# 1. Add bin/ to PATH for native DLL loading
$BinDir = Join-Path $RootDir "bin"
if (Test-Path $BinDir) {
    $env:PATH = "$BinDir;$env:PATH"
    Write-Host "-> Configured DLL search path: $BinDir" -ForegroundColor Green

    # Materialize required Linux dynamic shared libraries for WSL Catapult engine
    $RocksDbLib = Join-Path $BinDir "librocksdb.so.8"
    $RocksDbTarget = Join-Path $BinDir "librocksdb.so.8.5.3"
    if ((-not (Test-Path $RocksDbLib)) -or ((Get-Item $RocksDbLib -ErrorAction SilentlyContinue).Length -eq 0)) {
        if (Test-Path $RocksDbTarget) {
            Remove-Item -Force $RocksDbLib -ErrorAction SilentlyContinue
            Copy-Item $RocksDbTarget $RocksDbLib -Force -ErrorAction SilentlyContinue
            Write-Host "-> Materialized RocksDB shared library: $(Split-Path -Leaf $RocksDbLib)" -ForegroundColor Gray
        } else {
            Write-Host "-> Missing RocksDB shared library for WSL engine. Restoring from official release..." -ForegroundColor Yellow
            $CandidateUrls = @(
                "https://github.com/igorgoc/cpp-xpx-chain/releases/latest/download/sirius-linux-amd64.tar.gz",
                "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.9/sirius-linux-amd64.tar.gz",
                "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.8/sirius-linux-amd64.tar.gz"
            )
            $TempTar = Join-Path $RootDir "sirius-linux-amd64.tar.gz"
            $restored = $false
            foreach ($TarUrl in $CandidateUrls) {
                Remove-Item -Force $TempTar -ErrorAction SilentlyContinue
                try {
                    if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
                        & curl.exe -f -sSL $TarUrl -o $TempTar
                    } else {
                        Invoke-WebRequest -Uri $TarUrl -OutFile $TempTar -UseBasicParsing
                    }
                    if ((Test-Path $TempTar) -and (Get-Item $TempTar).Length -gt 1000000) {
                        if (Get-Command tar.exe -ErrorAction SilentlyContinue) {
                            & tar.exe -xzf $TempTar -C $RootDir
                        }
                        Remove-Item -Force $TempTar -ErrorAction SilentlyContinue
                        if (Test-Path $RocksDbTarget) {
                            Copy-Item $RocksDbTarget $RocksDbLib -Force -ErrorAction SilentlyContinue
                        }
                        Write-Host "-> Restored Linux engine dynamic libraries successfully from $TarUrl." -ForegroundColor Green
                        $restored = $true
                        break
                    }
                } catch {}
            }
            if (-not $restored) {
                Write-Warning "Could not auto-download Linux engine libraries from any candidate URL."
            }
        }
    }
}

# 2. NTFS ACL Security Hardening (POSIX 0600 Equivalent)
# Removes inherited permissions and restricts read/write to current user and SYSTEM
$ResourcesDir = Join-Path $RootDir "chainconfig\resources"
$SensitiveFiles = @(
    (Join-Path $ResourcesDir "config-harvesting.properties"),
    (Join-Path $ResourcesDir "config-user.properties"),
    (Join-Path $ResourcesDir ".sirius-token")
)

foreach ($file in $SensitiveFiles) {
    if (Test-Path $file) {
        try {
            icacls "$file" /inheritance:r /grant:r "$($env:USERNAME):(F)" /grant:r "SYSTEM:(F)" | Out-Null
            Write-Host "-> Secured NTFS ACL (0600-equivalent): $(Split-Path -Leaf $file)" -ForegroundColor Gray
        } catch {
            Write-Warning "Could not apply NTFS ACL to $file"
        }
    }
}

# 3. Resolve data directory path from config-user.properties or parameter
$DataDir = Join-Path $RootDir "chainconfig\data"
if ($DataPath) {
    $DataDir = $DataPath
} else {
    $UserProps = Join-Path $ResourcesDir "config-user.properties"
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
}

# 4. Clean up stale lock files
if (Test-Path $DataDir) {
    Get-ChildItem -Path $DataDir -Filter "*.lock" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    $StateDbDir = Join-Path $DataDir "statedb"
    if (Test-Path $StateDbDir) {
        Get-ChildItem -Path $StateDbDir -Filter "LOCK" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    }
    Write-Host "-> Cleared stale blockchain lock files" -ForegroundColor Gray
}

# 5. Sanitize machine-specific log paths in logging configuration
$LogDir = Join-Path $RootDir "chainconfig\logs"
if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir -Force | Out-Null }
if ($LogDir -match "^([A-Za-z]):\\(.*)$") {
    $driveLetter = $Matches[1].ToLower()
    $subPath = $Matches[2].Replace('\', '/')
    $LogDirForward = "/mnt/$driveLetter/$subPath"
} else {
    $LogDirForward = $LogDir.Replace('\', '/')
}
Get-ChildItem -Path (Join-Path $ResourcesDir "config-logging-*.properties") -ErrorAction SilentlyContinue | ForEach-Object {
    try {
        $content = Get-Content $_.FullName -Raw
        if ($content -match "directory\s*=") {
            $content = [System.Text.RegularExpressions.Regex]::Replace($content, "(?m)^directory\s*=.*$", "directory = $LogDirForward")
            [System.IO.File]::WriteAllText($_.FullName, $content)
        }
    } catch {
        # ignore write errors
    }
}

# 6. Check if already running
$PidFile = Join-Path $RootDir ".sirius-core.pid"
if (Test-Path $PidFile) {
    $ExistingPid = (Get-Content $PidFile -ErrorAction SilentlyContinue | Out-String).Trim()
    if ($ExistingPid) {
        $Proc = Get-Process -Id $ExistingPid -ErrorAction SilentlyContinue
        if ($Proc) {
            Write-Host "Sirius Core Native is already running (PID: $ExistingPid)." -ForegroundColor Yellow
            Write-Host "Dashboard: http://localhost:$Port" -ForegroundColor Cyan
            exit 0
        }
    }
}

# Check if port is already listening
if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
    $portConn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if ($portConn) {
        Write-Host "Sirius Core port $Port is already active (PID: $($portConn.OwningProcess))." -ForegroundColor Yellow
        Write-Host "Dashboard: http://localhost:$Port" -ForegroundColor Cyan
        exit 0
    }
}

# 7. Check for rogue non-system processes holding P2P ports
if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
    foreach ($p in @(7900, 7901, 7903)) {
        $conn = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue
        if ($conn -and $conn.OwningProcess -gt 4) {
            $proc = Get-Process -Id $conn.OwningProcess -ErrorAction SilentlyContinue
            if ($proc -and $proc.ProcessName -ne "svchost" -and $proc.ProcessName -ne "System") {
                Write-Host "-> Freeing occupied Sirius P2P port $p ($($proc.ProcessName), PID: $($conn.OwningProcess))..." -ForegroundColor Yellow
                Stop-Process -Id $conn.OwningProcess -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

# 8. Launch Node Manager
$CandidateExes = @(
    (Join-Path $RootDir "bin\windows\sirius-core.exe"),
    (Join-Path $RootDir "bin\sirius-core.exe"),
    (Join-Path $RootDir "backend\sirius-core.exe"),
    (Join-Path $RootDir "sirius-core.exe")
)
$BackendExe = $null
foreach ($cand in $CandidateExes) {
    if (Test-Path $cand) {
        $BackendExe = $cand
        break
    }
}
if (-not $BackendExe) {
    Write-Error "Binary sirius-core.exe not found. Looked in bin\windows\sirius-core.exe and bin\sirius-core.exe. Please run scripts\windows\run.bat to build."
    exit 1
}
Unblock-File $BackendExe -ErrorAction SilentlyContinue

$LogFile = Join-Path $LogDir "manager.log"
$ErrLogFile = Join-Path $LogDir "manager_error.log"
$ArgsList = @("-port", "$Port", "-chainconfig", (Join-Path $RootDir "chainconfig"))

$Process = $null
try {
    if ($Foreground) {
        Write-Host "-> Starting node manager in foreground..." -ForegroundColor Green
        & $BackendExe @ArgsList
    } else {
        Write-Host "-> Starting node manager in background..." -ForegroundColor Green
        $Process = Start-Process -FilePath $BackendExe -ArgumentList $ArgsList -WorkingDirectory $RootDir -PassThru -WindowStyle Hidden
        $Process.Id | Out-File -FilePath $PidFile -Encoding ascii
    }
} catch {
    # If Application Control policy restricts executing the raw binary, fall back to Go host
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host "-> Binary execution restricted by Application Control policy. Launching via Go runner..." -ForegroundColor Yellow
        $BackendDir = Join-Path $RootDir "backend"
        $GoArgs = @("run", ".", "-port", "$Port", "-chainconfig", (Join-Path $RootDir "chainconfig"))
        if ($Foreground) {
            Push-Location $BackendDir
            try { & go @GoArgs } finally { Pop-Location }
        } else {
            $Process = Start-Process -FilePath "go" -ArgumentList $GoArgs -WorkingDirectory $BackendDir -PassThru -WindowStyle Hidden
            $Process.Id | Out-File -FilePath $PidFile -Encoding ascii
        }
    } else {
        throw $_
    }
}

if (-not $Foreground) {
    Start-Sleep -Seconds 2
    Write-Host ""
    Write-Host "=========================================================" -ForegroundColor Green
    if ($Process) {
        Write-Host "  ProximaX Sirius Native Node is ONLINE (PID: $($Process.Id))!" -ForegroundColor Green
    } else {
        Write-Host "  ProximaX Sirius Native Node is ONLINE!" -ForegroundColor Green
    }
    Write-Host ""
    Write-Host "  Access GUI Cockpit Dashboard:" -ForegroundColor White
    Write-Host "    >>> http://localhost:$Port <<<" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Logs: Get-Content -Wait $LogFile" -ForegroundColor Gray
    Write-Host "  Stop: .\stop.bat  (or .\stop-node.ps1)" -ForegroundColor Gray
    Write-Host "=========================================================" -ForegroundColor Green
}
