package supervisor

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	DefaultContainerName = "sirius-native-peer"
	DefaultImageName     = "Native Sirius Core v1.9.7"
	DefaultSnapshotUrl   = "http://207.180.195.181/snapshot.tar.xz"

	Nemesis00001Url = "https://raw.githubusercontent.com/proximax-storage/xpx-mainnet-chain-onboarding/master/docker-method/data/00000/00001.dat"
	NemesisHashesUrl = "https://raw.githubusercontent.com/proximax-storage/xpx-mainnet-chain-onboarding/master/docker-method/data/00000/hashes.dat"
	NemesisIndexUrl  = "https://raw.githubusercontent.com/proximax-storage/xpx-mainnet-chain-onboarding/master/docker-method/data/index.dat"
)

type ContainerStatus string

const (
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusStarting ContainerStatus = "starting"
	StatusError    ContainerStatus = "error"
	StatusUnknown  ContainerStatus = "unknown"
)

type NodeMetrics struct {
	Status        ContainerStatus `json:"status"`
	Uptime        string          `json:"uptime"`
	CpuPercent    string          `json:"cpuPercent"`
	MemoryUsage   string          `json:"memoryUsage"`
	DiskUsage     string          `json:"diskUsage"`
	DiskFree      string          `json:"diskFree"`
	ContainerId   string          `json:"containerId"`
	Image         string          `json:"image"`
	BlockHeight   int64           `json:"blockHeight"`
	NetworkHeight int64           `json:"networkHeight"`
	PeersCount    int             `json:"peersCount"`
	ErrorMessage  string          `json:"errorMessage,omitempty"`
	ThreadsCount  int             `json:"threadsCount,omitempty"`
	NetworkRate   string          `json:"networkRate,omitempty"`
	DiskRate      string          `json:"diskRate,omitempty"`
}

type SnapshotStage string

const (
	SnapshotStageIdle        SnapshotStage = "idle"
	SnapshotStageDownloading SnapshotStage = "downloading"
	SnapshotStageDownloaded  SnapshotStage = "downloaded"
	SnapshotStageExtracting  SnapshotStage = "extracting"
	SnapshotStageComplete    SnapshotStage = "complete"
	SnapshotStageCancelled   SnapshotStage = "cancelled"
	SnapshotStageError       SnapshotStage = "error"
)

type DownloadProgress struct {
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Percentage      float64 `json:"percentage"`
	SpeedMBs        float64 `json:"speedMBs"`
	ETASeconds      int64   `json:"etaSeconds"`
}

type ExtractProgress struct {
	ExtractedFiles int64   `json:"extractedFiles"`
	ExtractedBytes int64   `json:"extractedBytes"`
	CurrentFile    string  `json:"currentFile"`
	Percentage     float64 `json:"percentage"`
}

type SnapshotStatus struct {
	Stage        SnapshotStage    `json:"stage"`
	Download     DownloadProgress `json:"download"`
	Extract      ExtractProgress  `json:"extract"`
	Message      string           `json:"message"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
}

type DataBackupStage string

const (
	DataBackupStageIdle      DataBackupStage = "idle"
	DataBackupStageBackingUp DataBackupStage = "backing_up"
	DataBackupStageComplete  DataBackupStage = "complete"
	DataBackupStageCancelled DataBackupStage = "cancelled"
	DataBackupStageError     DataBackupStage = "error"
)

type DataBackupStatus struct {
	Stage         DataBackupStage `json:"stage"`
	BackedUpBytes int64           `json:"backedUpBytes"`
	TotalBytes    int64           `json:"totalBytes"`
	Percentage    float64         `json:"percentage"`
	CurrentFile   string          `json:"currentFile"`
	TargetFile    string          `json:"targetFile,omitempty"`
	Message       string          `json:"message"`
	ErrorMessage  string          `json:"errorMessage,omitempty"`
}

type ProcessSupervisor struct {
	chainConfigPath string
	binPath         string
	mu              sync.Mutex
	logSubscribers  map[chan string]bool
	subMu           sync.RWMutex
	logBuffer       []string
	maxBufferLines  int
	latestLogHeight int64

	// Process management
	cmd        *exec.Cmd
	cmdCancel  context.CancelFunc
	startTime  time.Time
	isRunning  bool
	isStarting bool
	lastError  string

	// Snapshot state
	snapshotMu     sync.RWMutex
	snapshotStatus SnapshotStatus
	snapshotCancel context.CancelFunc

	// Data Backup state
	dataBackupMu     sync.RWMutex
	dataBackupStatus DataBackupStatus
	dataBackupCancel context.CancelFunc

	// Watchdog Auto-Recovery state
	autoRecoveryMu      sync.RWMutex
	autoRecoveryEnabled bool
	userIntendedRunning bool
	lastAutoRecovered   time.Time

	// In-memory metrics cache
	metricsCache    *NodeMetrics
	lastMetricsTime time.Time
	metricsMu       sync.RWMutex

	// System network I/O sampling
	lastNetSampleTime time.Time
	lastNetInBytes    int64
	lastNetOutBytes   int64
	cachedNetRate     string

	// Known peers cache & P2P socket discovery
	knownPeersCache map[string]ConnectedPeer
	knownPeersMu    sync.RWMutex
	knownPeersTime  time.Time
}

type ConnectedPeer struct {
	PublicKey    string `json:"publicKey"`
	Port         int    `json:"port"`
	NetworkId    int    `json:"networkIdentifier"`
	Version      int    `json:"version"`
	Roles        int    `json:"roles"`
	Host         string `json:"host"`
	FriendlyName string `json:"friendlyName"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
}

type knownPeerEntry struct {
	PublicKey string `json:"publicKey"`
	Endpoint  struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		DbrbPort int    `json:"dbrbPort"`
	} `json:"endpoint"`
	Metadata struct {
		Name  string `json:"name"`
		Roles string `json:"roles"`
	} `json:"metadata"`
}

type knownPeersFile struct {
	KnownPeers []knownPeerEntry `json:"knownPeers"`
}

func NewProcessSupervisor(chainConfigPath string) *ProcessSupervisor {
	absChainConfig, _ := filepath.Abs(chainConfigPath)
	binDir := filepath.Join(filepath.Dir(absChainConfig), "bin")
	if _, err := os.Stat(filepath.Join(binDir, "sirius.bc")); err != nil {
		if stat, err2 := os.Stat("./bin/sirius.bc"); err2 == nil && !stat.IsDir() {
			binDir, _ = filepath.Abs("./bin")
		} else if stat3, err3 := os.Stat("../bin/sirius.bc"); err3 == nil && !stat3.IsDir() {
			binDir, _ = filepath.Abs("../bin")
		}
	}

	dc := &ProcessSupervisor{
		chainConfigPath:     absChainConfig,
		binPath:             binDir,
		logSubscribers:      make(map[chan string]bool),
		maxBufferLines:      500,
		autoRecoveryEnabled: true,
		knownPeersCache:     make(map[string]ConnectedPeer),
		snapshotStatus: SnapshotStatus{
			Stage: SnapshotStageIdle,
		},
		dataBackupStatus: DataBackupStatus{
			Stage: DataBackupStageIdle,
		},
	}
	return dc
}

func (dc *ProcessSupervisor) GetBinPath() string {
	return dc.binPath
}

func (dc *ProcessSupervisor) IsRunning() bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.isRunning
}

func (dc *ProcessSupervisor) locateBinaries() (siriusBin string, recoveryBin string, err error) {
	siriusNames := []string{"sirius.bc"}
	recoveryNames := []string{"catapult.recovery"}
	if runtime.GOOS == "windows" {
		siriusNames = []string{"sirius.exe", "sirius.bc.exe", "sirius.bc"}
		recoveryNames = []string{"catapult.recovery.exe", "catapult.recovery"}
	}

	for _, name := range siriusNames {
		candidate := filepath.Join(dc.binPath, name)
		if _, e := os.Stat(candidate); e == nil {
			siriusBin = candidate
			break
		}
	}
	if siriusBin == "" {
		return "", "", fmt.Errorf("sirius binary not found in: %s", dc.binPath)
	}

	for _, name := range recoveryNames {
		candidate := filepath.Join(dc.binPath, name)
		if _, e := os.Stat(candidate); e == nil {
			recoveryBin = candidate
			break
		}
	}
	return siriusBin, recoveryBin, nil
}

