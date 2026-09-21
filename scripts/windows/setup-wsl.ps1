<#
.SYNOPSIS
    ProximaX Sirius Mainnet - Windows Subsystem for Linux (WSL2) Setup & Update Manager
.DESCRIPTION
    Configures Windows virtualization features, updates WSL2 to the latest kernel & MSI package,
    and installs the Sirius Linux distribution (Ubuntu-22.04 LTS).
    Runs in an elevated, persistent console window with detailed step-by-step progress logging.
#>

param(
    [ValidateSet("Update", "InstallDistro", "EnableWSL", "All")]
    [string]$Action = "Update",
    [string]$Distro = "Ubuntu-22.04",
    [string]$LogFile = ""
)

# Output encoding
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$host.UI.RawUI.WindowTitle = "ProximaX Sirius - WSL Subsystem Manager ($Action)"

# Self-Elevation: Ensure running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[!] Administrator permissions are required to configure Windows Subsystem for Linux." -ForegroundColor Yellow
    Write-Host "    Requesting UAC elevation..." -ForegroundColor Yellow
    $argList = @("-NoExit", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $PSCommandPath, "-Action", $Action, "-Distro", $Distro)
    if ($LogFile) { $argList += @("-LogFile", $LogFile) }
    Start-Process powershell.exe -ArgumentList $argList -Verb RunAs
    exit
}

# Resolve Log File
if (-not $LogFile) {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $LogFile = Join-Path $scriptDir "..\..\chainconfig\logs\wsl_install.log"
}
try {
    $logDir = Split-Path -Parent $LogFile
    if ($logDir -and (-not (Test-Path $logDir))) {
        New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    }
} catch {}

function Log-Message {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
    if ($LogFile) {
        $ts = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        Add-Content -Path $LogFile -Value "[$ts] $Message" -ErrorAction SilentlyContinue
    }
}

function Clean-WSLString {
    param([string]$InputStr)
    if (-not $InputStr) { return "" }
    return ($InputStr -replace '\0', '').Trim()
}

function Invoke-StepCommand {
    param(
        [string]$FilePath,
        [string[]]$ArgumentList,
        [string]$Description = ""
    )
    if ($Description) {
        Log-Message "-> $Description..." "Cyan"
    }
    Log-Message "   [Command] $FilePath $($ArgumentList -join ' ')" "DarkGray"

    # Properly escape arguments containing whitespace or quotes for Start-Process
    $escapedArgs = @()
    foreach ($arg in $ArgumentList) {
        if ($arg -match '[\s"]' -and -not ($arg.StartsWith('\"') -and $arg.EndsWith('\"')) -and -not ($arg.StartsWith('"') -and $arg.EndsWith('"'))) {
            $escaped = $arg -replace '"', '\"'
            $escapedArgs += ('"' + $escaped + '"')
        } else {
            $escapedArgs += $arg
        }
    }
    
    $proc = Start-Process -FilePath $FilePath -ArgumentList ($escapedArgs -join ' ') -NoNewWindow -Wait -PassThru
    $ec = $proc.ExitCode
    $color = if ($ec -eq 0 -or $ec -eq 3010) { "Green" } else { "Red" }
    Log-Message "   [Exit Code] $ec" $color
    return $ec
}

function Get-InstalledWSLVersion {
    try {
        $raw = & wsl.exe --version 2>&1 | Out-String
        $clean = Clean-WSLString $raw
        foreach ($line in ($clean -split '\r?\n')) {
            if ($line -match 'WSL\s*version:\s*([0-9\.]+)') {
                return $matches[1]
            }
        }
    } catch {}
    return ""
}

function Test-WSLOutdated {
    param([string]$ver)
    if (-not $ver) { return $true }
    $parts = $ver.Split('.')
    if ($parts.Length -eq 0) { return $true }
    $major = 0
    [int]::TryParse($parts[0], [ref]$major) | Out-Null
    if ($major -lt 2) { return $true }
    $minor = 0
    if ($parts.Length -gt 1) {
        [int]::TryParse($parts[1], [ref]$minor) | Out-Null
    }
    if ($major -eq 2 -and $minor -lt 3) { return $true }
    return $false
}

