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
    cmd.exe /c "npm run build"
    Set-Location $RootDir
}

# 2. Compile Go backend for Windows
Write-Host "-> Compiling Go backend for Windows ($Arch)..." -ForegroundColor Green
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    if (Test-Path "$env:ProgramFiles\Go\bin\go.exe") {
        $env:PATH = "$env:ProgramFiles\Go\bin;$env:PATH"
    } elseif (Test-Path "C:\Go\bin\go.exe") {
        $env:PATH = "C:\Go\bin;$env:PATH"
    }
}
Set-Location (Join-Path $RootDir "backend")
$BackendOut = Join-Path $BuildDir "sirius-core.exe"
go build -ldflags="-s -w" -o $BackendOut .

# Also stage into bin\windows in release package
$BuildWinBin = Join-Path $BuildDir "bin\windows"
New-Item -ItemType Directory -Path $BuildWinBin -Force | Out-Null
Copy-Item $BackendOut (Join-Path $BuildWinBin "sirius-core.exe")

# Also build Linux binary for WSL execution
Write-Host "-> Compiling Go backend for Linux (WSL engine)..." -ForegroundColor Green
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$LinuxBackendOut = Join-Path $BuildDir "sirius-core"
go build -ldflags="-s -w" -o $LinuxBackendOut .

$BuildLinuxBin = Join-Path $BuildDir "bin\linux"
New-Item -ItemType Directory -Path $BuildLinuxBin -Force | Out-Null
Copy-Item $LinuxBackendOut (Join-Path $BuildLinuxBin "sirius-core")

$env:GOOS = "windows"
$env:GOARCH = "amd64"
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
    Get-ChildItem -Path (Join-Path $RootDir "bin\*") -Exclude "*.so*" | Copy-Item -Destination $TargetBin -Force -ErrorAction SilentlyContinue
    if (Test-Path (Join-Path $RootDir "bin\libatomic.so.1")) {
        Copy-Item (Join-Path $RootDir "bin\libatomic.so.1") (Join-Path $TargetBin "libatomic.so.1") -Force -ErrorAction SilentlyContinue
    }
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

# Copy launchers and helper scripts (both at root of zip and in scripts\windows for maximum flexibility)
$WinScriptDir = Join-Path $RootDir "scripts\windows"
$BuildWinScripts = Join-Path $BuildDir "scripts\windows"
New-Item -ItemType Directory -Path $BuildWinScripts -Force | Out-Null
Copy-Item (Join-Path $WinScriptDir "*") $BuildWinScripts -Force

# Stage top-level launchers in release root
Copy-Item (Join-Path $WinScriptDir "start.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "stop.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "restart.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "run.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "start-node.ps1") $BuildDir
Copy-Item (Join-Path $WinScriptDir "stop-node.ps1") $BuildDir
Copy-Item (Join-Path $WinScriptDir "restart-node.ps1") $BuildDir
Copy-Item (Join-Path $WinScriptDir "pick-directory.ps1") $BuildDir
Copy-Item (Join-Path $WinScriptDir "setup-wsl.ps1") $BuildDir
Copy-Item (Join-Path $WinScriptDir "setup-wsl.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "WINDOWS_DEFENDER_NOTES.md") $BuildDir

if (Test-Path (Join-Path $RootDir "WINDOWS_HANDOVER.md")) {
    Copy-Item (Join-Path $RootDir "WINDOWS_HANDOVER.md") $BuildDir
}

# Stage Linux scripts for WSL execution support
$LinuxScriptDir = Join-Path $RootDir "scripts\linux"
$BuildLinuxScripts = Join-Path $BuildDir "scripts\linux"
New-Item -ItemType Directory -Path $BuildLinuxScripts -Force | Out-Null
if (Test-Path $LinuxScriptDir) {
    Copy-Item (Join-Path $LinuxScriptDir "*") $BuildLinuxScripts -Recurse -Force
}
if (Test-Path (Join-Path $LinuxScriptDir "start.sh")) {
    Copy-Item (Join-Path $LinuxScriptDir "start.sh") $BuildDir
    Copy-Item (Join-Path $LinuxScriptDir "stop.sh") $BuildDir
    Copy-Item (Join-Path $LinuxScriptDir "restart.sh") $BuildDir
}
if (Test-Path (Join-Path $LinuxScriptDir "run.sh")) {
    Copy-Item (Join-Path $LinuxScriptDir "run.sh") $BuildDir
}

# Ensure all staged shell scripts have strict Unix LF line endings
Get-ChildItem -Path $BuildDir -Filter "*.sh" -Recurse | ForEach-Object {
    $text = [System.IO.File]::ReadAllText($_.FullName)
    $text = $text.Replace("`r`n", "`n")
    [System.IO.File]::WriteAllText($_.FullName, $text, [System.Text.UTF8Encoding]::new($false))
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
