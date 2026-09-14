//go:build windows

package supervisor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type wslCache struct {
	WSLInstallInitiated bool   `json:"wsl_install_initiated,omitempty"`
	PreferredDistro     string `json:"preferred_distro,omitempty"`
}

func (dc *ProcessSupervisor) getCacheFilePath() string {
	return filepath.Join(dc.chainConfigPath, "cache.json")
}

func (dc *ProcessSupervisor) readCache() wslCache {
	cachePath := dc.getCacheFilePath()
	var cache wslCache
	data, err := os.ReadFile(cachePath)
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}

func (dc *ProcessSupervisor) writeCache(cache wslCache) {
	cachePath := dc.getCacheFilePath()
	data, err := json.MarshalIndent(cache, "", "  ")
	if err == nil {
		_ = os.WriteFile(cachePath, data, 0644)
	}
}

// ToWSLPath converts a Windows host path (e.g. C:\Project\chainconfig or .\chainconfig\data) into a WSL path (/mnt/c/Project/chainconfig)
func ToWSLPath(winPath string) string {
	if winPath == "" {
		return ""
	}
	// Already a WSL / Unix path
	if strings.HasPrefix(winPath, "/mnt/") || (strings.HasPrefix(winPath, "/") && !strings.Contains(winPath, ":")) {
		return winPath
	}
	absPath, err := filepath.Abs(winPath)
	if err == nil {
		winPath = absPath
	}
	clean := filepath.Clean(winPath)
	vol := filepath.VolumeName(clean)
	rest := clean[len(vol):]
	rest = filepath.ToSlash(rest)
	if len(vol) >= 2 && vol[1] == ':' {
		driveLetter := strings.ToLower(string(vol[0]))
		return fmt.Sprintf("/mnt/%s%s", driveLetter, rest)
	}
	return filepath.ToSlash(clean)
}

// FromWSLPath converts a WSL path (/mnt/c/Sirius_data) into a Windows host path (C:\Sirius_data)
func FromWSLPath(wslPath string) string {
	if wslPath == "" {
		return ""
	}
	clean := filepath.ToSlash(strings.TrimSpace(wslPath))
	if strings.HasPrefix(clean, "/mnt/") && len(clean) >= 7 && (len(clean) == 7 || clean[6] == '/') {
		driveLetter := strings.ToUpper(string(clean[5]))
		rest := clean[6:]
		rest = strings.ReplaceAll(rest, "/", "\\")
		return fmt.Sprintf("%s:%s", driveLetter, rest)
	}
	return filepath.FromSlash(wslPath)
}

func extractExitHex(err error) string {
	if err == nil {
		return ""
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Sprintf("0x%08x", uint32(exitErr.ExitCode()))
	}
	return ""
}

func isLinuxELF(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	var header [4]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return false
	}
	return header[0] == 0x7F && header[1] == 'E' && header[2] == 'L' && header[3] == 'F'
}

func (dc *ProcessSupervisor) getWSLEnginePid() int {
	status := dc.ProbeWSLStatus()
	if status.State != WSL2Ready {
		return 0
	}
	distro := status.DistroName
	if distro == "" {
		distro = "Ubuntu-22.04"
	}
	out, err := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pgrep", "-f", "sirius.bc").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(cleanWSLOutput(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		pid, _ := strconv.Atoi(strings.TrimSpace(lines[0]))
		return pid
	}
	return 0
}

func (dc *ProcessSupervisor) isWSLEngineRunning() bool {
	return dc.getWSLEnginePid() > 0
}

func parseWSLVersionInfo(output string) (wslVer string, kernelVer string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "wsl") && strings.Contains(lineLower, "version") && !strings.Contains(lineLower, "wslg") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				wslVer = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(lineLower, "kernel") && strings.Contains(lineLower, "version") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				kernelVer = strings.TrimSpace(parts[1])
			}
		}
	}
	return wslVer, kernelVer
}

func isWSLVersionOutdated(verStr string) bool {
	if verStr == "" {
		return true
	}
	parts := strings.Split(verStr, ".")
	if len(parts) == 0 {
		return true
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return true
	}
	if major < 2 {
		return true
	}
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if major == 2 && minor < 3 {
		return true
	}
	return false
}

func (dc *ProcessSupervisor) getOnlineDistrosCached(wslExe string) []string {
	dc.wslCacheMu.RLock()
	if time.Since(dc.wslOnlineDistrosTime) < 60*time.Second && len(dc.wslOnlineDistros) > 0 {
		cached := dc.wslOnlineDistros
		dc.wslCacheMu.RUnlock()
		return cached
	}
	dc.wslCacheMu.RUnlock()

	distros := dc.queryOnlineDistros(wslExe)
	if len(distros) > 0 {
		dc.wslCacheMu.Lock()
		dc.wslOnlineDistros = distros
		dc.wslOnlineDistrosTime = time.Now()
		dc.wslCacheMu.Unlock()
	}
	return distros
}