function Test-RebootPending {
    $cbs = Test-Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending"
    $wu = Test-Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired"
    return ($cbs -or $wu)
}

function Enable-WSLFeatures {
    Log-Message ""
    Log-Message "==========================================================================" "Cyan"
    Log-Message " STEP: Enabling Windows Optional Virtualization Features..." "Cyan"
    Log-Message "==========================================================================" "Cyan"

    # Enable Microsoft-Windows-Subsystem-Linux
    $c1 = Invoke-StepCommand -FilePath "dism.exe" -ArgumentList @("/online", "/enable-feature", "/featurename:Microsoft-Windows-Subsystem-Linux", "/all", "/norestart") -Description "Enabling Windows Subsystem for Linux"

    # Enable VirtualMachinePlatform
    $c2 = Invoke-StepCommand -FilePath "dism.exe" -ArgumentList @("/online", "/enable-feature", "/featurename:VirtualMachinePlatform", "/all", "/norestart") -Description "Enabling Virtual Machine Platform"

    if ($c1 -eq 3010 -or $c2 -eq 3010) {
        $script:RebootRequired = $true
        Log-Message ""
        Log-Message "==========================================================================" "Yellow"
        Log-Message "  [!] MANDATORY REBOOT REQUIRED" "Yellow"
        Log-Message "==========================================================================" "Yellow"
        Log-Message "  Windows has enabled virtualization features, but requires a system restart" "White"
        Log-Message "  to activate the Hyper-V / Virtual Machine Platform hypervisor." "White"
        Log-Message ""
        Log-Message "  Ubuntu cannot be installed or launched until your computer is restarted." "Yellow"
        Log-Message "  Please restart your computer now, then reopen start.bat." "Cyan"
        Log-Message "==========================================================================" "Yellow"
    }

    # Configure default version 2
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("--set-default-version", "2") -Description "Setting WSL default version to 2"
}

