<#
.SYNOPSIS
    ProximaX Sirius Mainnet Peer Node Stopper for Windows
#>

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $ScriptDir

$PidFile = Join-Path $ScriptDir ".sirius-core.pid"
$Stopped = $false

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

# Also gracefully terminate any child sirius.exe processes
Get-Process -Name "sirius" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "-> Terminating blockchain engine process (PID: $($_.Id))..." -ForegroundColor Yellow
    Stop-Process -Id $_.Id -ErrorAction SilentlyContinue
    $Stopped = $true
}

# Clean lock file
$LockFile = Join-Path $ScriptDir "chainconfig\data\server.lock"
if (Test-Path $LockFile) {
    Remove-Item -Force $LockFile -ErrorAction SilentlyContinue
}

if ($Stopped) {
    Write-Host "✓ ProximaX Sirius Core node processes stopped cleanly." -ForegroundColor Green
} else {
    Write-Host "ℹ No running Sirius Core processes detected." -ForegroundColor Gray
}