func (dc *ProcessSupervisor) queryOnlineDistros(wslExe string) []string {
	cmd := exec.Command(wslExe, "--list", "--online")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	cleaned := cleanWSLOutput(out)
	lines := strings.Split(cleaned, "\n")
	var distros []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "NAME") || strings.Contains(upper, "VALID DISTRIBUTIONS") || strings.Contains(upper, "INSTALL USING") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 {
			name := fields[0]
			if !strings.HasPrefix(name, "-") && !strings.EqualFold(name, "NAME") {
				distros = append(distros, name)
			}
		}
	}
	return distros
}

func (dc *ProcessSupervisor) ProbeWSLStatus() (status WSLStatus) {
	dc.wslCacheMu.RLock()
	if time.Since(dc.wslCacheTime) < 5*time.Second && dc.wslCachedStatus.State == WSL2Ready {
		cached := dc.wslCachedStatus
		dc.wslCacheMu.RUnlock()
		return cached
	}
	dc.wslCacheMu.RUnlock()

	defer func() {
		dc.wslCacheMu.Lock()
		dc.wslCachedStatus = status
		dc.wslCacheTime = time.Now()
		dc.wslCacheMu.Unlock()
	}()

	status = WSLStatus{
		IsWindows: true,
		State:     WSLNotInstalled,
	}

	cache := dc.readCache()
	status.WSLInstallInitiated = cache.WSLInstallInitiated

	// 1. Check if wsl.exe is available in PATH or System32
	wslExe, err := exec.LookPath("wsl.exe")
	if err != nil {
		sysWsl := filepath.Join(os.Getenv("SystemRoot"), "System32", "wsl.exe")
		if _, e := os.Stat(sysWsl); e == nil {
			wslExe = sysWsl
		} else {
			status.State = WSLNotInstalled
			status.ErrorMessage = "wsl.exe not found on system"
			return status
		}
	}

	// 2. Query wsl.exe --version to inspect installed WSL version
	cmdVer := exec.Command(wslExe, "--version")
	outVer, errVer := cmdVer.CombinedOutput()
	if errVer == nil {
		cleanedVer := cleanWSLOutput(outVer)
		wslVer, kernelVer := parseWSLVersionInfo(cleanedVer)
		status.WSLVersion = wslVer
		status.KernelVersion = kernelVer
		status.IsOutdated = isWSLVersionOutdated(wslVer)
	} else {
		status.IsOutdated = true
	}

	// 3. Check recent installation logs
	installLogPath := filepath.Join(dc.chainConfigPath, "logs", "wsl_install.log")
	if logData, err := os.ReadFile(installLogPath); err == nil && len(logData) > 0 {
		cleanedLog := cleanWSLOutput(logData)
		logLines := strings.Split(strings.TrimSpace(cleanedLog), "\n")
		startIdx := 0
		if len(logLines) > 8 {
			startIdx = len(logLines) - 8
		}
		status.InstallLog = strings.Join(logLines[startIdx:], "\n")
	}

	// 4. Query wsl.exe --status
	cmdStatus := exec.Command(wslExe, "--status")
	outStatus, errStatus := cmdStatus.CombinedOutput()
	cleanedStatus := cleanWSLOutput(outStatus)
	status.RawStatus = cleanedStatus

	statusCombined := cleanedStatus + " " + extractExitHex(errStatus) + " " + status.InstallLog
	statusLower := strings.ToLower(statusCombined)

	// Check for known error codes / HRESULTs first (locale-independent)
	// 0x80370102: WSL_E_VIRTUAL_MACHINE_PREREQUISITE_MINIMUM (BIOS Virtualization disabled or hypervisor not active)
	if strings.Contains(statusLower, "0x80370102") ||
		strings.Contains(statusLower, "80370102") ||
		strings.Contains(cleanedStatus, "Wsl/Service/CreateVm") ||
		strings.Contains(cleanedStatus, "CreateVm") {
		status.State = WSLNotInstalled
		status.ErrorCode = "BIOS_VIRTUALIZATION_DISABLED"
		status.ErrorMessage = "Hardware Virtualization is disabled in your computer's BIOS/UEFI, or a system restart is required to initialize the Virtual Machine Platform."
		return status
	}

	// 0x80072ee7: Network timeout / Microsoft Store or CDN unreachable
	if strings.Contains(statusLower, "0x80072ee7") || strings.Contains(statusLower, "80072ee7") {
		status.State = WSLNotInstalled
		status.ErrorCode = "NETWORK_TIMEOUT"
		status.ErrorMessage = "Network connection to Microsoft Store / WSL CDN timed out. Check your internet connection."
		return status
	}

	// 0x8024500c: Windows Update / Store blocked by Group Policy
	if strings.Contains(statusLower, "0x8024500c") || strings.Contains(statusLower, "8024500c") {
		status.State = WSLNotInstalled
		status.ErrorCode = "GROUP_POLICY_BLOCKED"
		status.ErrorMessage = "Microsoft Store / Windows Update is blocked by Group Policy in your organization."
		return status
	}

	// Locale-independent guard: If wsl.exe --status fails with non-zero exit, WSL is NOT ready/installed!
	if errStatus != nil {
		status.State = WSLNotInstalled
		status.ErrorCode = "WSL_NOT_INSTALLED"
		status.ErrorMessage = "Windows Subsystem for Linux (WSL2) optional component is not enabled on this system."
		return status
	}

	// Determine default WSL version
	status.DefaultVersion = 2
	for _, line := range strings.Split(cleanedStatus, "\n") {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		if (strings.Contains(lineLower, "version") || strings.Contains(lineLower, "версия") || strings.Contains(lineLower, "版本")) &&
			strings.Contains(lineLower, "1") && !strings.Contains(lineLower, "2") {
			status.DefaultVersion = 1
			break
		}
	}

	// 5. Query installed distributions with wsl.exe -l -v
	cmdList := exec.Command(wslExe, "-l", "-v")
	outList, errList := cmdList.CombinedOutput()
	cleanedList := cleanWSLOutput(outList)

	listCombined := cleanedList + " " + extractExitHex(errList)
	listLower := strings.ToLower(listCombined)

	if strings.Contains(listLower, "0x80370102") || strings.Contains(listLower, "80370102") {
		status.State = WSLNotInstalled
		status.ErrorCode = "BIOS_VIRTUALIZATION_DISABLED"
		status.ErrorMessage = "Hardware Virtualization is disabled in your computer's BIOS/UEFI, or a system restart is required."
		return status
	}
	if strings.Contains(listLower, "0x80072ee7") || strings.Contains(listLower, "80072ee7") {
		status.State = WSLNotInstalled
		status.ErrorCode = "NETWORK_TIMEOUT"
		status.ErrorMessage = "Network connection to Microsoft Store / WSL CDN timed out."
		return status
	}
	if strings.Contains(listLower, "0x8024500c") || strings.Contains(listLower, "8024500c") {
		status.State = WSLNotInstalled
		status.ErrorCode = "GROUP_POLICY_BLOCKED"
		status.ErrorMessage = "Microsoft Store / Windows Update is blocked by Group Policy."
		return status
	}

	// Parse distribution names and versions from `wsl.exe -l -v`
	lines := strings.Split(cleanedList, "\n")
	var selectedDistro string
	var selectedVersion int = 2

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "----") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}
		verStr := fields[len(fields)-1]
		if verStr != "1" && verStr != "2" {
			continue
		}

		ver := 2
		if verStr == "1" {
			ver = 1
		}

		name := fields[0]
		isDefault := false
		if name == "*" && len(fields) >= 4 {
			isDefault = true
			name = fields[1]
		}

		if isDefault || selectedDistro == "" || strings.Contains(strings.ToLower(name), "ubuntu") {
			selectedDistro = name
			selectedVersion = ver
		}
	}

	if errList != nil || selectedDistro == "" {
		if status.DefaultVersion == 1 {
			status.State = WSLV1Only
			status.ErrorMessage = "WSL1 is enabled, but WSL2 is required for Sirius ext4/RocksDB performance."
		} else {
			status.State = WSL2NoDistro
			status.ErrorMessage = "WSL2 kernel is ready, but no Sirius Linux distribution is installed."
		}

		// When no distro is installed, query online distros to check if Ubuntu-22.04 is available
		onlineDistros := dc.getOnlineDistrosCached(wslExe)
		status.OnlineDistros = onlineDistros
		if len(onlineDistros) > 0 {
			hasUbuntu2204 := false
			for _, d := range onlineDistros {
				if strings.EqualFold(d, "Ubuntu-22.04") {
					hasUbuntu2204 = true
					break
				}
			}
			if !hasUbuntu2204 {
				status.IsOutdated = true
			}
		}

		// Check if recent install log reported distro not found
		if status.InstallLog != "" {
			logLower := strings.ToLower(status.InstallLog)
			if strings.Contains(logLower, "not found") || strings.Contains(logLower, "keine verteilung") || strings.Contains(logLower, "introuvable") {
				status.ErrorCode = "DISTRO_NOT_FOUND"
				status.ErrorMessage = "Distribution 'Ubuntu-22.04' was not found in your WSL distribution catalog. Updating WSL is recommended."
			}
		}

		return status
	}

	status.DistroName = selectedDistro
	status.DefaultVersion = selectedVersion

	if selectedVersion == 1 {
		status.State = WSLV1Only
		status.ErrorMessage = fmt.Sprintf("Distribution %s is running WSL version 1. WSL2 is required.", selectedDistro)
		return status
	}

	// WSL2 is ready with a valid distribution!
	status.State = WSL2Ready

	// Reboot-Resume Invariant: If WSL install was initiated and is now ready, automatically clear marker!
	if cache.WSLInstallInitiated {
		cache.WSLInstallInitiated = false
		cache.PreferredDistro = selectedDistro
		dc.writeCache(cache)
		status.WSLInstallInitiated = false
	}

	return status
}