function Update-WSLSubsystem {
    Log-Message ""
    Log-Message "==========================================================================" "Cyan"
    Log-Message " STEP: Updating Windows Subsystem for Linux (WSL2)..." "Cyan"
    Log-Message "==========================================================================" "Cyan"

    $curVer = Get-InstalledWSLVersion
    if ($curVer) {
        Log-Message "-> Detected WSL version: $curVer" "White"
    } else {
        Log-Message "-> No modern WSL version reported by 'wsl.exe --version'." "Yellow"
    }

    # First attempt standard wsl.exe --update
    Log-Message "-> Running standard wsl.exe --update..." "Cyan"
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("--update") -Description "Checking Windows Update for WSL update"

    $afterVer = Get-InstalledWSLVersion
    $isOutdated = Test-WSLOutdated $afterVer

    if ($isOutdated) {
        Log-Message ""
        Log-Message "-> Windows Update WSL version ($afterVer) is older than required (>= 2.3.0)." "Yellow"
        Log-Message "-> Fetching latest official Microsoft WSL MSI package from GitHub..." "Cyan"

        $msiUrl = ""
        $tag = ""

        # Method 1: Query GitHub API
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/microsoft/WSL/releases/latest" -Headers @{"User-Agent"="PowerShell"} -TimeoutSec 10
            $asset = $rel.assets | Where-Object { $_.name -like "*x64.msi" } | Select-Object -First 1
            if ($asset) {
                $msiUrl = $asset.browser_download_url
                $tag = $rel.tag_name
            }
        } catch {
            Log-Message "   Notice: GitHub API check: $_" "DarkGray"
        }

        # Method 2: GitHub Releases latest redirect
        if (-not $msiUrl) {
            try {
                $req = [System.Net.HttpWebRequest]::Create("https://github.com/microsoft/WSL/releases/latest")
                $req.AllowAutoRedirect = $false
                $req.UserAgent = "Mozilla/5.0"
                $req.Timeout = 10000
                $resp = $req.GetResponse()
                $loc = $resp.GetResponseHeader("Location")
                $resp.Close()
                if ($loc) {
                    $tag = $loc.Split('/')[-1]
                    $cleanTag = $tag -replace '^v',''
                    $msiUrl = "https://github.com/microsoft/WSL/releases/download/$tag/wsl.$cleanTag.0.x64.msi"
                }
            } catch {
                Log-Message "   Notice: GitHub redirect check: $_" "DarkGray"
            }
        }

        $tempMsi = Join-Path $env:TEMP "wsl-package-x64.msi"
        if (Test-Path $tempMsi) { Remove-Item -Force $tempMsi -ErrorAction SilentlyContinue }
        $downloadSuccess = $false

        if ($msiUrl) {
            Log-Message "-> Downloading Microsoft WSL $tag ($msiUrl)..." "Green"
            Log-Message "   Package size is ~240 MB. Please wait..." "Yellow"
            try {
                Start-BitsTransfer -Source $msiUrl -Destination $tempMsi -DisplayName "Downloading Microsoft WSL" -ErrorAction Stop
                if (Test-Path $tempMsi) {
                    $sizeMB = [math]::Round((Get-Item $tempMsi).Length / 1MB, 2)
                    if ($sizeMB -gt 10) {
                        Log-Message "-> Downloaded $sizeMB MB successfully." "Green"
                        $downloadSuccess = $true
                    }
                }
            } catch {
                Log-Message "   BITS download failed ($_); falling back to WebClient..." "Yellow"
                try {
                    $wc = New-Object System.Net.WebClient
                    $wc.Headers.Add("User-Agent", "Mozilla/5.0")
                    $wc.DownloadFile($msiUrl, $tempMsi)
                    if (Test-Path $tempMsi) {
                        $sizeMB = [math]::Round((Get-Item $tempMsi).Length / 1MB, 2)
                        if ($sizeMB -gt 10) {
                            Log-Message "-> Downloaded $sizeMB MB successfully." "Green"
                            $downloadSuccess = $true
                        }
                    }
                } catch {
                    Log-Message "   WebClient download error: $_" "Red"
                }
            }
        }

        if (-not $downloadSuccess) {
            # Method 3: Fallback to Microsoft Kernel update blob
            $fallbackUrl = "https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi"
            Log-Message "-> Falling back to Microsoft WSL Kernel Update: $fallbackUrl" "Yellow"
            try {
                $wc = New-Object System.Net.WebClient
                $wc.Headers.Add("User-Agent", "Mozilla/5.0")
                $wc.DownloadFile($fallbackUrl, $tempMsi)
                if (Test-Path $tempMsi) {
                    $sizeMB = [math]::Round((Get-Item $tempMsi).Length / 1MB, 2)
                    if ($sizeMB -gt 5) {
                        Log-Message "-> Downloaded fallback kernel ($sizeMB MB) successfully." "Green"
                        $downloadSuccess = $true
                    }
                }
            } catch {
                Log-Message "   Fallback download error: $_" "Red"
            }
        }

        if ($downloadSuccess) {
            Log-Message "-> Installing WSL MSI package via msiexec (quiet mode)..." "Cyan"
            $msiCode = Invoke-StepCommand -FilePath "msiexec.exe" -ArgumentList @("/i", $tempMsi, "/qn", "/norestart") -Description "Installing WSL MSI package"
            Remove-Item -Force $tempMsi -ErrorAction SilentlyContinue

            Start-Sleep -Seconds 2
            $finalVer = Get-InstalledWSLVersion
            if ($finalVer) {
                Log-Message "-> [OK] WSL is now upgraded to version: $finalVer" "Green"
            } else {
                Log-Message "-> WSL MSI installation completed (exit code: $msiCode)." "Green"
            }
        } else {
            Log-Message "-> [!] Could not download WSL MSI package. Check internet connection to github.com." "Red"
        }
    } else {
        Log-Message "-> [OK] WSL is already running modern release: $afterVer" "Green"
    }

    # Ensure default version 2
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("--set-default-version", "2") -Description "Setting WSL default version to 2"

    # If an Ubuntu distribution is already installed, verify runtime dependencies as well
    $listRaw = & wsl.exe -l -v 2>&1 | Out-String
    $cleanList = Clean-WSLString $listRaw
    foreach ($line in ($cleanList -split '\r?\n')) {
        $trimmed = $line.Trim()
        if ($trimmed -match '(Ubuntu[A-Za-z0-9\._\-]*)') {
            $existingDistro = $matches[1]
            Ensure-DistroRuntimePackages -targetDistroName $existingDistro
            break
        }
    }
}