func (dc *ProcessSupervisor) StartNode(dataPath string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.isRunning {
		return fmt.Errorf("node is already running")
	}

	dc.isStarting = true
	dc.lastError = ""
	defer func() { dc.isStarting = false }()

	siriusBin, recoveryBin, err := dc.locateBinaries()
	if err != nil {
		dc.lastError = err.Error()
		return err
	}

	localDataDir := dataPath
	if localDataDir == "" {
		localDataDir = filepath.Join(dc.chainConfigPath, "data")
	} else if !filepath.IsAbs(localDataDir) {
		localDataDir = filepath.Join(dc.chainConfigPath, "..", localDataDir)
	}

	// 1. Pre-flight Validation: Check if the configured data directory or its mount volume exists
	if _, err := os.Stat(localDataDir); os.IsNotExist(err) {
		parentDir := filepath.Dir(localDataDir)
		if _, pErr := os.Stat(parentDir); os.IsNotExist(pErr) {
			friendlyErr := fmt.Sprintf("Data directory not found: %s — check that the drive is connected", localDataDir)
			dc.lastError = friendlyErr
			dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Startup pre-flight check failed: %s", friendlyErr))
			return fmt.Errorf("%s", friendlyErr)
		}
		// Attempt to create target directory if parent exists
		if mkErr := os.MkdirAll(localDataDir, 0755); mkErr != nil {
			friendlyErr := fmt.Sprintf("Data directory inaccessible: %s (%v) — check drive permissions", localDataDir, mkErr)
			dc.lastError = friendlyErr
			dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Startup pre-flight check failed: %s", friendlyErr))
			return fmt.Errorf("%s", friendlyErr)
		}
	}

	// 2. Pre-flight Validation: Test write permission
	testFile := filepath.Join(localDataDir, ".sirius_write_test")
	if wErr := os.WriteFile(testFile, []byte("ok"), 0644); wErr != nil {
		friendlyErr := fmt.Sprintf("Data directory is not writable: %s (%v) — check drive permissions and read-only status", localDataDir, wErr)
		dc.lastError = friendlyErr
		dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Startup pre-flight check failed: %s", friendlyErr))
		return fmt.Errorf("%s", friendlyErr)
	}
	_ = os.Remove(testFile)

	localLogsDir := filepath.Join(dc.chainConfigPath, "logs")
	certDir := filepath.Join(dc.chainConfigPath, "certificate")

	_ = os.MkdirAll(localDataDir, 0755)
	_ = os.MkdirAll(localLogsDir, 0755)
	_ = os.MkdirAll(certDir, 0755)

	_ = dc.EnsureNemesisSeed(localDataDir)

	dc.clearLocks(localDataDir)
	dc.syncProperties(localDataDir, certDir)

	// Data Integrity Preflight Check: Catch truncated index.dat, empty statedb, or 0-byte tail block files
	if err := dc.checkDataIntegrityPreflight(localDataDir); err != nil {
		friendlyErr := fmt.Sprintf("Startup data integrity pre-flight check failed: %v", err)
		dc.lastError = friendlyErr
		dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] %s", friendlyErr))
		return fmt.Errorf("%s", friendlyErr)
	}

	dyldPath := dc.binPath
	if homeDir, err := os.UserHomeDir(); err == nil {
		boostLib := filepath.Join(homeDir, "boost-build-1.81.0", "lib")
		if _, err := os.Stat(boostLib); err == nil {
			dyldPath = fmt.Sprintf("%s:%s", dc.binPath, boostLib)
		}
	}
	if envBoost := os.Getenv("BOOST_ROOT"); envBoost != "" {
		dyldPath = fmt.Sprintf("%s:%s/lib", dyldPath, envBoost)
	}

	// Build cross-platform library search paths (macOS, Linux, Windows)
	libEnvList := []string{
		fmt.Sprintf("DYLD_LIBRARY_PATH=%s", dyldPath),
		fmt.Sprintf("LD_LIBRARY_PATH=%s", dyldPath),
	}
	if runtime.GOOS == "windows" {
		libEnvList = append(libEnvList, fmt.Sprintf("PATH=%s;%s", dyldPath, os.Getenv("PATH")))
	}

	if _, e := os.Stat(recoveryBin); e == nil {
		dc.broadcastLog("[Supervisor] Running native catapult.recovery pre-flight check...")
		recCmd := exec.Command(recoveryBin, dc.chainConfigPath)
		recCmd.Env = append(os.Environ(), libEnvList...)
		_ = recCmd.Run()
		dc.clearLocks(localDataDir)
	}

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Starting native Sirius Core process from %s...", siriusBin))
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, siriusBin, dc.chainConfigPath)
	cmd.Env = append(os.Environ(), libEnvList...)
	cmd.Dir = filepath.Dir(dc.chainConfigPath)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		dc.lastError = err.Error()
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		dc.lastError = err.Error()
		return fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		dc.lastError = err.Error()
		return fmt.Errorf("failed to start native sirius.bc process: %w", err)
	}

	dc.cmd = cmd
	dc.cmdCancel = cancel
	dc.startTime = time.Now()
	dc.isRunning = true
	dc.userIntendedRunning = true

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Sirius Core started natively with PID %d", cmd.Process.Pid))

	go dc.streamPipe(stdoutPipe)
	go dc.streamPipe(stderrPipe)

	go func() {
		waitErr := cmd.Wait()
		dc.mu.Lock()
		dc.isRunning = false
		dc.cmd = nil
		dc.clearLocks(localDataDir)
		dc.mu.Unlock()

		if waitErr != nil {
			dc.lastError = fmt.Sprintf("Sirius Core process exited with error: %v", waitErr)
			dc.broadcastLog(fmt.Sprintf("<error> [Supervisor] Sirius Core process exited with error: %v", waitErr))
		} else {
			dc.broadcastLog("[Supervisor] Sirius Core process exited cleanly")
		}
	}()

	return nil
}

func (dc *ProcessSupervisor) clearLocks(dataPath string) {
	_ = os.Chmod(filepath.Join(dataPath, "server.lock"), 0777)
	_ = os.Remove(filepath.Join(dataPath, "server.lock"))
	_ = os.Chmod(filepath.Join(dataPath, "recovery.lock"), 0777)
	_ = os.Remove(filepath.Join(dataPath, "recovery.lock"))
	stateDbDir := filepath.Join(dataPath, "statedb")
	if entries, err := os.ReadDir(stateDbDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				_ = os.Chmod(filepath.Join(stateDbDir, entry.Name(), "LOCK"), 0777)
				_ = os.Remove(filepath.Join(stateDbDir, entry.Name(), "LOCK"))
			}
		}
	}
}