func (dc *ProcessSupervisor) InitiateWSLInstall() error {
	logDir := filepath.Join(dc.chainConfigPath, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "wsl_install.log")

	// Launch elevated PowerShell command to enable WSL2 without immediate distribution
	psScript := fmt.Sprintf(`$ErrorActionPreference = 'Continue'; Write-Host '=== ProximaX Sirius - Enabling Windows Subsystem for Linux (WSL2) ===' -ForegroundColor Cyan; wsl.exe --install --no-distribution *>&1 | Tee-Object -FilePath '%s'; if ($LASTEXITCODE -ne 0) { Write-Host 'WSL enable encountered an error. Exit code:' $LASTEXITCODE -ForegroundColor Red; Write-Host 'Press any key to close this window...' -ForegroundColor Yellow; $host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown') }`, logPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Start-Process powershell -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', \"%s\" -Verb RunAs", strings.ReplaceAll(psScript, `"`, `\"`)))

	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out) + " " + extractExitHex(err)
		outLower := strings.ToLower(outStr)
		if strings.Contains(outStr, "1223") || strings.Contains(outLower, "canceled by the user") || strings.Contains(outLower, "access is denied") {
			return fmt.Errorf("UAC_DENIED: Administrator permissions were declined. ProximaX Sirius requires permission once to enable the Windows virtualization feature.")
		}
		return fmt.Errorf("failed to initiate WSL install: %v (%s)", err, strings.TrimSpace(outStr))
	}

	// Persist local state marker in chainconfig/cache.json
	cache := dc.readCache()
	cache.WSLInstallInitiated = true
	dc.writeCache(cache)

	dc.wslCacheMu.Lock()
	dc.wslCacheTime = time.Time{}
	dc.wslCacheMu.Unlock()

	return nil
}