function Ensure-DistroRuntimePackages {
    param([string]$targetDistroName)
    if (-not $targetDistroName) { return }

    Log-Message ""
    Log-Message "==========================================================================" "Cyan"
    Log-Message " STEP: Configuring Sirius Engine Runtime Packages in $targetDistroName..." "Cyan"
    Log-Message "==========================================================================" "Cyan"

    Log-Message "-> Updating Ubuntu package lists (apt-get update)..." "Yellow"
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "update", "-qq") -Description "Updating package lists"

    Log-Message "-> Upgrading Ubuntu system packages (apt-get upgrade)..." "Yellow"
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "upgrade", "-y", "-qq") -Description "Upgrading system packages"

    Log-Message "-> Installing libatomic1 and essential tools (ca-certificates, curl, tar)..." "Yellow"
    $instCode = Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", "-qq", "libatomic1", "ca-certificates", "curl", "tar") -Description "Installing libatomic1 runtime"

    # Verify libatomic1 is installed
    $checkAtomic = Start-Process -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "dpkg", "-s", "libatomic1") -NoNewWindow -Wait -PassThru
    if ($checkAtomic.ExitCode -eq 0) {
        Log-Message "-> [OK] Sirius runtime packages successfully installed and verified in $targetDistroName!" "Green"
    } else {
        # Retry with explicit apt-get install
        Log-Message "-> Retrying libatomic1 installation..." "Yellow"
        Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "apt-get", "install", "-y", "libatomic1") -Description "Retrying libatomic1"
        $checkAtomic = Start-Process -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "dpkg", "-s", "libatomic1") -NoNewWindow -Wait -PassThru
        if ($checkAtomic.ExitCode -eq 0) {
            Log-Message "-> [OK] Sirius runtime packages successfully installed and verified in $targetDistroName!" "Green"
        } else {
            Log-Message "-> [!] Warning: Failed to install libatomic1 (exit code: $($checkAtomic.ExitCode))." "Yellow"
        }
    }

    # Initialize native ext4 storage directory for high-speed blockchain consensus
    Log-Message "-> Initializing native ext4 blockchain storage (/var/lib/sirius/data)..." "Yellow"
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "mkdir", "-p", "/var/lib/sirius/data/00000") -Description "Creating /var/lib/sirius/data"
    Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("-d", $targetDistroName, "-u", "root", "--", "chmod", "-R", "777", "/var/lib/sirius") -Description "Configuring permissions"

    # Seed genesis block into native ext4 storage if available from project
    $wslDataDir = "\\wsl.localhost\$targetDistroName\var\lib\sirius\data"
    $scriptDir = Split-Path -Parent $PSCommandPath
    $genesisSrc = Join-Path $scriptDir "..\..\chainconfig\data\00000"
    if (-not (Test-Path (Join-Path $genesisSrc "00001.dat"))) {
        $genesisSrc = Join-Path $scriptDir "..\..\chainconfig\genesis_seed\00000"
    }
    if (Test-Path (Join-Path $genesisSrc "00001.dat")) {
        $dest00000 = Join-Path $wslDataDir "00000"
        if (-not (Test-Path (Join-Path $dest00000 "00001.dat"))) {
            Copy-Item (Join-Path $genesisSrc "00001.dat") (Join-Path $dest00000 "00001.dat") -Force -ErrorAction SilentlyContinue
            Copy-Item (Join-Path $genesisSrc "hashes.dat") (Join-Path $dest00000 "hashes.dat") -Force -ErrorAction SilentlyContinue
            Log-Message "-> [OK] Seeded genesis block (00001.dat & hashes.dat) into native ext4 storage!" "Green"
        }
        $destIndex = Join-Path $wslDataDir "index.dat"
        if (-not (Test-Path $destIndex)) {
            [System.IO.File]::WriteAllBytes($destIndex, [byte[]]@(1,0,0,0,0,0,0,0))
        }
    }
}