// checkDataIntegrityPreflight validates core chain data files before starting the engine:
// 1. index.dat exists, is at least 8 bytes, and contains a valid non-zero block height
// 2. statedb directory is accessible and not empty when block height > 1
// 3. tail block files (blocks.dat, hashes.dat) are non-zero size
func (dc *ProcessSupervisor) checkDataIntegrityPreflight(dataDir string) error {
	indexPath := filepath.Join(dataDir, "index.dat")
	indexInfo, err := os.Stat(indexPath)
	if err == nil {
		if indexInfo.Size() < 8 {
			return fmt.Errorf("index.dat is truncated or corrupted (size %d bytes, minimum expected 8 bytes)", indexInfo.Size())
		}
		data, readErr := os.ReadFile(indexPath)
		if readErr != nil {
			return fmt.Errorf("failed to read index.dat: %w", readErr)
		}
		if len(data) < 8 {
			return fmt.Errorf("index.dat has insufficient bytes (%d < 8)", len(data))
		}
		height := binary.LittleEndian.Uint64(data[:8])
		if height == 0 {
			return fmt.Errorf("index.dat contains invalid block height 0")
		}

		// Check statedb if height > 1
		if height > 1 {
			stateDbPath := filepath.Join(dataDir, "statedb")
			stateInfo, sErr := os.Stat(stateDbPath)
			if sErr != nil {
				return fmt.Errorf("statedb directory is missing or unreadable at height %d: %w", height, sErr)
			}
			if !stateInfo.IsDir() {
				return fmt.Errorf("statedb path is not a directory: %s", stateDbPath)
			}
			entries, rErr := os.ReadDir(stateDbPath)
			if rErr != nil {
				return fmt.Errorf("failed to read statedb directory: %w", rErr)
			}
			if len(entries) == 0 {
				return fmt.Errorf("statedb directory is completely empty while block height is %d", height)
			}
		}
	}

	// Check tail block directory for non-zero blocks.dat and hashes.dat
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		highestDir := ""
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				if len(name) == 5 && isNumeric(name) {
					if name > highestDir {
						highestDir = name
					}
				}
			}
		}
		if highestDir != "" {
			tailBlockPath := filepath.Join(dataDir, highestDir, "blocks.dat")
			if info, bErr := os.Stat(tailBlockPath); bErr == nil {
				if info.Size() == 0 {
					return fmt.Errorf("tail block file %s/%s is 0 bytes (corrupted)", highestDir, "blocks.dat")
				}
			}
			tailHashesPath := filepath.Join(dataDir, highestDir, "hashes.dat")
			if info, hErr := os.Stat(tailHashesPath); hErr == nil {
				if info.Size() == 0 {
					return fmt.Errorf("tail hash file %s/%s is 0 bytes (corrupted)", highestDir, "hashes.dat")
				}
			}
		}
	}

	return nil
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (dc *ProcessSupervisor) syncProperties(dataPath string, certDir string) {
	resourcesDir := filepath.Join(dc.chainConfigPath, "resources")
	userProps := filepath.Join(resourcesDir, "config-user.properties")

	if content, err := os.ReadFile(userProps); err == nil {
		lines := strings.Split(string(content), "\n")
		var newLines []string
		hasDataDir := false
		hasPluginsDir := false
		hasCertDir := false

		for _, line := range lines {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "dataDirectory") {
				newLines = append(newLines, fmt.Sprintf("dataDirectory = %s", dataPath))
				hasDataDir = true
			} else if strings.HasPrefix(t, "pluginsDirectory") {
				newLines = append(newLines, fmt.Sprintf("pluginsDirectory = %s", dc.binPath))
				hasPluginsDir = true
			} else if strings.HasPrefix(t, "certificateDirectory") {
				newLines = append(newLines, fmt.Sprintf("certificateDirectory = %s", certDir))
				hasCertDir = true
			} else {
				newLines = append(newLines, line)
			}
		}

		if !hasDataDir {
			newLines = append(newLines, fmt.Sprintf("dataDirectory = %s", dataPath))
		}
		if !hasPluginsDir {
			newLines = append(newLines, fmt.Sprintf("pluginsDirectory = %s", dc.binPath))
		}
		if !hasCertDir {
			newLines = append(newLines, fmt.Sprintf("certificateDirectory = %s", certDir))
		}

		_ = os.WriteFile(userProps, []byte(strings.Join(newLines, "\n")), 0644)
	}

	logsDir := filepath.Join(dc.chainConfigPath, "logs")
	logConfigs := []string{
		filepath.Join(resourcesDir, "config-logging-server.properties"),
		filepath.Join(resourcesDir, "config-logging-recovery.properties"),
	}
	for _, lcfg := range logConfigs {
		if content, err := os.ReadFile(lcfg); err == nil {
			lines := strings.Split(string(content), "\n")
			var newLines []string
			for _, line := range lines {
				t := strings.TrimSpace(line)
				if strings.HasPrefix(t, "directory =") {
					newLines = append(newLines, fmt.Sprintf("directory = %s", logsDir))
				} else if strings.HasPrefix(t, "filePattern =") {
					patternName := "server_%4N.log"
					if strings.Contains(lcfg, "recovery") {
						patternName = "recovery_%4N.log"
					}
					newLines = append(newLines, fmt.Sprintf("filePattern = %s", filepath.Join(logsDir, patternName)))
				} else {
					newLines = append(newLines, line)
				}
			}
			_ = os.WriteFile(lcfg, []byte(strings.Join(newLines, "\n")), 0644)
		}
	}
}

var heightLogRegex = regexp.MustCompile(`height:?\s*([0-9]+)`)

func (dc *ProcessSupervisor) streamPipe(r io.Reader) {
	if r == nil {
		return
	}

	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			text := strings.TrimRight(line, "\r\n")
			if strings.TrimSpace(text) != "" {
				if matches := heightLogRegex.FindStringSubmatch(text); len(matches) > 1 {
					if h, parseErr := strconv.ParseInt(matches[1], 10, 64); parseErr == nil && h > 0 {
						dc.subMu.Lock()
						if h > dc.latestLogHeight {
							dc.latestLogHeight = h
						}
						dc.subMu.Unlock()
					}
				}
				dc.broadcastLog(text)
			}
		}

		if err != nil {
			if err != io.EOF {
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Pipe streaming closed: %v", err))
			}
			break
		}
	}
}

func (dc *ProcessSupervisor) StopNode() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.userIntendedRunning = false
	if !dc.isRunning || dc.cmd == nil || dc.cmd.Process == nil {
		return nil
	}

	dc.broadcastLog("[Supervisor] Stopping Sirius Core process gracefully...")
	if runtime.GOOS == "windows" {
		_ = dc.cmd.Process.Signal(os.Interrupt)
	} else {
		_ = dc.cmd.Process.Signal(syscall.SIGINT)
	}

	done := make(chan error, 1)
	go func() {
		if dc.cmd != nil && dc.cmd.Process != nil {
			_, _ = dc.cmd.Process.Wait()
		}
		done <- nil
	}()

	select {
	case <-done:
		dc.broadcastLog("[Supervisor] Sirius Core process terminated cleanly.")
	case <-time.After(10 * time.Second):
		dc.broadcastLog("[Supervisor] Process did not terminate in 10s, forcing SIGKILL...")
		if dc.cmd != nil && dc.cmd.Process != nil {
			_ = dc.cmd.Process.Kill()
		}
	}

	if dc.cmdCancel != nil {
		dc.cmdCancel()
	}
	dc.isRunning = false
	dc.cmd = nil
	return nil
}

func (dc *ProcessSupervisor) SetAutoRecovery(enabled bool) {
	dc.autoRecoveryMu.Lock()
	defer dc.autoRecoveryMu.Unlock()
	dc.autoRecoveryEnabled = enabled
}

func (dc *ProcessSupervisor) IsAutoRecoveryEnabled() bool {
	dc.autoRecoveryMu.RLock()
	defer dc.autoRecoveryMu.RUnlock()
	return dc.autoRecoveryEnabled
}

func (dc *ProcessSupervisor) StartWatchdog(dataPathProvider func() string) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			dc.autoRecoveryMu.RLock()
			enabled := dc.autoRecoveryEnabled
			userWanted := dc.userIntendedRunning
			lastRecovered := dc.lastAutoRecovered
			dc.autoRecoveryMu.RUnlock()

			if !enabled || !userWanted {
				continue
			}

			status, _ := dc.GetStatus()
			if status != StatusRunning && status != StatusStarting {
				if time.Since(lastRecovered) < 30*time.Second {
					continue
				}

				dc.autoRecoveryMu.Lock()
				dc.lastAutoRecovered = time.Now()
				dc.autoRecoveryMu.Unlock()

				dc.broadcastLog("[Watchdog] Native Sirius Core node is stopped unexpectedly. Initiating auto-recovery...")
				dp := ""
				if dataPathProvider != nil {
					dp = dataPathProvider()
				}
				_ = dc.StartNode(dp)
			}
		}
	}()
}

func (dc *ProcessSupervisor) RestartNode(dataPath string) error {
	_ = dc.StopNode()
	time.Sleep(1 * time.Second)
	return dc.StartNode(dataPath)
}

func (dc *ProcessSupervisor) GetStatus() (ContainerStatus, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.isStarting {
		return StatusStarting, nil
	}
	if dc.isRunning && dc.cmd != nil && dc.cmd.Process != nil {
		if err := dc.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			return StatusRunning, nil
		}
	}
	if dc.lastError != "" {
		return StatusError, nil
	}
	return StatusStopped, nil
}