func (dc *ProcessSupervisor) UpdateWSL() error {
	logDir := filepath.Join(dc.chainConfigPath, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "wsl_install.log")

	// Invalidate cache
	dc.wslCacheMu.Lock()
	dc.wslCacheTime = time.Time{}
	dc.wslOnlineDistrosTime = time.Time{}
	dc.wslCacheMu.Unlock()

	psScript := fmt.Sprintf(`$ErrorActionPreference = 'Continue'; Write-Host '=== ProximaX Sirius - Updating Windows Subsystem for Linux (WSL2) ===' -ForegroundColor Cyan; wsl.exe --update --web-download *>&1 | Tee-Object -FilePath '%s'; if ($LASTEXITCODE -ne 0) { wsl.exe --update *>&1 | Tee-Object -FilePath '%s' -Append }; if ($LASTEXITCODE -ne 0) { Write-Host 'WSL Update encountered an error. Exit code:' $LASTEXITCODE -ForegroundColor Red; Write-Host 'Press any key to close this window...' -ForegroundColor Yellow; $host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown') }`, logPath, logPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Start-Process powershell -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', \"%s\" -Verb RunAs", strings.ReplaceAll(psScript, `"`, `\"`)))

	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out) + " " + extractExitHex(err)
		outLower := strings.ToLower(outStr)
		if strings.Contains(outStr, "1223") || strings.Contains(outLower, "canceled by the user") || strings.Contains(outLower, "access is denied") {
			return fmt.Errorf("UAC_DENIED: Administrator permissions were declined.")
		}
		return fmt.Errorf("failed to initiate WSL update: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (dc *ProcessSupervisor) SetupWSLDistro(distroName string) error {
	logDir := filepath.Join(dc.chainConfigPath, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "wsl_install.log")

	// Invalidate cache
	dc.wslCacheMu.Lock()
	dc.wslCacheTime = time.Time{}
	dc.wslCacheMu.Unlock()

	// 1. Ensure WSL2 default version
	_ = exec.Command("wsl.exe", "--set-default-version", "2").Run()

	wslExe, _ := exec.LookPath("wsl.exe")
	if wslExe == "" {
		wslExe = filepath.Join(os.Getenv("SystemRoot"), "System32", "wsl.exe")
	}

	targetDistro := distroName
	if targetDistro == "" {
		targetDistro = "Ubuntu-22.04"
	}

	// 2. Query online distros to resolve exact catalog name
	onlineDistros := dc.getOnlineDistrosCached(wslExe)
	hasTarget := false
	hasGenericUbuntu := false
	for _, d := range onlineDistros {
		if strings.EqualFold(d, targetDistro) {
			hasTarget = true
		}
		if strings.EqualFold(d, "Ubuntu") {
			hasGenericUbuntu = true
		}
	}

	// If Ubuntu-22.04 requested but not found in catalog, and generic Ubuntu is available, fallback to Ubuntu
	if !hasTarget && len(onlineDistros) > 0 {
		if strings.Contains(strings.ToLower(targetDistro), "ubuntu") && hasGenericUbuntu {
			targetDistro = "Ubuntu"
		}
	}

	// 3. Launch installation with log capture and error pause so it never silently vanishes
	psScript := fmt.Sprintf(`$ErrorActionPreference = 'Continue'; Write-Host '=== ProximaX Sirius - Installing Linux Subsystem (%s) ===' -ForegroundColor Cyan; Write-Host 'Downloading distribution package (~500 MB). Please keep this window open...' -ForegroundColor Yellow; wsl.exe --install -d %s --no-launch *>&1 | Tee-Object -FilePath '%s'; if ($LASTEXITCODE -ne 0) { Write-Host 'Distribution setup encountered an error. Exit code:' $LASTEXITCODE -ForegroundColor Red; Write-Host 'Press any key to close this window...' -ForegroundColor Yellow; $host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown') }`, targetDistro, targetDistro, logPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Start-Process powershell -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', \"%s\" -Verb RunAs", strings.ReplaceAll(psScript, `"`, `\"`)))

	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out) + " " + extractExitHex(err)
		outLower := strings.ToLower(outStr)
		if strings.Contains(outStr, "1223") || strings.Contains(outLower, "canceled by the user") || strings.Contains(outLower, "access is denied") {
			return fmt.Errorf("UAC_DENIED: Administrator permissions were declined.")
		}
		if strings.Contains(outLower, "0x80370102") || strings.Contains(outLower, "80370102") {
			return fmt.Errorf("BIOS_VIRTUALIZATION_DISABLED: Hardware Virtualization is disabled or a system restart is required.")
		}
		if strings.Contains(outLower, "0x80072ee7") || strings.Contains(outLower, "80072ee7") {
			return fmt.Errorf("NETWORK_TIMEOUT: Network connection to Microsoft Store / WSL CDN timed out.")
		}
		if strings.Contains(outLower, "0x8024500c") || strings.Contains(outLower, "8024500c") {
			return fmt.Errorf("GROUP_POLICY_BLOCKED: Windows Update / Store is blocked by Group Policy.")
		}
		return fmt.Errorf("failed to install distribution %s: %v (%s)", distroName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// executeWSL executes the Sirius Catapult engine inside WSL2, streaming logs and orchestrating processes
func (dc *ProcessSupervisor) executeWSL(ctx context.Context, siriusBin string, chainConfigPath string, localDataDir string, libEnvList []string) error {
	status := dc.ProbeWSLStatus()
	if status.State != WSL2Ready {
		return fmt.Errorf("cannot start Sirius engine: WSL2 subsystem is not ready (%s: %s)", status.State, status.ErrorMessage)
	}

	distro := status.DistroName
	if distro == "" {
		distro = "Ubuntu-22.04"
	}

	// Configure PortProxy now that WSL2 is verified ready
	go func() {
		if err := dc.SetupPortProxy(); err != nil {
			dc.broadcastLog(fmt.Sprintf("[Supervisor] Note: PortProxy setup: %v", err))
		}
	}()

	// 1. Silent pre-flight self-healing: Ensure essential dynamic runtime dependencies are installed inside WSL (e.g. libatomic1)
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
		"sh", "-c", "dpkg -s libatomic1 >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq libatomic1 >/dev/null 2>&1)").Run()

	// 2. Binary Architecture Self-Healing: Verify sirius.bc and catapult.recovery are valid Linux ELF binaries.
	// If foreign binaries (e.g. macOS Mach-O) were checked out from Git, automatically restore official Linux x86_64 binaries.
	recoveryBin := filepath.Join(dc.binPath, "catapult.recovery")
	if !isLinuxELF(siriusBin) || !isLinuxELF(recoveryBin) {
		dc.broadcastLog("<warning> [Supervisor] Catapult engine binary is not a Linux ELF executable (architecture mismatch). Auto-healing precompiled Linux x86_64 binaries...")
		wslRootDir := ToWSLPath(filepath.Dir(dc.binPath))
		healCmd := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
			"sh", "-c", fmt.Sprintf("mkdir -p '%s/bin' && curl -f -sSL https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-linux-amd64.tar.gz | tar -xz -C '%s'", wslRootDir, wslRootDir))
		if healOut, err := healCmd.CombinedOutput(); err != nil {
			dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Failed to auto-heal Linux engine binaries: %v (%s)", err, strings.TrimSpace(cleanWSLOutput(healOut))))
		} else {
			dc.broadcastLog("[Supervisor] Successfully restored Linux ELF x86_64 Catapult engine.")
		}
	}

	wslBinDir := ToWSLPath(dc.binPath)
	wslSiriusBin := ToWSLPath(siriusBin)
	wslChainConfig := ToWSLPath(chainConfigPath)
	wslWorkDir := ToWSLPath(filepath.Dir(chainConfigPath))
	wslDataDir := ToWSLPath(localDataDir)

	// Clear any stale locks before starting
	dc.clearLocks(localDataDir)
	if wslDataDir != "" {
		_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
			"sh", "-c", fmt.Sprintf("rm -f '%s'/*.lock '%s'/statedb/*/LOCK >/dev/null 2>&1", wslDataDir, wslDataDir)).Run()
	}

	// 3. Pre-flight recovery execution inside WSL - only run if existing chain data has synced beyond nemesis (height > 1)
	shouldRunRecovery := false
	if indexPath := filepath.Join(localDataDir, "index.dat"); isPathExists(indexPath) {
		if data, err := os.ReadFile(indexPath); err == nil && len(data) >= 8 {
			if binary.LittleEndian.Uint64(data[:8]) > 1 {
				shouldRunRecovery = true
			}
		}
	}

	// If block height is <= 1, clean incomplete/corrupted statedb leftover from any previous aborted boots
	// so NemesisBlockLoader can calculate initial state cleanly
	if !shouldRunRecovery {
		stateDbDir := filepath.Join(localDataDir, "statedb")
		if _, sErr := os.Stat(stateDbDir); sErr == nil {
			_ = os.RemoveAll(stateDbDir)
		}
		if wslDataDir != "" {
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
				"sh", "-c", fmt.Sprintf("rm -rf '%s'/statedb >/dev/null 2>&1", wslDataDir)).Run()
		}
	} else if _, e := os.Stat(recoveryBin); e == nil {
		dc.broadcastLog("[Supervisor] Running catapult.recovery pre-flight check inside WSL2...")
		dc.runCatapultRecovery(localDataDir)
	}

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Starting Sirius Core engine in WSL2 (%s) from %s...", distro, wslSiriusBin))

	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "-u", "root", "--cd", wslWorkDir, "--",
		"env", fmt.Sprintf("LD_LIBRARY_PATH=%s", wslBinDir), wslSiriusBin, wslChainConfig)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open WSL stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to open WSL stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Sirius engine in WSL2: %w", err)
	}

	exitChan := make(chan struct{})
	dc.cmd = cmd
	dc.exitChan = exitChan
	dc.startTime = time.Now()
	dc.isRunning = true
	dc.isStopping = false
	dc.userIntendedRunning = true

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Sirius Core started inside WSL2 with Windows host wrapper PID %d", cmd.Process.Pid))

	go dc.streamPipe(stdoutPipe)
	go dc.streamPipe(stderrPipe)

	go func() {
		waitErr := cmd.Wait()
		close(exitChan)
		dc.mu.Lock()
		dc.isRunning = false
		dc.isStopping = false
		dc.cmd = nil
		dc.exitChan = nil
		dc.clearLocks(localDataDir)
		if wslDataDir != "" {
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
				"sh", "-c", fmt.Sprintf("rm -f '%s'/*.lock '%s'/statedb/*/LOCK >/dev/null 2>&1", wslDataDir, wslDataDir)).Run()
		}
		userStopped := !dc.userIntendedRunning
		if waitErr != nil && !userStopped {
			dc.lastError = fmt.Sprintf("Sirius Core process exited with error: %v", waitErr)
			dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Sirius Core process exited with error: %v", waitErr))
		} else {
			dc.lastError = ""
			dc.broadcastLog("[Supervisor] Sirius Core process stopped.")
		}
		dc.mu.Unlock()
	}()

	return nil
}

