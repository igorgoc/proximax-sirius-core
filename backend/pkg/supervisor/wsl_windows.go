//go:build windows

package supervisor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// 2. Query wsl.exe --status
	cmdStatus := exec.Command(wslExe, "--status")
	outStatus, errStatus := cmdStatus.CombinedOutput()
	cleanedStatus := cleanWSLOutput(outStatus)
	status.RawStatus = cleanedStatus

	// Detect BIOS/UEFI Virtualization disabled
	if strings.Contains(cleanedStatus, "0x80370102") ||
		strings.Contains(cleanedStatus, "CreateVm") ||
		strings.Contains(cleanedStatus, "Wsl/Service/CreateVm") ||
		strings.Contains(strings.ToLower(cleanedStatus), "virtual machine platform") {
		status.State = WSLNotInstalled
		status.ErrorCode = "BIOS_VIRTUALIZATION_DISABLED"
		status.ErrorMessage = "Hardware Virtualization is disabled in your computer's BIOS/UEFI. WSL2 requires CPU virtualization (Intel VT-x or AMD-V) to run."
		return status
	}

	if errStatus != nil && strings.Contains(strings.ToLower(cleanedStatus), "optional component") {
		status.State = WSLNotInstalled
		status.ErrorCode = "WSL_NOT_INSTALLED"
		status.ErrorMessage = "Windows Subsystem for Linux optional component is not enabled."
		return status
	}

	// Determine default WSL version
	status.DefaultVersion = 2
	if strings.Contains(cleanedStatus, "Default Version: 1") {
		status.DefaultVersion = 1
	}

	// 3. Query installed distributions with wsl.exe -l -v
	cmdList := exec.Command(wslExe, "-l", "-v")
	outList, errList := cmdList.CombinedOutput()
	cleanedList := cleanWSLOutput(outList)

	if errList != nil || strings.Contains(strings.ToLower(cleanedList), "no installed distributions") ||
		strings.Contains(cleanedList, "WSL_E_DEFAULT_DISTRO_NOT_FOUND") ||
		len(strings.TrimSpace(cleanedList)) == 0 {
		if status.DefaultVersion == 1 {
			status.State = WSLV1Only
			status.ErrorMessage = "WSL1 is enabled, but WSL2 is required for Sirius ext4/RocksDB performance."
		} else {
			status.State = WSL2NoDistro
			status.ErrorMessage = "WSL2 kernel is ready, but no Sirius Linux distribution is installed."
		}
		return status
	}

	// Parse distribution names and versions from `wsl.exe -l -v`
	lines := strings.Split(cleanedList, "\n")
	var selectedDistro string
	var selectedVersion int = 2

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "NAME") || strings.HasPrefix(trimmed, "----") || trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 3 {
			name := fields[0]
			isDefault := false
			if name == "*" && len(fields) >= 4 {
				isDefault = true
				name = fields[1]
			}
			verStr := fields[len(fields)-1]
			ver := 2
			if verStr == "1" {
				ver = 1
			}

			if isDefault || selectedDistro == "" || strings.Contains(strings.ToLower(name), "ubuntu") {
				selectedDistro = name
				selectedVersion = ver
			}
		}
	}

	if selectedDistro == "" {
		status.State = WSL2NoDistro
		status.ErrorMessage = "No suitable Linux distribution found in WSL."
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
	// Launch elevated PowerShell command to enable WSL2 without immediate distribution
	const psScript = `Start-Process wsl -ArgumentList '--install --no-distribution' -Verb RunAs`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		// Check for UAC cancellation (Win32 1223)
		if strings.Contains(outStr, "1223") || strings.Contains(strings.ToLower(outStr), "canceled by the user") || strings.Contains(strings.ToLower(outStr), "access is denied") {
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

func (dc *ProcessSupervisor) SetupWSLDistro(distroName string) error {
	if distroName == "" {
		distroName = "Ubuntu-22.04"
	}

	// Invalidate cache
	dc.wslCacheMu.Lock()
	dc.wslCacheTime = time.Time{}
	dc.wslCacheMu.Unlock()

	// 1. Ensure WSL2 default version
	_ = exec.Command("wsl.exe", "--set-default-version", "2").Run()

	// 2. Install requested distribution
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Start-Process wsl -ArgumentList '--install -d %s --no-launch' -Verb RunAs", distroName))
	out, err := cmd.CombinedOutput()
	if err != nil {
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

	// 1. Silent pre-flight self-healing: Ensure essential dynamic runtime dependencies are installed inside WSL (e.g. libatomic1)
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
		"sh", "-c", "dpkg -s libatomic1 >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq libatomic1 >/dev/null 2>&1)").Run()

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

	// 2. Pre-flight recovery execution inside WSL - only run if existing chain data has synced beyond nemesis (height > 1)
	recoveryBin := filepath.Join(dc.binPath, "catapult.recovery")
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
		wslRecoveryBin := ToWSLPath(recoveryBin)
		dc.broadcastLog("[Supervisor] Running catapult.recovery pre-flight check inside WSL2...")
		recCmd := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--cd", wslWorkDir, "--",
			"env", fmt.Sprintf("LD_LIBRARY_PATH=%s", wslBinDir), wslRecoveryBin, wslChainConfig)
		_ = recCmd.Run()
		dc.clearLocks(localDataDir)
		if wslDataDir != "" {
			_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--",
				"sh", "-c", fmt.Sprintf("rm -f '%s'/*.lock '%s'/statedb/*/LOCK >/dev/null 2>&1", wslDataDir, wslDataDir)).Run()
		}
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

	dc.cmd = cmd
	dc.startTime = time.Now()
	dc.isRunning = true
	dc.userIntendedRunning = true

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Sirius Core started inside WSL2 with Windows host wrapper PID %d", cmd.Process.Pid))

	go dc.streamPipe(stdoutPipe)
	go dc.streamPipe(stderrPipe)

	go func() {
		waitErr := cmd.Wait()
		dc.mu.Lock()
		dc.isRunning = false
		dc.cmd = nil
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

// stopWSL gracefully terminates sirius.bc inside WSL2 via SIGINT, then SIGKILL if needed
func (dc *ProcessSupervisor) stopWSL() error {
	status := dc.ProbeWSLStatus()
	distro := status.DistroName
	if distro == "" {
		distro = "Ubuntu-22.04"
	}

	// Send SIGINT to sirius.bc inside WSL
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pkill", "-INT", "-f", "sirius.bc").Run()

	// Wait up to 15 seconds for process termination
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		checkCmd := exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pgrep", "-f", "sirius.bc")
		out, err := checkCmd.Output()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			// Process has terminated
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Force kill if still lingering
	_ = exec.Command("wsl.exe", "-d", distro, "-u", "root", "--", "pkill", "-9", "-f", "sirius.bc").Run()
	return nil
}

// SetupPortProxy ensures Windows forwards P2P and REST ports to WSL2 adapter
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
	for _, port := range ports {
		ruleCmd := fmt.Sprintf("netsh interface portproxy add v4tov4 listenport=%d listenaddress=0.0.0.0 connectport=%d connectaddress=%s", port, port, wslIP)
		_ = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("Start-Process cmd -ArgumentList '/c %s' -Verb RunAs -WindowStyle Hidden", ruleCmd)).Run()
	}
	return nil
}