func (dc *ProcessSupervisor) GetMetrics(dataPath string) (*NodeMetrics, error) {
	dc.metricsMu.RLock()
	if dc.metricsCache != nil && time.Since(dc.lastMetricsTime) < 2*time.Second {
		cached := *dc.metricsCache
		dc.metricsMu.RUnlock()
		return &cached, nil
	}
	dc.metricsMu.RUnlock()

	status, _ := dc.GetStatus()
	metrics := &NodeMetrics{
		Status:       status,
		Image:        fmt.Sprintf("%s v1.9.7", getPlatformArchLabel()),
		CpuPercent:   "0.0%",
		MemoryUsage:  "0 MB",
		DiskUsage:    "0 B",
		DiskFree:     "0 B",
		BlockHeight:  dc.GetLatestLogHeight(),
		ErrorMessage: dc.lastError,
	}

	targetDataDir := dataPath
	if targetDataDir == "" {
		targetDataDir = filepath.Join(dc.chainConfigPath, "data")
	} else if !filepath.IsAbs(targetDataDir) {
		targetDataDir = filepath.Join(dc.chainConfigPath, "..", targetDataDir)
	}

	if free, total, used, err := getDiskSpaceBytes(targetDataDir); err == nil && total > 0 {
		metrics.DiskFree = formatBytes(int64(free))
		metrics.DiskUsage = fmt.Sprintf("%s / %s (%.1f%%)", formatBytes(int64(used)), formatBytes(int64(total)), (float64(used)/float64(total))*100)
	}

	runningPid := 0
	if dc.cmd != nil && dc.cmd.Process != nil {
		runningPid = dc.cmd.Process.Pid
	} else if pOut, pErr := exec.Command("pgrep", "-f", "sirius.bc").Output(); pErr == nil {
		lines := strings.Split(strings.TrimSpace(string(pOut)), "\n")
		if len(lines) > 0 && lines[0] != "" {
			runningPid, _ = strconv.Atoi(strings.TrimSpace(lines[0]))
		}
	}

	if status == StatusRunning && runningPid > 0 {
		metrics.ContainerId = fmt.Sprintf("%s (PID: %d)", getPlatformArchLabel(), runningPid)
		if !dc.startTime.IsZero() {
			uptimeDuration := time.Since(dc.startTime).Round(time.Second)
			metrics.Uptime = uptimeDuration.String()
		} else {
			// Query process uptime from ps etime
			if upOut, upErr := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(runningPid)).Output(); upErr == nil {
				metrics.Uptime = strings.TrimSpace(string(upOut))
			}
		}

		psCmd := exec.Command("ps", "-o", "%cpu,rss", "-p", strconv.Itoa(runningPid))
		if out, err := psCmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 2 {
					metrics.CpuPercent = fields[0] + "%"
					if rssKB, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						metrics.MemoryUsage = formatBytes(rssKB * 1024)
					}
				}
			}
		}

		// Fast thread count
		if runtime.GOOS == "linux" {
			if tasks, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", runningPid)); err == nil && len(tasks) > 0 {
				metrics.ThreadsCount = len(tasks)
			}
		} else if thOut, thErr := exec.Command("sh", "-c", fmt.Sprintf("ps -M -p %d 2>/dev/null | tail -n +2 | wc -l", runningPid)).Output(); thErr == nil {
			if count, err := strconv.Atoi(strings.TrimSpace(string(thOut))); err == nil && count > 0 {
				metrics.ThreadsCount = count
			}
		}

		// System / P2P Network I/O delta sampling
		now := time.Now()
		if curIn, curOut, err := getSystemNetworkBytes(); err == nil {
			if !dc.lastNetSampleTime.IsZero() && now.After(dc.lastNetSampleTime) {
				secs := now.Sub(dc.lastNetSampleTime).Seconds()
				if secs > 0 && curIn >= dc.lastNetInBytes && curOut >= dc.lastNetOutBytes {
					inRate := float64(curIn-dc.lastNetInBytes) / secs
					outRate := float64(curOut-dc.lastNetOutBytes) / secs
					dc.cachedNetRate = fmt.Sprintf("↓ %s/s  ↑ %s/s", formatBytes(int64(inRate)), formatBytes(int64(outRate)))
				}
			}
			dc.lastNetInBytes = curIn
			dc.lastNetOutBytes = curOut
			dc.lastNetSampleTime = now
		}
		if dc.cachedNetRate != "" {
			metrics.NetworkRate = dc.cachedNetRate
		} else {
			metrics.NetworkRate = "Active P2P Mesh"
		}
		metrics.DiskRate = "Normal (RocksDB WAL)"
	} else {
		metrics.ContainerId = "None"
		metrics.Uptime = "0s"
	}

	dc.metricsMu.Lock()
	dc.metricsCache = metrics
	dc.lastMetricsTime = time.Now()
	dc.metricsMu.Unlock()

	return metrics, nil
}

func (dc *ProcessSupervisor) SubscribeLogs() (chan string, []string) {
	dc.subMu.Lock()
	defer dc.subMu.Unlock()

	ch := make(chan string, 100)
	dc.logSubscribers[ch] = true

	if len(dc.logBuffer) == 0 {
		dc.loadLatestLogsFromDisk()
	}

	history := make([]string, len(dc.logBuffer))
	copy(history, dc.logBuffer)

	return ch, history
}

func (dc *ProcessSupervisor) loadLatestLogsFromDisk() {
	logsDir := filepath.Join(dc.chainConfigPath, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return
	}

	var latestFile string
	var latestModTime time.Time

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if (strings.HasPrefix(name, "server_") && strings.HasSuffix(name, ".log")) || name == "catapult_server.log" {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestModTime) {
				latestModTime = info.ModTime()
				latestFile = filepath.Join(logsDir, name)
			}
		}
	}

	if latestFile == "" {
		return
	}

	file, err := os.Open(latestFile)
	if err != nil {
		return
	}
	defer file.Close()

	const maxLines = 1000
	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}

	dc.logBuffer = lines
}

func (dc *ProcessSupervisor) UnsubscribeLogs(ch chan string) {
	dc.subMu.Lock()
	defer dc.subMu.Unlock()

	if _, ok := dc.logSubscribers[ch]; ok {
		delete(dc.logSubscribers, ch)
		close(ch)
	}
}

func (dc *ProcessSupervisor) broadcastLog(line string) {
	dc.subMu.Lock()
	defer dc.subMu.Unlock()

	dc.logBuffer = append(dc.logBuffer, line)
	if len(dc.logBuffer) > dc.maxBufferLines {
		dc.logBuffer = dc.logBuffer[len(dc.logBuffer)-dc.maxBufferLines:]
	}

	for ch := range dc.logSubscribers {
		select {
		case ch <- line:
		default:
		}
	}
}

func (dc *ProcessSupervisor) GetLatestLogHeight() int64 {
	dc.subMu.RLock()
	defer dc.subMu.RUnlock()
	return dc.latestLogHeight
}

func (dc *ProcessSupervisor) loadKnownPeers() map[string]ConnectedPeer {
	dc.knownPeersMu.RLock()
	if len(dc.knownPeersCache) > 0 && time.Since(dc.knownPeersTime) < 10*time.Minute {
		res := make(map[string]ConnectedPeer, len(dc.knownPeersCache))
		for k, v := range dc.knownPeersCache {
			res[k] = v
		}
		dc.knownPeersMu.RUnlock()
		return res
	}
	dc.knownPeersMu.RUnlock()

	dc.knownPeersMu.Lock()
	defer dc.knownPeersMu.Unlock()

	if len(dc.knownPeersCache) > 0 && time.Since(dc.knownPeersTime) < 10*time.Minute {
		res := make(map[string]ConnectedPeer, len(dc.knownPeersCache))
		for k, v := range dc.knownPeersCache {
			res[k] = v
		}
		return res
	}

	peersMap := make(map[string]ConnectedPeer)

	files := []string{
		filepath.Join(dc.chainConfigPath, "resources", "peers-p2p.json"),
		filepath.Join(dc.chainConfigPath, "resources", "peers-api.json"),
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var kf knownPeersFile
		if err := json.Unmarshal(data, &kf); err != nil {
			continue
		}

		for _, p := range kf.KnownPeers {
			role := 1 // default Peer Validator
			if strings.EqualFold(p.Metadata.Roles, "Api") {
				role = 2
			} else if strings.EqualFold(p.Metadata.Roles, "Dual") {
				role = 3
			}

			cp := ConnectedPeer{
				PublicKey:    p.PublicKey,
				Port:         p.Endpoint.Port,
				NetworkId:    184,
				Version:      1,
				Roles:        role,
				Host:         p.Endpoint.Host,
				FriendlyName: p.Metadata.Name,
			}
			if cp.Port == 0 {
				cp.Port = 7900
			}

			peersMap[p.Endpoint.Host] = cp

			ips, err := net.LookupIP(p.Endpoint.Host)
			if err == nil {
				for _, ip := range ips {
					peersMap[ip.String()] = cp
				}
			}
		}
	}

	dc.knownPeersCache = peersMap
	dc.knownPeersTime = time.Now()

	res := make(map[string]ConnectedPeer, len(peersMap))
	for k, v := range peersMap {
		res[k] = v
	}
	return res
}

