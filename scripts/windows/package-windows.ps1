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
$SourceBin = Join-Path $RootDir "bin"
$RocksDbLib = Join-Path $SourceBin "librocksdb.so.8"
$needsEngineFetch = $false
if (-not (Test-Path $RocksDbLib)) {
    $needsEngineFetch = $true
} else {
    $rockItem = Get-Item $RocksDbLib -ErrorAction SilentlyContinue
    if ($rockItem -and $rockItem.Length -eq 0) {
        $needsEngineFetch = $true
    }
}

if ($needsEngineFetch) {
    Write-Host "-> Fetching official Sirius Linux engine binaries & shared libraries for WSL..." -ForegroundColor Yellow
    $TarUrl = "https://github.com/igorgoc/cpp-xpx-chain/releases/download/v$Version/sirius-linux-amd64.tar.gz"
    $TempTar = Join-Path $BuildDir "sirius-linux-amd64.tar.gz"
    try {
        if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
            & curl.exe -f -sSL $TarUrl -o $TempTar
        } else {
            Invoke-WebRequest -Uri $TarUrl -OutFile $TempTar -UseBasicParsing
        }
        if (Test-Path $TempTar) {
            if (Get-Command tar.exe -ErrorAction SilentlyContinue) {
                & tar.exe -xzf $TempTar -C $RootDir
            }
            Remove-Item -Force $TempTar -ErrorAction SilentlyContinue
        }
    } catch {
        Write-Warning "Could not pre-fetch Linux engine archive: $_"
    }
}

# Materialize shared library aliases (symlinks in Linux) as real files on NTFS so Windows packaging and extraction succeed
$SymlinkAliases = @{
    "librocksdb.so.8" = "librocksdb.so.8.5.3"
    "librocksdb.so" = "librocksdb.so.8.5.3"
    "libsnappy.so.1" = "libsnappy.so.1.1.8"
    "libsnappy.so" = "libsnappy.so.1.1.8"
    "libzstd.so.1" = "libzstd.so.1.4.8"
    "libzstd.so" = "libzstd.so.1.4.8"
    "libtorrent-sirius.so.2.0" = "libtorrent-sirius.so.2.0.4"
    "libtorrent-sirius.so" = "libtorrent-sirius.so.2.0.4"
    "libcrypto.so" = "libcrypto.so.3"
    "libssl.so" = "libssl.so.3"
}
Get-ChildItem -Path $SourceBin -Filter "libboost_*.so.1.81.0" -ErrorAction SilentlyContinue | ForEach-Object {
    $baseAlias = $_.Name -replace '\.1\.81\.0$', ''
    $SymlinkAliases[$baseAlias] = $_.Name
}

foreach ($alias in $SymlinkAliases.Keys) {
    $targetName = $SymlinkAliases[$alias]
    $aliasPath = Join-Path $SourceBin $alias
    $targetPath = Join-Path $SourceBin $targetName
    if (Test-Path $targetPath) {
        $needCopy = $false
        if (-not (Test-Path $aliasPath)) {
            $needCopy = $true
        } else {
            $item = Get-Item $aliasPath -ErrorAction SilentlyContinue
            if ($item.Length -eq 0 -or ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force $aliasPath -ErrorAction SilentlyContinue
                $needCopy = $true
            }
        }
        if ($needCopy) {
            Copy-Item $targetPath $aliasPath -Force -ErrorAction SilentlyContinue
        }
    }
}

