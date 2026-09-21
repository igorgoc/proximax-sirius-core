<#
.SYNOPSIS
    ProximaX Sirius Mainnet Peer Node Restart Script for Windows
#>

[CmdletBinding()]
param(
    [int]$Port = 8080,
    [switch]$Foreground,
    [string]$DataPath = ""
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

if (Test-Path (Join-Path $ScriptDir "..\..\..\chainconfig\resources")) {
    $RootDir = (Resolve-Path (Join-Path $ScriptDir "..\..\..")).Path
    $StopScript = Join-Path $ScriptDir "stop-node.ps1"
    $StartScript = Join-Path $ScriptDir "start-node.ps1"
} elseif (Test-Path (Join-Path $ScriptDir "..\..\chainconfig\resources")) {
    $RootDir = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
    $StopScript = Join-Path $ScriptDir "stop-node.ps1"
    $StartScript = Join-Path $ScriptDir "start-node.ps1"
} elseif (Test-Path (Join-Path $ScriptDir "chainconfig\resources")) {
    $RootDir = (Resolve-Path $ScriptDir).Path
    $StopScript = Join-Path $RootDir "stop-node.ps1"
    $StartScript = Join-Path $RootDir "start-node.ps1"
} else {
    $RootDir = $ScriptDir
    $StopScript = Join-Path $RootDir "stop-node.ps1"
    $StartScript = Join-Path $RootDir "start-node.ps1"
}
Set-Location $RootDir

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  Restarting ProximaX Sirius Mainnet Peer Node...        " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

# 1. Stop existing node
& $StopScript -Port $Port

Write-Host "Waiting 2 seconds before restart..." -ForegroundColor Gray
Start-Sleep -Seconds 2

# 2. Start node
$startParams = @{ Port = $Port }
if ($Foreground) { $startParams["Foreground"] = $true }
if ($DataPath) { $startParams["DataPath"] = $DataPath }

& $StartScript @startParams