func (dc *ProcessSupervisor) GetConnectedPeers() []ConnectedPeer {
	dc.mu.Lock()
	running := dc.isRunning
	dc.mu.Unlock()

	if !running {
		return []ConnectedPeer{}
	}

	knownPeers := dc.loadKnownPeers()

	// 1. Query established socket connections with numeric ports/ips (-n -P)
	out, err := exec.Command("lsof", "-n", "-P", "-i", ":7900").Output()
	var lines []string
	if err == nil {
		lines = strings.Split(string(out), "\n")
	} else {
		// Fallback for Linux environments without lsof: ss
		ssOut, ssErr := exec.Command("ss", "-t", "-a", "-n", "(", "sport", "=", ":7900", "or", "dport", "=", ":7900", ")").Output()
		if ssErr == nil {
			lines = strings.Split(string(ssOut), "\n")
		}
	}

	seenIPs := make(map[string]bool)
	var peers []ConnectedPeer

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "ESTABLISHED") {
			continue
		}

		var remoteEndpoint string

		if strings.Contains(line, "->") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "->") {
					tokens := strings.Split(part, "->")
					if len(tokens) == 2 {
						remoteEndpoint = tokens[1]
					}
					break
				}
			}
		} else {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				remoteEndpoint = fields[4]
			}
		}

		if remoteEndpoint == "" {
			continue
		}

		remoteHost, remotePortStr, err := net.SplitHostPort(remoteEndpoint)
		if err != nil {
			remoteHost = remoteEndpoint
			remotePortStr = "7900"
		}

		remoteHost = strings.TrimPrefix(remoteHost, "[")
		remoteHost = strings.TrimSuffix(remoteHost, "]")

		if remoteHost == "" || remoteHost == "127.0.0.1" || remoteHost == "::1" || remoteHost == "localhost" {
			continue
		}

		if seenIPs[remoteHost] {
			continue
		}
		seenIPs[remoteHost] = true

		remotePort, _ := strconv.Atoi(remotePortStr)
		if remotePort == 0 {
			remotePort = 7900
		}

		if known, ok := knownPeers[remoteHost]; ok {
			peer := known
			peers = append(peers, peer)
		} else {
			peers = append(peers, ConnectedPeer{
				PublicKey:    "",
				Port:         remotePort,
				NetworkId:    184,
				Version:      1,
				Roles:        1,
				Host:         remoteHost,
				FriendlyName: fmt.Sprintf("peer-%s", remoteHost),
			})
		}
	}

	return peers
}

func (dc *ProcessSupervisor) GetConnectedPeersCount() int {
	return len(dc.GetConnectedPeers())
}

func (dc *ProcessSupervisor) ResetChain(dataPath string) error {
	_ = dc.StopNode()

	targetDataDir := dataPath
	if targetDataDir == "" {
		targetDataDir = filepath.Join(dc.chainConfigPath, "data")
	} else if !filepath.IsAbs(targetDataDir) {
		targetDataDir = filepath.Join(dc.chainConfigPath, "..", targetDataDir)
	}

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Initiating nemesis reset for blockchain data at %s...", targetDataDir))

	// Create temporary backup staging directory (following reset.sh pattern: backup to /tmp)
	backupDir := filepath.Join(os.TempDir(), fmt.Sprintf("sirius_nemesis_backup_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp backup dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(backupDir) }()

	backupDat := filepath.Join(backupDir, "00001.dat")
	backupHashes := filepath.Join(backupDir, "hashes.dat")

	defaultNemesisDat := filepath.Join(dc.chainConfigPath, "data", "00000", "00001.dat")
	defaultNemesisHashes := filepath.Join(dc.chainConfigPath, "data", "00000", "hashes.dat")

	// 1. Back up nemesis block from local data if safely available
	if hasValidNemesis(targetDataDir) {
		dc.broadcastLog("[Supervisor] Backing up local nemesis block (00001.dat, hashes.dat)...")
		if err := copyFile(filepath.Join(targetDataDir, "00000", "00001.dat"), backupDat); err != nil {
			return fmt.Errorf("failed to backup 00001.dat: %w", err)
		}
		if err := copyFile(filepath.Join(targetDataDir, "00000", "hashes.dat"), backupHashes); err != nil {
			return fmt.Errorf("failed to backup hashes.dat: %w", err)
		}
	} else if hasValidNemesis(filepath.Join(dc.chainConfigPath, "data")) {
		dc.broadcastLog("[Supervisor] Using seed nemesis block from chainconfig...")
		if err := copyFile(defaultNemesisDat, backupDat); err != nil {
			return fmt.Errorf("failed to read chainconfig 00001.dat: %w", err)
		}
		if err := copyFile(defaultNemesisHashes, backupHashes); err != nil {
			return fmt.Errorf("failed to read chainconfig hashes.dat: %w", err)
		}
	} else {
		// 2. Download from official GitHub repo if no safe local copy exists
		dc.broadcastLog("[Supervisor] No valid local nemesis block found. Downloading official genesis block from GitHub...")
		if err := downloadFile(Nemesis00001Url, backupDat); err != nil {
			return fmt.Errorf("failed to download official nemesis block (00001.dat): %w", err)
		}
		if err := downloadFile(NemesisHashesUrl, backupHashes); err != nil {
			return fmt.Errorf("failed to download official nemesis hashes (hashes.dat): %w", err)
		}
		// Also seed chainconfig/data for future resets
		_ = os.MkdirAll(filepath.Dir(defaultNemesisDat), 0755)
		_ = copyFile(backupDat, defaultNemesisDat)
		_ = copyFile(backupHashes, defaultNemesisHashes)
	}

	// 3. Delete data directory contents (following reset.sh: "for dir in data/*; do rm -rf $dir; done")
	dc.broadcastLog(fmt.Sprintf("[Supervisor] Purging blockchain data directory at %s...", targetDataDir))
	if entries, err := os.ReadDir(targetDataDir); err == nil {
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(targetDataDir, entry.Name()))
		}
	} else {
		_ = os.MkdirAll(targetDataDir, 0755)
	}

	// 4. Restore nemesis block (following reset.sh: "mkdir data/00000; mv /tmp/00001.dat /tmp/hashes.dat data/00000/")
	dc.broadcastLog("[Supervisor] Restoring official nemesis block (Block 1)...")
	_ = os.MkdirAll(filepath.Join(targetDataDir, "00000"), 0755)
	if err := copyFile(backupDat, filepath.Join(targetDataDir, "00000", "00001.dat")); err != nil {
		return fmt.Errorf("failed to restore 00001.dat: %w", err)
	}
	if err := copyFile(backupHashes, filepath.Join(targetDataDir, "00000", "hashes.dat")); err != nil {
		return fmt.Errorf("failed to restore hashes.dat: %w", err)
	}

	// 5. Create index.dat with block 1 counter
	if err := os.WriteFile(filepath.Join(targetDataDir, "index.dat"), []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0644); err != nil {
		return fmt.Errorf("failed to write index.dat: %w", err)
	}

	// Clear any leftover lock file
	_ = os.Remove(filepath.Join(targetDataDir, "server.lock"))

	dc.broadcastLog("[Supervisor] Blockchain data reset completed successfully. Starting point set to Block 1.")
	return nil
}

func (dc *ProcessSupervisor) GetSnapshotStatus() SnapshotStatus {
	dc.snapshotMu.RLock()
	defer dc.snapshotMu.RUnlock()
	return dc.snapshotStatus
}

func (dc *ProcessSupervisor) CancelSnapshot() error {
	dc.snapshotMu.Lock()
	defer dc.snapshotMu.Unlock()

	if dc.snapshotCancel != nil {
		dc.snapshotCancel()
		dc.snapshotCancel = nil
	}
	dc.snapshotStatus.Stage = SnapshotStageCancelled
	dc.snapshotStatus.Message = "Snapshot download cancelled by user"
	return nil
}

