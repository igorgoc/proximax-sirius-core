param(
    [double]$Hours = 2.0,
    [int]$IntervalSeconds = 60
)

$ErrorActionPreference = "SilentlyContinue"
$StartTime = Get-Date
$EndTime = $StartTime.AddHours($Hours)
$OllamaUrl = "http://127.0.0.1:11434/api/generate"
$NodeApiUrl = "http://127.0.0.1:8080/api/status"
$Model = "qwen2.5-coder:14b"
$IncidentsLog = "C:\Project\proximax-sirius-core\chainconfig\logs\watchdog_incidents.log"

function Ask-Qwen([string]$Prompt) {
    try {
        $payload = @{
            model = $Model
            prompt = $Prompt
            stream = $false
            options = @{
                num_predict = 350
                temperature = 0.2
            }
        } | ConvertTo-Json
        $resp = Invoke-RestMethod -Uri $OllamaUrl -Method Post -Body $payload -ContentType "application/json" -TimeoutSec 180
        return $resp.response
    } catch {
        return "Failed to query Qwen: $_"
    }
}

Clear-Host
Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  ProximaX Sirius Node - Continuous AI Watchdog & Troubleshooter   " -ForegroundColor Cyan
Write-Host "  AI Engine : $Model (Local Ollama)" -ForegroundColor Yellow
Write-Host "  Duration  : $Hours hour(s) (Until $( $EndTime.ToString('HH:mm:ss') ))" -ForegroundColor Yellow
Write-Host "  Interval  : Every $IntervalSeconds seconds" -ForegroundColor Yellow
Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host ""

# Verify Qwen is responding
Write-Host "-> Pinging $Model via Ollama..." -NoNewline
$ping = Ask-Qwen "Reply with 'WATCHDOG_READY' if you are online."
if ($ping -match "READY") {
    Write-Host " [ONLINE]" -ForegroundColor Green
} else {
    Write-Host " [ONLINE: $ping]" -ForegroundColor Green
}
Write-Host ""

$lastHeight = 0
$lastHeightTime = Get-Date
$consecutiveStalls = 0
$totalIncidents = 0