// stopWSL gracefully terminates sirius.bc inside WSL2 via SIGINT, ensuring in-flight
// disruptor blocks are fully committed and RocksDB on DrvFS flushes cleanly to disk without corruption.
func (dc *ProcessSupervisor) stopWSL() error {
	status := dc.ProbeWSLStatus()
	distro := status.DistroName
	if distro == "" {
		distro = "Ubuntu-22.04"
	}

	// 1. Send SIGINT to sirius.bc inside WSL so Catapult initiates its clean shutdown sequence
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pkill", "-INT", "-f", "sirius.bc").Run()

	startTime := time.Now()
	lastProgressTime := time.Now()
	lastLogBroadcast := time.Now()

	// Multi-signal progress tracking: active log, rotation, total log sizes, and storage height
	var lastActiveLogName string
	var lastActiveLogSize int64 = -1
	var lastTotalLogsSize int64 = -1
	var lastStorageHeight uint64 = 0

	logsDir := filepath.Join(filepath.Dir(dc.chainConfigPath), "chainconfig", "logs")
	if !isPathExists(logsDir) {
		logsDir = filepath.Join(dc.chainConfigPath, "logs")
	}

	indexPath := ""
	if dc.currentDataDir != "" {
		indexPath = filepath.Join(dc.currentDataDir, "index.dat")
	}
	if indexPath == "" || !isPathExists(indexPath) {
		indexPath = filepath.Join(filepath.Dir(dc.chainConfigPath), "chainconfig", "data", "index.dat")
		if !isPathExists(indexPath) {
			indexPath = filepath.Join(dc.chainConfigPath, "data", "index.dat")
		}
	}

	// Loop indefinitely as long as the engine is actively making forward progress (committing blocks)
	for {
		checkCmd := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pgrep", "-f", "sirius.bc")
		out, err := checkCmd.Output()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			// Engine process has terminated cleanly!
			dc.broadcastLog("[Supervisor] Flushing kernel filesystem buffers and RocksDB tables to physical storage...")
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "sync").Run()
			return nil
		}

		// Multi-Signal Forward Progress Check:
		// Signal 1: Log file size growth or log file rotation (e.g. server_0000.log -> server_0001.log)
		activeLog, activeSize, totalSize := getEngineLogProgress(logsDir)
		if activeLog != "" {
			if lastActiveLogName == "" {
				lastActiveLogName = activeLog
				lastActiveLogSize = activeSize
				lastTotalLogsSize = totalSize
			} else if activeLog != lastActiveLogName {
				// Log rotation occurred! Engine is actively writing to a new rotated log file
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Log rotation detected: %s -> %s (engine active)", lastActiveLogName, activeLog))
				lastProgressTime = time.Now()
				lastActiveLogName = activeLog
				lastActiveLogSize = activeSize
				lastTotalLogsSize = totalSize
			} else if activeSize > lastActiveLogSize || totalSize > lastTotalLogsSize {
				// Active log is receiving new writes
				lastProgressTime = time.Now()
				lastActiveLogSize = activeSize
				lastTotalLogsSize = totalSize
			}
		}

		// Signal 2: Storage height progress in index.dat
		// In-flight disruptor blocks committing increment storage height directly
		if idxBytes, iErr := os.ReadFile(indexPath); iErr == nil && len(idxBytes) >= 8 {
			curHeight := binary.LittleEndian.Uint64(idxBytes[:8])
			if curHeight > lastStorageHeight {
				if lastStorageHeight > 0 {
					lastProgressTime = time.Now()
				}
				lastStorageHeight = curHeight
			}
		}

		elapsed := int(time.Since(startTime).Seconds())
		timeSinceProgress := time.Since(lastProgressTime)

		if time.Since(lastLogBroadcast) >= 5*time.Second {
			lastLogBroadcast = time.Now()
			if timeSinceProgress < 10*time.Second {
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Gracefully committing in-flight blocks & flushing RocksDB statedb to disk (%ds elapsed, active log: %s)...", elapsed, activeLog))
			} else {
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Waiting for disk synchronization and RocksDB compaction to finish (%ds elapsed, idle: %ds)...", elapsed, int(timeSinceProgress.Seconds())))
			}
		}

		// Strictly progress-based timeout: NEVER interrupt or kill as long as forward progress continues.
		// Only if process has made ZERO progress for 60 consecutive seconds (genuine freeze / deadlock):
		if timeSinceProgress > 60*time.Second {
			dc.broadcastLog("<error> [Supervisor] Process made ZERO forward progress for 60 seconds (deadlock detected). Escalating with SIGKILL...")
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pkill", "-9", "-f", "sirius.bc").Run()
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "sync").Run()
			return fmt.Errorf("process was deadlocked and terminated with SIGKILL")
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// runCatapultRecovery executes catapult.recovery inside WSL2 to reconcile RocksDB WAL and flat files cleanly.
func (dc *ProcessSupervisor) runCatapultRecovery(localDataDir string) {
	status := dc.ProbeWSLStatus()
	distro := status.DistroName
	if distro == "" {
		distro = "Ubuntu-22.04"
	}

	recoveryBin := filepath.Join(dc.binPath, "catapult.recovery")
	if !isPathExists(recoveryBin) || !isLinuxELF(recoveryBin) {
		return
	}

	wslBinDir := ToWSLPath(dc.binPath)
	wslRecoveryBin := ToWSLPath(recoveryBin)
	wslChainConfig := ToWSLPath(dc.chainConfigPath)
	wslWorkDir := ToWSLPath(filepath.Dir(dc.chainConfigPath))
	wslDataDir := ToWSLPath(localDataDir)

	dc.broadcastLog("[Supervisor] Executing catapult.recovery reconciliation inside WSL2...")
	recCmd := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--cd", wslWorkDir, "--",
		"env", fmt.Sprintf("LD_LIBRARY_PATH=%s", wslBinDir), wslRecoveryBin, wslChainConfig)
	out, recErr := recCmd.CombinedOutput()
	if recErr != nil {
		dc.broadcastLog(fmt.Sprintf("<warning> [Supervisor] catapult.recovery returned: %v (details: %s)", recErr, strings.TrimSpace(cleanWSLOutput(out))))
	} else {
		dc.broadcastLog("[Supervisor] catapult.recovery reconciliation completed successfully.")
	}

	// Commit filesystem buffers to physical storage
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "sync").Run()

	dc.clearLocks(localDataDir)
	if wslDataDir != "" {
		_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
			"sh", "-c", fmt.Sprintf("rm -f '%s'/*.lock '%s'/statedb/*/LOCK >/dev/null 2>&1", wslDataDir, wslDataDir)).Run()
	}
}