function Install-WSLDistro {
    param(
        [string]$targetDistro = "Ubuntu-22.04"
    )

    if ($script:RebootRequired -or (Test-RebootPending)) {
        Log-Message ""
        Log-Message "==========================================================================" "Red"
        Log-Message "  [!] SYSTEM RESTART MANDATORY BEFORE INSTALLING UBUNTU" "Red"
        Log-Message "==========================================================================" "Red"
        Log-Message "  Windows virtualization features were recently enabled or updated." "White"
        Log-Message "  The WSL2 hypervisor cannot start until your computer is restarted." "White"
        Log-Message ""
        Log-Message "  Please restart your computer now, then reopen start.bat." "Cyan"
        Log-Message "==========================================================================" "Red"
        return
    }

    Log-Message ""
    Log-Message "==========================================================================" "Cyan"
    Log-Message " STEP: Installing Sirius Linux Distribution ($targetDistro)..." "Cyan"
    Log-Message "==========================================================================" "Cyan"

    # Query currently installed distributions
    $listRaw = & wsl.exe -l -v 2>&1 | Out-String
    $cleanList = Clean-WSLString $listRaw
    $alreadyInstalled = $false
    $installedDistroName = ""

    foreach ($line in ($cleanList -split '\r?\n')) {
        $trimmed = $line.Trim()
        if ($trimmed -and -not ($trimmed -like "---*") -and -not ($trimmed -like "NAME*")) {
            $parts = $trimmed -split '\s+'
            if ($parts.Length -gt 0) {
                $name = $parts[0]
                if ($name -eq "*" -and $parts.Length -gt 1) { $name = $parts[1] }
                if ($name -ieq $targetDistro -or ($targetDistro -ieq "Ubuntu-22.04" -and $name -ieq "Ubuntu")) {
                    $alreadyInstalled = $true
                    $installedDistroName = $name
                    break
                }
            }
        }
    }

    if ($alreadyInstalled) {
        Log-Message "-> Distribution '$installedDistroName' is already installed!" "Green"
    } else {
        # Check online catalog
        Log-Message "-> Checking available distributions in online catalog..." "Cyan"
        $onlineRaw = & wsl.exe --list --online 2>&1 | Out-String
        $cleanOnline = Clean-WSLString $onlineRaw

        $distroToInstall = $targetDistro
        $hasExact = $false
        $hasGenericUbuntu = $false

        foreach ($line in ($cleanOnline -split '\r?\n')) {
            $trimmed = $line.Trim()
            if ($trimmed -match '^\s*([A-Za-z0-9\._\-]+)') {
                $cand = $matches[1]
                if ($cand -ieq $targetDistro) { $hasExact = $true }
                if ($cand -ieq "Ubuntu") { $hasGenericUbuntu = $true }
            }
        }

        if (-not $hasExact -and $targetDistro -ieq "Ubuntu-22.04" -and $hasGenericUbuntu) {
            Log-Message "-> 'Ubuntu-22.04' not listed in online catalog. Selecting 'Ubuntu' LTS fallback..." "Yellow"
            $distroToInstall = "Ubuntu"
        }

        Log-Message "-> Installing $distroToInstall from Microsoft Store / Canonical CDN..." "Green"
        Log-Message "   Downloading distribution package (~500 MB). Please keep this window open..." "Yellow"

        $instCode = Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("--install", "-d", $distroToInstall, "--no-launch") -Description "Installing $distroToInstall"

        # Check if Windows reported that a reboot is mandatory
        $logTail = ""
        if ($LogFile -and (Test-Path $LogFile)) {
            $logTail = (Get-Content -Path $LogFile -Tail 10 -ErrorAction SilentlyContinue | Out-String)
        }
        $wslStatusRaw = & wsl.exe --status 2>&1 | Out-String
        $isRebootMandatory = ($logTail -like "*Changes will not be effective until the system is rebooted*") -or
                             ($logTail -like "*system is rebooted*") -or
                             ($wslStatusRaw -like "*Changes will not be effective until the system is rebooted*") -or
                             ($wslStatusRaw -like "*virtualization is not enabled on this machine*")

        if ($isRebootMandatory -or (Test-RebootPending)) {
            Log-Message ""
            Log-Message "==========================================================================" "Yellow"
            Log-Message "  [!] SYSTEM RESTART MANDATORY BEFORE CONFIGURING UBUNTU" "Yellow"
            Log-Message "==========================================================================" "Yellow"
            Log-Message "  Windows reported: 'Changes will not be effective until the system is rebooted.'" "White"
            Log-Message "  Ubuntu-22.04 has been staged, but Windows cannot initialize it until reboot." "White"
            Log-Message ""
            Log-Message "  Please restart your computer now, then reopen start.bat." "Cyan"
            Log-Message "==========================================================================" "Yellow"
            return
        }

        if ($instCode -eq 0) {
            Log-Message "-> [OK] Distribution $distroToInstall installed successfully!" "Green"
            $installedDistroName = $distroToInstall
        } else {
            Log-Message "-> [!] Distribution installation exited with code $instCode." "Red"
        }
    }

    # Ensure distribution is on WSL version 2
    if ($installedDistroName) {
        $isV2 = $false
        $listRaw = & wsl.exe -l -v 2>&1 | Out-String
        $cleanList = Clean-WSLString $listRaw
        foreach ($line in ($cleanList -split '\r?\n')) {
            $trimmed = $line.Trim()
            if ($trimmed -match "^\*?\s*$([regex]::Escape($installedDistroName))\s+\w+\s+(\d+)") {
                if ($matches[1] -eq "2") {
                    $isV2 = $true
                    break
                }
            }
        }

        if ($isV2) {
            Log-Message "-> [OK] Distribution '$installedDistroName' is already running on WSL2." "Green"
        } else {
            $verCode = Invoke-StepCommand -FilePath "wsl.exe" -ArgumentList @("--set-version", $installedDistroName, "2") -Description "Setting WSL2 version for $installedDistroName"
            if ($verCode -ne 0) {
                # WSL CLI returns -1 / WSL_E_VM_MODE_INVALID_STATE if it is already version 2
                Log-Message "   Note: If distribution was already on WSL2, exit code -1 is normal." "Gray"
            }
        }

        # Update package lists and install essential runtime dependencies
        Ensure-DistroRuntimePackages -targetDistroName $installedDistroName
    }

    # List current installed distributions
    $finalList = & wsl.exe -l -v 2>&1 | Out-String
    Log-Message ""
    Log-Message "-> Installed WSL Distributions:" "White"
    Log-Message (Clean-WSLString $finalList) "DarkGray"
}

# Main Execution Flow
Log-Message "==========================================================================" "Cyan"
Log-Message "        PROXIMAX SIRIUS CORE - WSL2 SUBSYSTEM MANAGER" "Cyan"
Log-Message "==========================================================================" "Cyan"
Log-Message " Action: $Action | Distro: $Distro" "White"
Log-Message " Log: $LogFile" "DarkGray"
Log-Message " Window will remain open when finished so you can inspect results." "DarkGray"
Log-Message "==========================================================================" "Cyan"

switch ($Action) {
    "EnableWSL" {
        Enable-WSLFeatures
    }
    "Update" {
        Update-WSLSubsystem
    }
    "InstallDistro" {
        Install-WSLDistro -targetDistro $Distro
    }
    "All" {
        Enable-WSLFeatures
        Update-WSLSubsystem
        Install-WSLDistro -targetDistro $Distro
    }
}

Log-Message ""
Log-Message "==========================================================================" "Cyan"
Log-Message "  Setup operations completed!" "Green"
Log-Message "  You can now return to the ProximaX Sirius Cockpit in your browser." "Green"
Log-Message "==========================================================================" "Cyan"
Log-Message "Press Enter to close this window (or close it manually at any time)..." "White"
Read-Host