func (dc *ProcessSupervisor) RestoreSnapshot(snapshotUrl string, targetDataPath string) error {
	if snapshotUrl == "" {
		snapshotUrl = DefaultSnapshotUrl
	}

	// Security: Validate URL scheme and host against SSRF
	u, err := url.Parse(snapshotUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid snapshot URL scheme: %s (only http/https allowed)", snapshotUrl)
	}

	host := u.Hostname()
	allowed := host == "207.180.195.181" ||
		strings.HasSuffix(host, ".xpxsirius.io") ||
		strings.HasSuffix(host, ".proximax.io") ||
		host == "xpxsirius.io" ||
		host == "proximax.io"

	if !allowed {
		return fmt.Errorf("unauthorized snapshot download host: %s", host)
	}

	_ = dc.StopNode()

	targetDir := targetDataPath
	if targetDir == "" {
		targetDir = filepath.Join(dc.chainConfigPath, "data")
	} else if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(dc.chainConfigPath, "..", targetDir)
	}

	_ = os.MkdirAll(targetDir, 0755)

	dc.snapshotMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	dc.snapshotCancel = cancel
	dc.snapshotStatus = SnapshotStatus{
		Stage:   SnapshotStageDownloading,
		Message: "Connecting to snapshot server...",
	}
	dc.snapshotMu.Unlock()

	go func() {
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", snapshotUrl, nil)
		if err != nil {
			dc.setSnapshotError(fmt.Sprintf("Failed to create request: %v", err))
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				dc.setSnapshotCancelled()
				return
			}
			dc.setSnapshotError(fmt.Sprintf("Failed to connect: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			dc.setSnapshotError(fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status))
			return
		}

		totalBytes := resp.ContentLength
		dc.snapshotMu.Lock()
		dc.snapshotStatus.Download.TotalBytes = totalBytes
		dc.snapshotStatus.Message = "Streaming and decompressing snapshot directly into data directory..."
		dc.snapshotMu.Unlock()

		tarCmd := exec.CommandContext(ctx, "tar", "-xJf", "-", "-C", targetDir)
		stdinPipe, err := tarCmd.StdinPipe()
		if err != nil {
			dc.setSnapshotError(fmt.Sprintf("Failed to open tar stdin: %v", err))
			return
		}

		if err := tarCmd.Start(); err != nil {
			dc.setSnapshotError(fmt.Sprintf("Failed to start tar extraction: %v", err))
			return
		}

		buf := make([]byte, 512*1024)
		var downloaded int64
		startTime := time.Now()
		lastUpdate := time.Now()

		for {
			select {
			case <-ctx.Done():
				_ = stdinPipe.Close()
				_ = tarCmd.Process.Kill()
				dc.setSnapshotCancelled()
				return
			default:
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := stdinPipe.Write(buf[:n]); writeErr != nil {
					dc.setSnapshotError(fmt.Sprintf("Extraction write error: %v", writeErr))
					return
				}
				downloaded += int64(n)

				if time.Since(lastUpdate) >= 500*time.Millisecond {
					lastUpdate = time.Now()
					elapsed := time.Since(startTime).Seconds()
					speed := float64(downloaded) / (1024 * 1024 * elapsed)
					var percentage float64
					var eta int64
					if totalBytes > 0 {
						percentage = (float64(downloaded) / float64(totalBytes)) * 100
						remaining := totalBytes - downloaded
						if speed > 0 {
							eta = int64(float64(remaining) / (speed * 1024 * 1024))
						}
					}

					dc.snapshotMu.Lock()
					dc.snapshotStatus.Download.DownloadedBytes = downloaded
					dc.snapshotStatus.Download.Percentage = percentage
					dc.snapshotStatus.Download.SpeedMBs = speed
					dc.snapshotStatus.Download.ETASeconds = eta
					dc.snapshotStatus.Message = fmt.Sprintf("Streaming: %.1f%% (%s) at %.1f MB/s", percentage, formatBytes(downloaded), speed)
					dc.snapshotMu.Unlock()
				}
			}

			if readErr != nil {
				if readErr == io.EOF {
					break
				}
				dc.setSnapshotError(fmt.Sprintf("Download read error: %v", readErr))
				return
			}
		}

		_ = stdinPipe.Close()
		if err := tarCmd.Wait(); err != nil {
			if ctx.Err() == context.Canceled {
				dc.setSnapshotCancelled()
				return
			}
			dc.setSnapshotError(fmt.Sprintf("Extraction failed: %v", err))
			return
		}

		dc.clearLocks(targetDir)

		dc.snapshotMu.Lock()
		dc.snapshotStatus.Stage = SnapshotStageComplete
		dc.snapshotStatus.Message = "Fast-sync snapshot restored successfully!"
		dc.snapshotStatus.Download.Percentage = 100.0
		dc.snapshotMu.Unlock()

		dc.broadcastLog("[Supervisor] Fast-sync snapshot streaming extraction completed successfully.")
	}()

	return nil
}

func (dc *ProcessSupervisor) RestoreLocalSnapshot(localFilePath string, targetDataPath string) error {
	localFilePath = strings.TrimSpace(localFilePath)
	if localFilePath == "" {
		return fmt.Errorf("local snapshot file path cannot be empty")
	}

	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		return fmt.Errorf("snapshot file not found at %s: %w", localFilePath, err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("specified path is a directory, not an archive file: %s", localFilePath)
	}

	targetDir := targetDataPath
	if targetDir == "" {
		targetDir = filepath.Join(dc.chainConfigPath, "data")
	} else if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(dc.chainConfigPath, "..", targetDir)
	}

	_ = dc.StopNode()
	_ = os.MkdirAll(targetDir, 0755)

	dc.snapshotMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	dc.snapshotCancel = cancel
	dc.snapshotStatus = SnapshotStatus{
		Stage:   SnapshotStageExtracting,
		Message: fmt.Sprintf("Preparing extraction of %s...", filepath.Base(localFilePath)),
	}
	dc.snapshotStatus.Download.TotalBytes = fileInfo.Size()
	dc.snapshotMu.Unlock()

	dc.broadcastLog(fmt.Sprintf("[Supervisor] Starting local snapshot restoration from %s (%s)...", filepath.Base(localFilePath), formatBytes(fileInfo.Size())))

	go func() {
		defer cancel()

		file, err := os.Open(localFilePath)
		if err != nil {
			dc.setSnapshotError(fmt.Sprintf("Failed to open file: %v", err))
			return
		}
		defer file.Close()

		lowerName := strings.ToLower(localFilePath)
		totalBytes := fileInfo.Size()
		startTime := time.Now()
		lastUpdate := time.Now()

		// For .tar.xz, use system tar command if tar tool available
		if strings.HasSuffix(lowerName, ".tar.xz") || strings.HasSuffix(lowerName, ".xz") {
			dc.broadcastLog("[Supervisor] Using tar -xJf to decompress XZ archive...")
			tarCmd := exec.CommandContext(ctx, "tar", "-xJf", localFilePath, "-C", targetDir)
			if err := tarCmd.Run(); err != nil {
				if ctx.Err() == context.Canceled {
					dc.setSnapshotCancelled()
					return
				}
				dc.setSnapshotError(fmt.Sprintf("XZ extraction failed: %v", err))
				return
			}
		} else {
			// For .tar.zst, .zst, .tar.gz, .tgz, .tar, use streaming decompressors
			var compReader io.Reader
			var closeComp func() error

			if strings.HasSuffix(lowerName, ".tar.zst") || strings.HasSuffix(lowerName, ".zst") {
				dc.broadcastLog("[Supervisor] Using multi-threaded Zstandard decoder for .tar.zst archive...")
				zstdDecoder, err := zstd.NewReader(file)
				if err != nil {
					dc.setSnapshotError(fmt.Sprintf("Failed to initialize Zstandard decoder: %v", err))
					return
				}
				compReader = zstdDecoder
				closeComp = func() error { zstdDecoder.Close(); return nil }
			} else if strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz") || strings.HasSuffix(lowerName, ".gz") {
				dc.broadcastLog("[Supervisor] Using Gzip decoder for .tar.gz archive...")
				gzReader, err := gzip.NewReader(file)
				if err != nil {
					dc.setSnapshotError(fmt.Sprintf("Failed to initialize Gzip decoder: %v", err))
					return
				}
				compReader = gzReader
				closeComp = gzReader.Close
			} else {
				compReader = file
			}

			if closeComp != nil {
				defer closeComp()
			}

			tarReader := tar.NewReader(compReader)
			var extractedBytes int64

			for {
				select {
				case <-ctx.Done():
					dc.setSnapshotCancelled()
					return
				default:
				}

				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					dc.setSnapshotError(fmt.Sprintf("Tar archive read error: %v", err))
					return
				}

				cleanedPath := filepath.Clean(header.Name)
				if strings.HasPrefix(cleanedPath, "..") || filepath.IsAbs(cleanedPath) {
					continue
				}

				targetFile := filepath.Join(targetDir, cleanedPath)

				switch header.Typeflag {
				case tar.TypeDir:
					_ = os.MkdirAll(targetFile, 0755)
				case tar.TypeReg, tar.TypeRegA:
					_ = os.MkdirAll(filepath.Dir(targetFile), 0755)
					outFile, err := os.OpenFile(targetFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
					if err != nil {
						dc.setSnapshotError(fmt.Sprintf("Failed to create file %s: %v", targetFile, err))
						return
					}
					written, err := io.Copy(outFile, tarReader)
					_ = outFile.Close()
					if err != nil {
						dc.setSnapshotError(fmt.Sprintf("Failed to write file %s: %v", targetFile, err))
						return
					}
					extractedBytes += written

					if time.Since(lastUpdate) >= 500*time.Millisecond {
						lastUpdate = time.Now()
						elapsed := time.Since(startTime).Seconds()
						speed := float64(extractedBytes) / (1024 * 1024 * elapsed)
						dc.snapshotMu.Lock()
						dc.snapshotStatus.Download.DownloadedBytes = extractedBytes
						dc.snapshotStatus.Download.SpeedMBs = speed
						dc.snapshotStatus.Message = fmt.Sprintf("Extracting: %s (%s processed at %.1f MB/s)", filepath.Base(cleanedPath), formatBytes(extractedBytes), speed)
						if totalBytes > 0 {
							currentPos, _ := file.Seek(0, io.SeekCurrent)
							if currentPos > 0 {
								dc.snapshotStatus.Download.Percentage = (float64(currentPos) / float64(totalBytes)) * 100.0
							}
						}
						dc.snapshotMu.Unlock()
					}
				}
			}
		}

		dc.clearLocks(targetDir)

		dc.snapshotMu.Lock()
		dc.snapshotStatus.Stage = SnapshotStageComplete
		dc.snapshotStatus.Message = fmt.Sprintf("Local snapshot '%s' restored successfully!", filepath.Base(localFilePath))
		dc.snapshotStatus.Download.Percentage = 100.0
		dc.snapshotMu.Unlock()

		dc.broadcastLog(fmt.Sprintf("[Supervisor] Local snapshot '%s' restoration completed successfully.", filepath.Base(localFilePath)))
	}()

	return nil
}