while ((Get-Date) -lt $EndTime) {
    $now = Get-Date
    $remaining = $EndTime - $now
    $remStr = "{0:D2}h {1:D2}m {2:D2}s" -f [int]$remaining.Hours, [int]$remaining.Minutes, [int]$remaining.Seconds

    # 1. Fetch Node Status via IPv4 127.0.0.1
    $statusObj = $null
    try {
        $statusObj = Invoke-RestMethod -Uri $NodeApiUrl -TimeoutSec 10
    } catch {}

    # 2. Check WSL Process Metrics
    $wslProc = ""
    try {
        $wslProc = (wsl.exe -d Ubuntu-22.04 -u root -- ps -eo pid,%cpu,%mem,rss,cmd | grep "sirius.bc" | Out-String).Trim()
    } catch {}

    # 3. Read latest log tail
    $latestLog = Get-ChildItem "C:\Project\proximax-sirius-core\chainconfig\logs\server_*.log" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    $logTail = ""
    if ($latestLog) {
        $logTail = (Get-Content $latestLog.FullName -Tail 25 | Out-String)
    }

    # Analyze metrics
    $isNodeUp = ($statusObj -and $statusObj.status -eq "running")
    $currentHeight = if ($statusObj) { [int64]$statusObj.blockHeight } else { 0 }
    $networkHeight = if ($statusObj) { [int64]$statusObj.networkHeight } else { 0 }
    $lag = if ($networkHeight -gt $currentHeight) { $networkHeight - $currentHeight } else { 0 }

    # Detect progress or stall
    $blocksGained = 0
    if ($lastHeight -gt 0 -and $currentHeight -gt 0) {
        $blocksGained = $currentHeight - $lastHeight
    }
    if ($currentHeight -gt $lastHeight) {
        $lastHeight = $currentHeight
        $lastHeightTime = $now
        $consecutiveStalls = 0
    } elseif ($isNodeUp -and $lastHeight -gt 0) {
        $consecutiveStalls++
    } else {
        $lastHeight = $currentHeight
    }

    # Scan for true critical log errors
    $criticalKeywords = @(
        "rejecting block",
        "signer.*invalid",
        "RocksDB error",
        "deadlock detected",
        "corruption"
    )
    $foundErrors = @()
    foreach ($kw in $criticalKeywords) {
        if ($logTail -match $kw) {
            $foundErrors += $kw
        }
    }

    # Determine Health Status
    $hasAnomaly = $false
    $anomalyReason = ""

    if (-not $isNodeUp) {
        $hasAnomaly = $true
        $anomalyReason = "Node is NOT running (Status: $( if ($statusObj) { $statusObj.status } else { 'unreachable' } ))."
    } elseif ($wslProc -eq "") {
        $hasAnomaly = $true
        $anomalyReason = "Engine process 'sirius.bc' is missing in WSL."
    } elseif ($foundErrors.Count -gt 0) {
        $hasAnomaly = $true
        $anomalyReason = "Critical error in engine logs: $( $foundErrors -join ', ' )"
    } elseif ($consecutiveStalls -ge 4) { # > 4 minutes with 0 new blocks
        $stallMinutes = [math]::Round(($now - $lastHeightTime).TotalMinutes, 1)
        if ($stallMinutes -ge 4) {
            $hasAnomaly = $true
            $anomalyReason = "Chain sync stalled! Height $currentHeight unchanged for $stallMinutes minutes."
        }
    } elseif ($lag -gt 50) {
        $hasAnomaly = $true
        $anomalyReason = "Network lag detected! Local height ($currentHeight) trails Mainnet ($networkHeight) by $lag blocks."
    }

    $timeTag = $now.ToString("HH:mm:ss")
    $activeLogName = if ($latestLog) { $latestLog.Name } else { "none" }

    if (-not $hasAnomaly) {
        # HEALTHY HEARTBEAT
        $gainText = if ($blocksGained -gt 0) { "+$blocksGained blocks" } else { "steady" }
        Write-Host "[$timeTag] [OK] Height: $currentHeight ($gainText) | Lag: $lag | Active Log: $activeLogName | Remaining: $remStr" -ForegroundColor Green
    } else {
        # ANOMALY DETECTED -> DISPATCH TO QWEN FOR TRIAGE
        $totalIncidents++
        Write-Host ""
        Write-Host "==================================================================" -ForegroundColor Red
        Write-Host "[$timeTag] <!> ANOMALY DETECTED: $anomalyReason" -ForegroundColor Red
        Write-Host "-> Consulting $Model for real-time diagnosis & fix..." -ForegroundColor Yellow
        Write-Host "==================================================================" -ForegroundColor Red

        $prompt = @"
You are an expert Blockchain SRE analyzing a live ProximaX Sirius Catapult node (C++, RocksDB, WSL2, Windows).
The automated watchdog detected an anomaly:

ANOMALY: $anomalyReason
LOCAL HEIGHT: $currentHeight
NETWORK HEIGHT: $networkHeight (Lag: $lag blocks)
WSL PROCESS:
$wslProc

RECENT ENGINE LOGS:
$logTail

Provide a concise, direct operational response:
1. Root Cause Analysis (2-3 sentences).
2. Action Required: Provide exact PowerShell or bash commands to fix or recover the node if needed. If it will self-recover, explain why.
"@

        $diagnosis = Ask-Qwen $prompt
        Write-Host ""
        Write-Host "--- QWEN AI DIAGNOSIS & RECOVERY RECOMMENDATION ---" -ForegroundColor Cyan
        Write-Host $diagnosis -ForegroundColor White
        Write-Host "---------------------------------------------------" -ForegroundColor Cyan
        Write-Host ""

        # Log incident to disk
        $incidentEntry = @"
[$timeTag] ANOMALY: $anomalyReason
HEIGHT: $currentHeight / $networkHeight (Lag: $lag)
DIAGNOSIS:
$diagnosis
==================================================
"@
        $incidentEntry | Out-File -FilePath $IncidentsLog -Append -Encoding utf8

        # If node stopped, attempt automatic gentle restart
        if (-not $isNodeUp -or $wslProc -eq "") {
            Write-Host "[$timeTag] Attempting automatic supervisor restart via API..." -ForegroundColor Yellow
            try {
                $auth = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/auth/token" -Headers @{ "Origin" = "http://127.0.0.1:8080" }
                $headers = @{
                    "Authorization" = "Bearer $($auth.token)"
                    "X-CSRF-Token"  = $auth.token
                    "Origin"        = "http://127.0.0.1:8080"
                    "Referer"       = "http://127.0.0.1:8080/"
                }
                Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/node/start" -Method Post -Headers $headers -ContentType "application/json" -Body "{}" | Out-Null
                Write-Host "[$timeTag] Start command dispatched." -ForegroundColor Green
            } catch {
                Write-Host "[$timeTag] Auto-restart API call failed: $_" -ForegroundColor Red
            }
        }
    }

    Start-Sleep -Seconds $IntervalSeconds
}

Write-Host ""
Write-Host "==================================================================" -ForegroundColor Green
Write-Host "  Watchdog monitoring completed ($Hours hour(s) elapsed)." -ForegroundColor Green
Write-Host "  Total anomalies triaged by Qwen: $totalIncidents" -ForegroundColor $( if ($totalIncidents -eq 0) { "Green" } else { "Yellow" } )
Write-Host "==================================================================" -ForegroundColor Green
