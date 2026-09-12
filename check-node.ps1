$ErrorActionPreference = 'SilentlyContinue'

$s = Invoke-RestMethod http://localhost:8080/api/status
$statusSummary = [PSCustomObject]@{
    Status = $s.status
    BlockHeight = $s.blockHeight
    NetworkHeight = $s.networkHeight
    HeightDelta = ($s.networkHeight - $s.blockHeight)
    PeersCount = $s.peersCount
    Harvesting = $s.harvestStats
} | ConvertTo-Json -Depth 3

$logFile = Get-ChildItem "C:\Project\proximax-sirius-core\chainconfig\logs\server_*.log" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
$logs = if ($logFile) { Get-Content $logFile.FullName -Tail 35 | Out-String } else { "No logs found" }

$wslProc = wsl.exe -d Ubuntu-22.04 -u root -- ps -eo pid,%cpu,%mem,rss,cmd | grep "sirius.bc" | Out-String

$report = @"
You are a Blockchain SRE for ProximaX Sirius Mainnet.
Analyze my node's live state and give me a clear health report:
1. Are blocks advancing normally? (BlockHeight vs NetworkHeight)
2. Are there any errors in the logs (rejections, desync, RocksDB stalls)?
3. Is CPU and RAM usage normal?
4. Verdict: [HEALTHY / WARNING / CRITICAL] and any actions needed.

--- LIVE NODE DATA ---
API STATUS:
$statusSummary

WSL PROCESS USAGE:
$wslProc

RECENT ENGINE LOGS:
$logs
"@

Set-Clipboard -Value $report
Write-Host ""
Write-Host "==========================================================" -ForegroundColor Green
Write-Host " >>> SUCCESS! Data copied to your clipboard!           <<<" -ForegroundColor Green
Write-Host " >>> Now switch to Qwen and press Ctrl + V             <<<" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Green
Write-Host ""