func (dc *ProcessSupervisor) setSnapshotError(errMsg string) {
	dc.snapshotMu.Lock()
	defer dc.snapshotMu.Unlock()
	dc.snapshotStatus.Stage = SnapshotStageError
	dc.snapshotStatus.ErrorMessage = errMsg
	dc.snapshotStatus.Message = errMsg
}

func (dc *ProcessSupervisor) setSnapshotCancelled() {
	dc.snapshotMu.Lock()
	defer dc.snapshotMu.Unlock()
	dc.snapshotStatus.Stage = SnapshotStageCancelled
	dc.snapshotStatus.Message = "Snapshot restored cancelled."
}

func (dc *ProcessSupervisor) GetDataBackupStatus() DataBackupStatus {
	dc.dataBackupMu.RLock()
	defer dc.dataBackupMu.RUnlock()
	return dc.dataBackupStatus
}

func (dc *ProcessSupervisor) CancelDataBackup() error {
	dc.dataBackupMu.Lock()
	defer dc.dataBackupMu.Unlock()

	if dc.dataBackupCancel != nil {
		dc.dataBackupCancel()
		dc.dataBackupCancel = nil
	}
	dc.dataBackupStatus.Stage = DataBackupStageCancelled
	dc.dataBackupStatus.Message = "Data backup cancelled by user."
	return nil
}

func (dc *ProcessSupervisor) setDataBackupError(errMsg string) {
	dc.dataBackupMu.Lock()
	defer dc.dataBackupMu.Unlock()
	dc.dataBackupStatus.Stage = DataBackupStageError
	dc.dataBackupStatus.ErrorMessage = errMsg
	dc.dataBackupStatus.Message = errMsg
}

func (dc *ProcessSupervisor) setDataBackupCancelled() {
	dc.dataBackupMu.Lock()
	defer dc.dataBackupMu.Unlock()
	dc.dataBackupStatus.Stage = DataBackupStageCancelled
	dc.dataBackupStatus.Message = "Data backup was cancelled."
}

func (dc *ProcessSupervisor) CreateDataBackup(sourceDataPath, targetDestPath, format string) error {
	srcDir := sourceDataPath
	if srcDir == "" {
		srcDir = filepath.Join(dc.chainConfigPath, "data")
	}
	if !filepath.IsAbs(srcDir) {
		if abs, err := filepath.Abs(srcDir); err == nil {
			srcDir = abs
		}
	}

	srcInfo, err := os.Stat(srcDir)
	if err != nil || !srcInfo.IsDir() {
		return fmt.Errorf("source data directory does not exist or is not a directory: %s", srcDir)
	}

	destDir := targetDestPath
	if destDir == "" {
		destDir = filepath.Dir(srcDir)
	}
	if !filepath.IsAbs(destDir) {
		if abs, err := filepath.Abs(destDir); err == nil {
			destDir = abs
		}
	}

	cleanFmt := strings.ToLower(strings.TrimSpace(format))
	isZstd := cleanFmt == "zst" || cleanFmt == "zstandard" || cleanFmt == "tar.zst"
	if cleanFmt == "" {
		if strings.HasSuffix(destDir, ".tar.gz") || strings.HasSuffix(destDir, ".tgz") {
			isZstd = false
		} else {
			isZstd = true
		}
	}

	var targetFilePath string
	if strings.HasSuffix(destDir, ".tar.zst") || strings.HasSuffix(destDir, ".zst") {
		isZstd = true
		targetFilePath = destDir
		if err := os.MkdirAll(filepath.Dir(targetFilePath), 0755); err != nil {
			return fmt.Errorf("cannot create destination directory: %w", err)
		}
	} else if strings.HasSuffix(destDir, ".tar.gz") || strings.HasSuffix(destDir, ".tgz") || strings.HasSuffix(destDir, ".tar") {
		isZstd = false
		targetFilePath = destDir
		if err := os.MkdirAll(filepath.Dir(targetFilePath), 0755); err != nil {
			return fmt.Errorf("cannot create destination directory: %w", err)
		}
	} else {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("cannot create destination directory: %w", err)
		}
		timestamp := time.Now().Format("2006-01-02-150405")
		if isZstd {
			targetFilePath = filepath.Join(destDir, fmt.Sprintf("sirius-data-backup-%s.tar.zst", timestamp))
		} else {
			targetFilePath = filepath.Join(destDir, fmt.Sprintf("sirius-data-backup-%s.tar.gz", timestamp))
		}
	}

	dc.dataBackupMu.Lock()
	if dc.dataBackupStatus.Stage == DataBackupStageBackingUp {
		dc.dataBackupMu.Unlock()
		return fmt.Errorf("a data backup operation is already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	dc.dataBackupCancel = cancel
	dc.dataBackupStatus = DataBackupStatus{
		Stage:      DataBackupStageBackingUp,
		TargetFile: targetFilePath,
		Message:    "Scanning data directory for backup...",
	}
	dc.dataBackupMu.Unlock()

	go func() {
		defer cancel()
		fmtDesc := "Zstandard Multi-Threaded (.tar.zst)"
		if !isZstd {
			fmtDesc = "Gzip (.tar.gz)"
		}
		dc.broadcastLog(fmt.Sprintf("[Data Backup] Starting %s backup of %s to %s ...", fmtDesc, srcDir, targetFilePath))

		var totalBytes int64
		var fileList []string

		_ = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.Name() == "server.lock" || strings.HasSuffix(info.Name(), ".tmp") {
				return nil
			}
			if !info.IsDir() {
				totalBytes += info.Size()
			}
			fileList = append(fileList, path)
			return nil
		})

		dc.dataBackupMu.Lock()
		dc.dataBackupStatus.TotalBytes = totalBytes
		dc.dataBackupStatus.Message = fmt.Sprintf("Backing up %d items (%s)...", len(fileList), formatBytes(totalBytes))
		dc.dataBackupMu.Unlock()

		outFile, err := os.Create(targetFilePath)
		if err != nil {
			dc.setDataBackupError(fmt.Sprintf("Failed to create backup archive file: %v", err))
			return
		}
		defer outFile.Close()

		var compWriter io.WriteCloser
		if isZstd {
			zw, errZ := zstd.NewWriter(outFile, zstd.WithEncoderConcurrency(0), zstd.WithEncoderLevel(zstd.SpeedDefault))
			if errZ != nil {
				dc.setDataBackupError(fmt.Sprintf("Failed to initialize zstd compressor: %v", errZ))
				return
			}
			compWriter = zw
		} else {
			compWriter = gzip.NewWriter(outFile)
		}
		defer compWriter.Close()

		tw := tar.NewWriter(compWriter)
		defer tw.Close()

		var backedUpBytes int64
		buf := make([]byte, 512*1024)
		lastUpdate := time.Now()

		for _, path := range fileList {
			select {
			case <-ctx.Done():
				_ = tw.Close()
				_ = compWriter.Close()
				_ = outFile.Close()
				_ = os.Remove(targetFilePath)
				dc.setDataBackupCancelled()
				return
			default:
			}

			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			relPath, err := filepath.Rel(srcDir, path)
			if err != nil {
				relPath = filepath.Base(path)
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				continue
			}
			header.Name = relPath

			if err := tw.WriteHeader(header); err != nil {
				dc.setDataBackupError(fmt.Sprintf("Failed writing tar header for %s: %v", relPath, err))
				return
			}

			if info.IsDir() {
				continue
			}

			f, err := os.Open(path)
			if err != nil {
				continue
			}

			for {
				select {
				case <-ctx.Done():
					f.Close()
					_ = tw.Close()
					_ = compWriter.Close()
					_ = outFile.Close()
					_ = os.Remove(targetFilePath)
					dc.setDataBackupCancelled()
					return
				default:
				}

				n, rErr := f.Read(buf)
				if n > 0 {
					if _, wErr := tw.Write(buf[:n]); wErr != nil {
						f.Close()
						dc.setDataBackupError(fmt.Sprintf("Failed writing to backup archive: %v", wErr))
						return
					}
					backedUpBytes += int64(n)

					if time.Since(lastUpdate) >= 300*time.Millisecond {
						lastUpdate = time.Now()
						var pct float64
						if totalBytes > 0 {
							pct = (float64(backedUpBytes) / float64(totalBytes)) * 100.0
						}
						dc.dataBackupMu.Lock()
						dc.dataBackupStatus.BackedUpBytes = backedUpBytes
						dc.dataBackupStatus.Percentage = pct
						dc.dataBackupStatus.CurrentFile = relPath
						dc.dataBackupStatus.Message = fmt.Sprintf("Archived %s / %s (%.1f%%)", formatBytes(backedUpBytes), formatBytes(totalBytes), pct)
						dc.dataBackupMu.Unlock()
					}
				}
				if rErr != nil {
					break
				}
			}
			f.Close()
		}

		_ = tw.Close()
		_ = compWriter.Close()
		_ = outFile.Close()

		dc.dataBackupMu.Lock()
		dc.dataBackupStatus.Stage = DataBackupStageComplete
		dc.dataBackupStatus.BackedUpBytes = backedUpBytes
		dc.dataBackupStatus.Percentage = 100.0
		dc.dataBackupStatus.Message = fmt.Sprintf("Data backup completed successfully! Archive saved to %s (%s)", filepath.Base(targetFilePath), formatBytes(backedUpBytes))
		dc.dataBackupMu.Unlock()

		dc.broadcastLog(fmt.Sprintf("[Data Backup] Full blockchain data backup created successfully: %s (%s)", targetFilePath, formatBytes(backedUpBytes)))
	}()

	return nil
}

