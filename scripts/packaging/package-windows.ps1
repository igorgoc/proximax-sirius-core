<#
.SYNOPSIS
    Packages ProximaX Sirius Core Native for Windows (amd64)
#>

param(
    [string]$Version = "",
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"

if (-not $Version) { $Version = $env:VERSION }
if (-not $Version) { $Version = $env:GITHUB_REF_NAME }
if (-not $Version) { $Version = "1.9.8" }
$Version = $Version -replace '^v', ''
if (-not ($Version -match '^\d')) {
    $Version = "1.9.8"
}

if ($PSScriptRoot) {
    $RootDir = (Resolve-Path "$PSScriptRoot\..\..").Path
} else {
    $RootDir = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Definition))
}
Set-Location $RootDir

$DistDir = Join-Path $RootDir "dist\windows-$Arch"
$BuildDir = Join-Path $RootDir "dist\build-windows-$Arch\proximax-sirius-core"

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  Packaging ProximaX Sirius Core for Windows ($Arch)      " -ForegroundColor Cyan
Write-Host "  Version: $Version                                       " -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan

if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
if (Test-Path $DistDir) { Remove-Item -Recurse -Force $DistDir }
New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
New-Item -ItemType Directory -Path $DistDir -Force | Out-Null

# 1. Build frontend if needed
$FrontendDist = Join-Path $RootDir "backend\dist"
if (-not (Test-Path $FrontendDist)) {
    Write-Host "-> Building React UI..." -ForegroundColor Green
    Set-Location (Join-Path $RootDir "frontend")
    npm run build
    Set-Location $RootDir
}

# 2. Compile Go backend for Windows
Write-Host "-> Compiling Go backend for Windows ($Arch)..." -ForegroundColor Green
Set-Location (Join-Path $RootDir "backend")
$BackendOut = Join-Path $BuildDir "sirius-core.exe"
go build -ldflags="-s -w" -o $BackendOut .
Set-Location $RootDir

# 3. Stage directories
Write-Host "-> Staging configuration files and launchers..." -ForegroundColor Green
$TargetResources = Join-Path $BuildDir "chainconfig\resources"
$TargetData = Join-Path $BuildDir "chainconfig\data\00000"
$TargetLogs = Join-Path $BuildDir "chainconfig\logs"
$TargetBin = Join-Path $BuildDir "bin"

New-Item -ItemType Directory -Path $TargetResources -Force | Out-Null
New-Item -ItemType Directory -Path $TargetData -Force | Out-Null
New-Item -ItemType Directory -Path $TargetLogs -Force | Out-Null
New-Item -ItemType Directory -Path $TargetBin -Force | Out-Null
if (Test-Path (Join-Path $RootDir "bin")) {
    Copy-Item -Recurse (Join-Path $RootDir "bin\*") $TargetBin -Force -ErrorAction SilentlyContinue
}

# Copy engine & manager compatibility manifests
Copy-Item (Join-Path $RootDir "chainconfig\engine.compat.json") (Join-Path $BuildDir "chainconfig\engine.compat.json")
if (Test-Path (Join-Path $RootDir "chainconfig\manager.compat.json")) {
    Copy-Item (Join-Path $RootDir "chainconfig\manager.compat.json") (Join-Path $BuildDir "chainconfig\manager.compat.json")
}

# Copy resource files strictly without active private keys or sensitive state
Copy-Item -Recurse (Join-Path $RootDir "chainconfig\resources\*") $TargetResources
$SensitiveFiles = @("config-harvesting.properties", "config-storage.properties", "config-user.properties", "config-manager.properties", ".sirius-token", "harvest-stats.json")
foreach ($sf in $SensitiveFiles) {
    $p = Join-Path $TargetResources $sf
    if (Test-Path $p) { Remove-Item -Force $p }
}

# Copy clean starter templates
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-harvesting.properties.template") (Join-Path $TargetResources "config-harvesting.properties")
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-storage.properties.template") (Join-Path $TargetResources "config-storage.properties")
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-user.properties.template") (Join-Path $TargetResources "config-user.properties")

# Genesis bootstrap data
if (Test-Path (Join-Path $RootDir "chainconfig\data\00000\00001.dat")) {
    Copy-Item (Join-Path $RootDir "chainconfig\data\00000\00001.dat") $TargetData
    Copy-Item (Join-Path $RootDir "chainconfig\data\00000\hashes.dat") $TargetData
}

# Copy launchers and helper scripts
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\start-node.ps1") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\stop-node.ps1") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\restart-node.ps1") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\start.bat") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\stop.bat") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\restart.bat") $BuildDir
Copy-Item (Join-Path $RootDir "scripts\packaging\windows\WINDOWS_DEFENDER_NOTES.md") $BuildDir
if (Test-Path (Join-Path $RootDir "WINDOWS_HANDOVER.md")) {
    Copy-Item (Join-Path $RootDir "WINDOWS_HANDOVER.md") $BuildDir
}

# 4. Create ZIP distribution
$ZipName = "proximax-sirius-windows-$Arch-$Version.zip"
$ZipPath = Join-Path $DistDir $ZipName
Write-Host "-> Compressing release archive: $ZipName..." -ForegroundColor Green
Compress-Archive -Path (Join-Path $BuildDir "*") -DestinationPath $ZipPath -Force

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "  Windows Packaging Complete!" -ForegroundColor Green
Write-Host "  Archive: $ZipPath" -ForegroundColor White
Write-Host "=========================================================" -ForegroundColor Cyan