// SetupPortProxy ensures Windows forwards P2P and REST ports to WSL2 adapter.
// It queries existing rules first without elevation. If all rules are already up to date,
// it returns immediately with zero UAC prompts. If the WSL IP changed, it batches
// updates into a SINGLE elevated command instead of prompting 4 separate times.
func (dc *ProcessSupervisor) SetupPortProxy() error {
	// Find WSL IP
	out, err := exec.Command("wsl.exe", "hostname", "-I").Output()
	if err != nil {
		return err
	}
	wslIP := strings.TrimSpace(strings.Fields(cleanWSLOutput(out))[0])
	if wslIP == "" {
		return fmt.Errorf("could not determine WSL IP")
	}

	// Ports: 7900 (P2P), 7901 (API), 7903 (DBRB), 3000 (REST)
	ports := []int{7900, 7901, 7903, 3000}

	// 1. Check existing portproxy configuration (read-only query requires no elevation)
	showCmd := exec.Command("netsh", "interface", "portproxy", "show", "all")
	showOut, showErr := showCmd.Output()

	needUpdate := false
	if showErr != nil {
		needUpdate = true
	} else {
		showStr := string(showOut)
		for _, port := range ports {
			portStr := fmt.Sprintf("%d", port)
			// Check if rule for this port already points to the current WSL IP
			if !strings.Contains(showStr, portStr) || !strings.Contains(showStr, wslIP) {
				needUpdate = true
				break
			}
		}
	}

	// If all rules already match current WSL IP, return immediately with zero UAC prompts!
	if !needUpdate {
		return nil
	}

	// 2. Batch all portproxy commands into a SINGLE elevated call
	var commands []string
	for _, port := range ports {
		commands = append(commands, fmt.Sprintf("netsh interface portproxy add v4tov4 listenport=%d listenaddress=0.0.0.0 connectport=%d connectaddress=%s", port, port, wslIP))
	}
	batchedScript := strings.Join(commands, "; ")

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Start-Process powershell -ArgumentList '-NoProfile -Command \"%s\"' -Verb RunAs -WindowStyle Hidden", batchedScript))
	return cmd.Run()
}