func (dc *ProcessSupervisor) CleanLogsAndCache(dataPath string) (int64, error) {
	logsDir := filepath.Join(dc.chainConfigPath, "logs")
	var reclaimed int64

	if entries, err := os.ReadDir(logsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
				info, err := e.Info()
				if err == nil {
					reclaimed += info.Size()
					_ = os.Remove(filepath.Join(logsDir, e.Name()))
				}
			}
		}
	}

	return reclaimed, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func downloadFile(fileUrl, dst string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fileUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading %s: %s", fileUrl, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	tmpFile := dst + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmpFile)
		return err
	}
	out.Close()

	return os.Rename(tmpFile, dst)
}

func hasValidNemesis(dir string) bool {
	f1, err1 := os.Stat(filepath.Join(dir, "00000", "00001.dat"))
	fh, errh := os.Stat(filepath.Join(dir, "00000", "hashes.dat"))
	return err1 == nil && f1.Size() > 0 && errh == nil && fh.Size() > 0
}

func (dc *ProcessSupervisor) EnsureNemesisSeed(targetDataDir string) error {
	genesisDat := filepath.Join(targetDataDir, "00000", "00001.dat")
	genesisHashes := filepath.Join(targetDataDir, "00000", "hashes.dat")
	genesisIndex := filepath.Join(targetDataDir, "index.dat")

	defaultNemesisDat := filepath.Join(dc.chainConfigPath, "data", "00000", "00001.dat")
	defaultNemesisHashes := filepath.Join(dc.chainConfigPath, "data", "00000", "hashes.dat")

	_ = os.MkdirAll(filepath.Join(targetDataDir, "00000"), 0755)

	needs00001 := true
	if fi, err := os.Stat(genesisDat); err == nil && fi.Size() > 0 {
		needs00001 = false
	}
	needsHashes := true
	if fi, err := os.Stat(genesisHashes); err == nil && fi.Size() > 0 {
		needsHashes = false
	}

	if needs00001 {
		if fi, err := os.Stat(defaultNemesisDat); err == nil && fi.Size() > 0 {
			_ = copyFile(defaultNemesisDat, genesisDat)
		} else {
			dc.broadcastLog("[Supervisor] Fetching nemesis 00001.dat from official GitHub repository...")
			if err := downloadFile(Nemesis00001Url, genesisDat); err != nil {
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Warning: Failed to download 00001.dat: %v", err))
			} else {
				_ = os.MkdirAll(filepath.Dir(defaultNemesisDat), 0755)
				_ = copyFile(genesisDat, defaultNemesisDat)
			}
		}
	}

	if needsHashes {
		if fi, err := os.Stat(defaultNemesisHashes); err == nil && fi.Size() > 0 {
			_ = copyFile(defaultNemesisHashes, genesisHashes)
		} else {
			dc.broadcastLog("[Supervisor] Fetching nemesis hashes.dat from official GitHub repository...")
			if err := downloadFile(NemesisHashesUrl, genesisHashes); err != nil {
				dc.broadcastLog(fmt.Sprintf("[Supervisor] Warning: Failed to download hashes.dat: %v", err))
			} else {
				_ = os.MkdirAll(filepath.Dir(defaultNemesisHashes), 0755)
				_ = copyFile(genesisHashes, defaultNemesisHashes)
			}
		}
	}

	if _, err := os.Stat(genesisIndex); os.IsNotExist(err) {
		_ = os.WriteFile(genesisIndex, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0644)
	}

	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getPlatformArchLabel() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "Native Apple Silicon ARM64"
		}
		return "Native macOS Intel x86_64"
	case "linux":
		if runtime.GOARCH == "amd64" {
			return "Native Linux x86_64"
		}
		return fmt.Sprintf("Native Linux %s", runtime.GOARCH)
	case "windows":
		return fmt.Sprintf("Native Windows %s", runtime.GOARCH)
	default:
		return fmt.Sprintf("Native %s %s", runtime.GOOS, runtime.GOARCH)
	}
}

func getSystemNetworkBytes() (int64, int64, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/net/dev")
		if err != nil {
			return 0, 0, err
		}
		var totalIn, totalOut int64
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if lineNum <= 2 {
				continue
			}
			line := strings.TrimSpace(scanner.Text())
			parts := strings.Split(line, ":")
			if len(parts) < 2 {
				continue
			}
			iface := strings.TrimSpace(parts[0])
			if strings.HasPrefix(iface, "lo") {
				continue
			}
			fields := strings.Fields(parts[1])
			if len(fields) >= 9 {
				ib, _ := strconv.ParseInt(fields[0], 10, 64)
				ob, _ := strconv.ParseInt(fields[8], 10, 64)
				totalIn += ib
				totalOut += ob
			}
		}
		return totalIn, totalOut, nil
	}

	out, err := exec.Command("netstat", "-b", "-i", "-n").Output()
	if err != nil {
		return 0, 0, err
	}
	lines := strings.Split(string(out), "\n")
	var totalIn, totalOut int64
	seenIface := make(map[string]bool)
	for _, line := range lines {
		if !strings.Contains(line, "<Link#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		iface := fields[0]
		if strings.HasPrefix(iface, "lo") || seenIface[iface] {
			continue
		}
		seenIface[iface] = true
		ib, _ := strconv.ParseInt(fields[6], 10, 64)
		ob, _ := strconv.ParseInt(fields[9], 10, 64)
		totalIn += ib
		totalOut += ob
	}
	return totalIn, totalOut, nil
}