if (Test-Path $SourceBin) {
    # Copy all real files (binaries, materialized .so libraries) into TargetBin, skipping directory trees like bin\linux or bin\macos and foreign macOS .dylib files
    Get-ChildItem -Path $SourceBin -File | Where-Object {
        $_.Length -gt 0 -and (-not ($_.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) -and
        $_.Name -notmatch '\.dylib$' -and $_.Name -ne '.DS_Store'
    } | Copy-Item -Destination $TargetBin -Force -ErrorAction SilentlyContinue

    # Ensure all symlink aliases are also mirrored into TargetBin
    foreach ($alias in $SymlinkAliases.Keys) {
        $targetName = $SymlinkAliases[$alias]
        $aliasInBin = Join-Path $TargetBin $alias
        $targetInBin = Join-Path $TargetBin $targetName
        if ((Test-Path $targetInBin) -and (-not (Test-Path $aliasInBin))) {
            Copy-Item $targetInBin $aliasInBin -Force -ErrorAction SilentlyContinue
        }
    }
}

# Copy engine, manager & snapshot compatibility manifests
Copy-Item (Join-Path $RootDir "chainconfig\engine.compat.json") (Join-Path $BuildDir "chainconfig\engine.compat.json")
if (Test-Path (Join-Path $RootDir "chainconfig\manager.compat.json")) {
    Copy-Item (Join-Path $RootDir "chainconfig\manager.compat.json") (Join-Path $BuildDir "chainconfig\manager.compat.json")
}
if (Test-Path (Join-Path $RootDir "chainconfig\snapshot.compat.json")) {
    Copy-Item (Join-Path $RootDir "chainconfig\snapshot.compat.json") (Join-Path $BuildDir "chainconfig\snapshot.compat.json")
}

# Copy resource files strictly without active private keys or sensitive state
Copy-Item -Recurse (Join-Path $RootDir "chainconfig\resources\*") $TargetResources
$SensitiveFiles = @("config-harvesting.properties", "config-storage.properties", "config-user.properties", "config-manager.properties", ".sirius-token", "harvest-stats.json")
foreach ($sf in $SensitiveFiles) {
    $p = Join-Path $TargetResources $sf
    if (Test-Path $p) { Remove-Item -Force $p }
}

# Populate clean non-template properties
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-harvesting.properties.template") (Join-Path $TargetResources "config-harvesting.properties")
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-storage.properties.template") (Join-Path $TargetResources "config-storage.properties")
Copy-Item (Join-Path $RootDir "chainconfig\resources\config-user.properties.template") (Join-Path $TargetResources "config-user.properties")
if (Test-Path (Join-Path $RootDir "chainconfig\resources\config-manager.properties.template")) {
    Copy-Item (Join-Path $RootDir "chainconfig\resources\config-manager.properties.template") (Join-Path $TargetResources "config-manager.properties")
}

# Remove all .template files from bundle package so only non-template configuration files remain
Get-ChildItem -Path $TargetResources -Filter "*.template" | Remove-Item -Force

# Genesis bootstrap data & seed package
$GenesisSeedDir = Join-Path $RootDir "chainconfig\genesis_seed"
$TargetGenesisSeed = Join-Path $BuildDir "chainconfig\genesis_seed"
if (Test-Path $GenesisSeedDir) {
    Copy-Item -Recurse $GenesisSeedDir $TargetGenesisSeed -Force
    if (Test-Path (Join-Path $GenesisSeedDir "00000\00001.dat")) {
        Copy-Item (Join-Path $GenesisSeedDir "00000\00001.dat") $TargetData -Force
        Copy-Item (Join-Path $GenesisSeedDir "00000\hashes.dat") $TargetData -Force
    }
    if (Test-Path (Join-Path $GenesisSeedDir "index.dat")) {
        Copy-Item (Join-Path $GenesisSeedDir "index.dat") (Join-Path $BuildDir "chainconfig\data\index.dat") -Force
    }
} elseif (Test-Path (Join-Path $RootDir "chainconfig\data\00000\00001.dat")) {
    Copy-Item (Join-Path $RootDir "chainconfig\data\00000\00001.dat") $TargetData -Force
    Copy-Item (Join-Path $RootDir "chainconfig\data\00000\hashes.dat") $TargetData -Force
}

# Ensure index.dat (height 1 genesis state) is present
$TargetIndex = Join-Path $BuildDir "chainconfig\data\index.dat"
if (-not (Test-Path $TargetIndex)) {
    [System.IO.File]::WriteAllBytes($TargetIndex, [byte[]]@(1, 0, 0, 0, 0, 0, 0, 0))
}

# Copy Windows launchers and helper scripts into scripts\windows
$WinScriptDir = Join-Path $RootDir "scripts\windows"
$BuildWinScripts = Join-Path $BuildDir "scripts\windows"
New-Item -ItemType Directory -Path $BuildWinScripts -Force | Out-Null
Copy-Item (Join-Path $WinScriptDir "*") $BuildWinScripts -Force
# Remove developer packaging script from the release bundle
Remove-Item -Force (Join-Path $BuildWinScripts "package-windows.ps1") -ErrorAction SilentlyContinue

# Stage ONLY top-level single entry launchers and README in release root
Copy-Item (Join-Path $WinScriptDir "start.bat") $BuildDir
Copy-Item (Join-Path $WinScriptDir "stop.bat") $BuildDir
if (Test-Path (Join-Path $WinScriptDir "README.md")) {
    Copy-Item (Join-Path $WinScriptDir "README.md") $BuildDir
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
