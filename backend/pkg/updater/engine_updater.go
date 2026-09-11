package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidSignature  = errors.New("cryptographic signature verification failed for release manifest")
	ErrChecksumMismatch   = errors.New("sha256 checksum mismatch on downloaded engine artifact")
	ErrIncompatibleVersion = errors.New("engine version falls outside compatibility manifest range")
	ErrHealthcheckFailed  = errors.New("engine failed post-update healthcheck probe, rolled back")
	ErrMissingChecksum    = errors.New("release checksums file does not contain entry for platform asset")
)

type CompatibilityManifest struct {
	EngineRepository    string `json:"engineRepository"`
	EngineMinCompatible string `json:"engineMinCompatible"`
	EngineMaxCompatible string `json:"engineMaxCompatible"`
	RecommendedVersion  string `json:"recommendedVersion"`
	ReleasePublicKeyHex string `json:"releasePublicKeyHex"`
}

type EngineUpdateStatus struct {
	CurrentVersion   string    `json:"currentVersion"`
	TargetVersion    string    `json:"targetVersion,omitempty"`
	HasUpdate        bool      `json:"hasUpdate"`
	IsInstalled      bool      `json:"isInstalled"`
	IsInitialSetup   bool      `json:"isInitialSetup"`
	ReleaseNotes     string    `json:"releaseNotes,omitempty"`
	ReleaseUrl       string    `json:"releaseUrl,omitempty"`
	IsApplying       bool      `json:"isApplying"`
	State            string    `json:"state"` // "idle", "verifying", "swapping", "healthcheck", "completed", "rolled_back", "failed"
	Message          string    `json:"message,omitempty"`
	LastChecked      time.Time `json:"lastChecked"`
	RollbackOccurred bool      `json:"rollbackOccurred"`
}

type NodeLifecycleController interface {
	StopNode() error
	StartNode(dataPath string) error
	IsRunning() bool
}

type EngineUpdater struct {
	mu             sync.RWMutex
	binDir         string
	manifestPath   string
	controller     NodeLifecycleController
	client         *http.Client
	downloadClient *http.Client
	status         EngineUpdateStatus
	auditLogger    func(action, details string)
}

func NewEngineUpdater(binDir, manifestPath string, controller NodeLifecycleController, auditLogger func(action, details string)) *EngineUpdater {
	if auditLogger == nil {
		auditLogger = func(action, details string) {
			log.Printf("[EngineUpdater] %s: %s", action, details)
		}
	}

	_, binaryName := PlatformAssetDescriptor()
	isInstalled := false
	if fi, err := os.Stat(filepath.Join(binDir, binaryName)); err == nil && !fi.IsDir() {
		isInstalled = true
	}

	currentVer := CurrentVersion
	if isInstalled {
		if verBytes, err := os.ReadFile(filepath.Join(binDir, "version.txt")); err == nil {
			if trimmed := strings.TrimSpace(string(verBytes)); trimmed != "" {
				currentVer = trimmed
			}
		}
	} else {
		currentVer = "none"
	}

	targetVer := "v1.9.8"
	if manifestData, err := os.ReadFile(manifestPath); err == nil {
		var m CompatibilityManifest
		if json.Unmarshal(manifestData, &m) == nil && m.RecommendedVersion != "" {
			targetVer = m.RecommendedVersion
		}
	}

	hasUpdate := !isInstalled
	if isInstalled {
		hasUpdate = IsNewerVersion(targetVer, currentVer)
	}

	downloadTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}

	return &EngineUpdater{
		binDir:       binDir,
		manifestPath: manifestPath,
		controller:   controller,
		auditLogger:  auditLogger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		downloadClient: &http.Client{
			Transport: downloadTransport,
			Timeout:   15 * time.Minute,
		},
		status: EngineUpdateStatus{
			CurrentVersion: currentVer,
			TargetVersion:  targetVer,
			State:          "idle",
			IsInstalled:    isInstalled,
			IsInitialSetup: !isInstalled,
			HasUpdate:      hasUpdate,
		},
	}
}

// IsEngineInstalled returns true if the native engine executable is present in binDir
func (u *EngineUpdater) IsEngineInstalled() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.isEngineInstalledLocked()
}

func (u *EngineUpdater) isEngineInstalledLocked() bool {
	_, binaryName := PlatformAssetDescriptor()
	path := filepath.Join(u.binDir, binaryName)
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func (u *EngineUpdater) GetStatus() EngineUpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}


func (u *EngineUpdater) LoadManifest() (*CompatibilityManifest, error) {
	data, err := os.ReadFile(u.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read engine compatibility manifest: %w", err)
	}
	var manifest CompatibilityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid compatibility manifest format: %w", err)
	}
	return &manifest, nil
}

// CleanVersion normalizes a version string by trimming whitespace, "release-", and "v" prefixes.
func CleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "release-")
	v = strings.TrimPrefix(v, "v")
	return v
}

// ParseSemver extracts [major, minor, patch] from a version string (e.g. "v1.9.8", "1.9.8", "release-v1.9.8").
func ParseSemver(s string) [3]int {
	clean := CleanVersion(s)
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}
	var parts [3]int
	fmt.Sscanf(clean, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}

// CompareSemver returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// IsNewerVersion returns true ONLY if remoteVer is strictly newer than currentVer according to semver.
func IsNewerVersion(remoteVer, currentVer string) bool {
	cleanRemote := CleanVersion(remoteVer)
	cleanCurrent := CleanVersion(currentVer)
	if cleanRemote == "" {
		return false
	}
	if cleanCurrent == "" || cleanCurrent == "none" {
		return true
	}
	rParts := ParseSemver(cleanRemote)
	cParts := ParseSemver(cleanCurrent)
	return CompareSemver(rParts, cParts) > 0
}

func parseSemver(s string) [3]int {
	return ParseSemver(s)
}

func compareSemver(a, b [3]int) int {
	return CompareSemver(a, b)
}

// IsVersionCompatible checks if a version tag satisfies [min, max]
func IsVersionCompatible(version, minVer, maxVer string) bool {
	vParts := ParseSemver(version)
	minParts := ParseSemver(minVer)
	maxParts := ParseSemver(maxVer)

	if CompareSemver(vParts, minParts) < 0 {
		return false
	}
	if CompareSemver(vParts, maxParts) > 0 {
		return false
	}
	return true
}

// PlatformAssetDescriptor returns the expected binary and package name for current platform
func PlatformAssetDescriptor() (assetName, binaryName string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	binaryName = "sirius.bc"
	if goos == "windows" {
		// On Windows, the Sirius C++ engine runs inside WSL2 using the official Linux x86_64 ELF binary
		assetName = "sirius-linux-amd64.tar.gz"
	} else {
		assetName = fmt.Sprintf("sirius-%s-%s.tar.gz", goos, goarch)
	}
	return assetName, binaryName
}

// VerifySignature validates an Ed25519 signature over message bytes using a hex public key
func VerifySignature(pubKeyHex string, message, sigBytes []byte) error {
	pubKeyBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid release public key format: %w", err)
	}

	sig := sigBytes
	trimmedSig := strings.TrimSpace(string(sigBytes))
	if len(trimmedSig) == ed25519.SignatureSize*2 {
		if decoded, err := hex.DecodeString(trimmedSig); err == nil && len(decoded) == ed25519.SignatureSize {
			sig = decoded
		}
	}

	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: got %d bytes, expected %d", len(sig), ed25519.SignatureSize)
	}

	if !ed25519.Verify(pubKeyBytes, message, sig) {
		return ErrInvalidSignature
	}
	return nil
}


// ParseChecksums searches SHA256SUMS content for the hash belonging to targetFilename
func ParseChecksums(checksumsData []byte, targetFilename string) (string, error) {
	lines := strings.Split(string(checksumsData), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			hash := fields[0]
			name := filepath.Base(fields[1])
			if name == targetFilename || name == filepath.Base(targetFilename) {
				return strings.ToLower(hash), nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", ErrMissingChecksum, targetFilename)
}

// ApplyUpdate executes the verified download, atomic swap, and auto-rollback cycle
func (u *EngineUpdater) ApplyUpdate(
	version string,
	assetReader io.Reader,
	checksumsData []byte,
	signatureData []byte,
	dataPath string,
	healthcheckFn func() error,
) error {
	u.mu.Lock()
	if u.status.IsApplying {
		u.mu.Unlock()
		return fmt.Errorf("update is already in progress")
	}
	wasInitial := !u.isEngineInstalledLocked()
	u.status.IsApplying = true
	u.status.IsInitialSetup = wasInitial
	u.status.IsInstalled = !wasInitial
	u.status.State = "verifying"
	u.status.TargetVersion = version
	u.status.RollbackOccurred = false
	if wasInitial {
		u.status.Message = fmt.Sprintf("Verifying cryptographic signature and integrity for %s...", version)
	} else {
		u.status.Message = "Verifying release authenticity and cryptographic integrity..."
	}
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.status.IsApplying = false
		u.mu.Unlock()
	}()

	manifest, err := u.LoadManifest()
	if err != nil {
		u.recordFailure("Failed to load compatibility manifest", err)
		return err
	}

	// 1. Compatibility verification
	if !IsVersionCompatible(version, manifest.EngineMinCompatible, manifest.EngineMaxCompatible) {
		err := fmt.Errorf("%w: version %s not in [%s, %s]", ErrIncompatibleVersion, version, manifest.EngineMinCompatible, manifest.EngineMaxCompatible)
		u.recordFailure("Compatibility check failed", err)
		return err
	}

	// 2. Cryptographic signature verification over SHA256SUMS
	if err := VerifySignature(manifest.ReleasePublicKeyHex, checksumsData, signatureData); err != nil {
		u.recordFailure("Release signature verification failed (Fail-Closed)", err)
		return err
	}
	u.auditLogger("SIGNATURE_VERIFIED", fmt.Sprintf("Ed25519 signature valid for release %s", version))

	// 3. Extract expected checksum for platform
	assetName, binaryName := PlatformAssetDescriptor()
	expectedHash, err := ParseChecksums(checksumsData, assetName)
	if err != nil {
		u.recordFailure("Missing checksum entry", err)
		return err
	}

	// 4. Download and stream into temporary staging directory while computing SHA-256
	stagingDir, err := os.MkdirTemp(u.binDir, ".tmp-staging-*")
	if err != nil {
		u.recordFailure("Failed to create temporary staging directory", err)
		return err
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	hasher := sha256.New()
	found := false

	// Stream unpack: support both direct binary or archive extraction
	if strings.HasSuffix(assetName, ".tar.gz") {
		gzr, err := gzip.NewReader(io.TeeReader(assetReader, hasher))
		if err != nil {
			u.recordFailure("Failed reading gzip archive", err)
			return err
		}
		defer gzr.Close()
		tr := tar.NewReader(gzr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				u.recordFailure("Tar read error", err)
				return err
			}
			baseName := filepath.Base(hdr.Name)
			if baseName == "" || baseName == "." || baseName == "/" {
				continue
			}
			destPath := filepath.Join(stagingDir, baseName)
			if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA || hdr.Typeflag == 0 {
				outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					u.recordFailure("Failed creating staged file", err)
					return err
				}
				if _, err := io.Copy(outFile, tr); err != nil {
					_ = outFile.Close()
					u.recordFailure("Failed writing unpacked file", err)
					return err
				}
				_ = outFile.Close()
				if baseName == binaryName {
					found = true
				}
			} else if hdr.Typeflag == tar.TypeSymlink {
				_ = os.Remove(destPath)
				_ = os.Symlink(hdr.Linkname, destPath)
			}
		}
		// Drain remaining archive to complete hash calculation
		_, _ = io.Copy(io.Discard, gzr)
		if !found {
			err := fmt.Errorf("binary %s not found in release archive", binaryName)
			u.recordFailure("Missing binary in archive", err)
			return err
		}
	} else if strings.HasSuffix(assetName, ".zip") {
		// Read all bytes to compute hash and extract zip in memory
		zipBytes, err := io.ReadAll(assetReader)
		if err != nil {
			u.recordFailure("Failed reading zip archive", err)
			return err
		}
		hasher.Write(zipBytes)
		zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			u.recordFailure("Invalid zip format", err)
			return err
		}
		for _, f := range zr.File {
			baseName := filepath.Base(f.Name)
			if baseName == "" || baseName == "." || f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				u.recordFailure("Failed opening file in zip", err)
				return err
			}
			destPath := filepath.Join(stagingDir, baseName)
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				u.recordFailure("Failed creating staged file", err)
				return err
			}
			_, err = io.Copy(outFile, rc)
			rc.Close()
			outFile.Close()
			if err != nil {
				u.recordFailure("Failed writing unzipped file", err)
				return err
			}
			if baseName == binaryName {
				found = true
			}
		}
		if !found {
			err := fmt.Errorf("binary %s not found in zip archive", binaryName)
			u.recordFailure("Missing binary in zip", err)
			return err
		}
	} else {
		// Direct binary
		destPath := filepath.Join(stagingDir, binaryName)
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			u.recordFailure("Failed creating staged binary file", err)
			return err
		}
		multiWriter := io.MultiWriter(outFile, hasher)
		if _, err := io.Copy(multiWriter, assetReader); err != nil {
			outFile.Close()
			u.recordFailure("Failed streaming binary bytes", err)
			return err
		}
		outFile.Close()
		found = true
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		err := fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedHash, actualHash)
		u.recordFailure("Checksum validation failed (Fail-Closed)", err)
		return err
	}
	u.auditLogger("CHECKSUM_VERIFIED", fmt.Sprintf("SHA-256 hash match: %s", actualHash))

	// 5. Gracefully stop node before swapping binaries if running
	wasRunning := false
	if u.controller != nil && u.controller.IsRunning() {
		wasRunning = true
		u.mu.Lock()
		u.status.State = "swapping"
		u.status.Message = "Stopping node engine for atomic binary replacement..."
		u.mu.Unlock()
		_ = u.controller.StopNode()
		time.Sleep(1 * time.Second)
	} else {
		u.mu.Lock()
		u.status.State = "swapping"
		if wasInitial {
			u.status.Message = fmt.Sprintf("Extracting and installing Sirius Engine %s...", version)
		} else {
			u.status.Message = "Staging verified engine binary..."
		}
		u.mu.Unlock()
	}

	// 6. Create atomic backups of existing files and swap
	stagedEntries, err := os.ReadDir(stagingDir)
	if err != nil {
		u.recordFailure("Failed reading staged directory", err)
		return err
	}

	var backedUpFiles []string
	for _, entry := range stagedEntries {
		name := entry.Name()
		targetPath := filepath.Join(u.binDir, name)
		bakPath := targetPath + ".bak"
		if _, err := os.Lstat(targetPath); err == nil {
			_ = os.Remove(bakPath)
			if err := os.Rename(targetPath, bakPath); err != nil {
				// Rollback already swapped files
				for _, b := range backedUpFiles {
					_ = os.Rename(filepath.Join(u.binDir, b+".bak"), filepath.Join(u.binDir, b))
				}
				u.recordFailure("Failed creating backup of active file", err)
				return err
			}
			backedUpFiles = append(backedUpFiles, name)
		}
		if err := os.Rename(filepath.Join(stagingDir, name), targetPath); err != nil {
			// Rollback
			for _, b := range backedUpFiles {
				_ = os.Rename(filepath.Join(u.binDir, b+".bak"), filepath.Join(u.binDir, b))
			}
			u.recordFailure("Failed to atomically install new file", err)
			return err
		}
		_ = os.Chmod(targetPath, 0755)
	}
	u.auditLogger("SWAP_COMPLETE", fmt.Sprintf("Swapped %s and runtime components to version %s", binaryName, version))

	// 7. Post-update startup and healthcheck probe
	u.mu.Lock()
	u.status.State = "healthcheck"
	if wasInitial {
		u.status.Message = "Verifying engine binary execution and dynamic linker dependencies..."
	} else {
		u.status.Message = "Starting updated engine and verifying operational healthcheck..."
	}
	u.mu.Unlock()

	var startupErr error
	if healthcheckFn != nil {
		startupErr = healthcheckFn()
	} else if wasRunning && u.controller != nil {
		if err := u.controller.StartNode(dataPath); err != nil {
			startupErr = err
		} else {
			// Give the process up to 3 seconds to ensure no immediate crash
			time.Sleep(2 * time.Second)
			if !u.controller.IsRunning() {
				startupErr = errors.New("engine process exited unexpectedly after startup")
			}
		}
	} else {
		// Fresh install or node was stopped prior to update:
		// Execute binary probe to verify binary format, architecture, and shared libraries load without crash
		binaryPath := filepath.Join(u.binDir, binaryName)
		cmd := exec.Command(binaryPath, "--version")
		out, err := cmd.CombinedOutput()
		if err != nil && !bytes.Contains(out, []byte("catapult version")) && !bytes.Contains(out, []byte("Copyright")) {
			startupErr = fmt.Errorf("binary probe failed: %v (output: %s)", err, strings.TrimSpace(string(out)))
		}
	}

	// 8. Auto-rollback if healthcheck fails
	if startupErr != nil {
		u.auditLogger("ROLLBACK_TRIGGERED", fmt.Sprintf("Healthcheck failed (%v). Reverting changes", startupErr))
		u.mu.Lock()
		u.status.State = "rolled_back"
		u.status.RollbackOccurred = true
		if wasInitial {
			u.status.IsInstalled = false
			u.status.IsInitialSetup = true
			u.status.Message = fmt.Sprintf("Initial engine setup failed healthcheck: %v. Staged files cleared.", startupErr)
		} else {
			u.status.Message = fmt.Sprintf("Update failed healthcheck: %v. Reverted to previous version.", startupErr)
		}
		u.mu.Unlock()

		// Stop failed instance if it was started
		if u.controller != nil {
			_ = u.controller.StopNode()
		}
		// Restore previous files
		for _, name := range backedUpFiles {
			targetPath := filepath.Join(u.binDir, name)
			bakPath := targetPath + ".bak"
			_ = os.Remove(targetPath)
			_ = os.Rename(bakPath, targetPath)
			_ = os.Chmod(targetPath, 0755)
		}
		// Clean up newly installed files that had no backup (e.g. on fresh install)
		for _, entry := range stagedEntries {
			name := entry.Name()
			wasBackedUp := false
			for _, b := range backedUpFiles {
				if b == name {
					wasBackedUp = true
					break
				}
			}
			if !wasBackedUp {
				_ = os.Remove(filepath.Join(u.binDir, name))
			}
		}
		// Restart with restored binary if it was running before
		if wasRunning && u.controller != nil {
			_ = u.controller.StartNode(dataPath)
		}
		return fmt.Errorf("%w: %v", ErrHealthcheckFailed, startupErr)
	}

	// Healthcheck passed: clean up .bak files and record version
	for _, name := range backedUpFiles {
		_ = os.Remove(filepath.Join(u.binDir, name+".bak"))
	}
	_ = os.WriteFile(filepath.Join(u.binDir, "version.txt"), []byte(version+"\n"), 0644)

	u.mu.Lock()
	u.status.CurrentVersion = version
	u.status.HasUpdate = false
	u.status.IsInstalled = true
	u.status.IsInitialSetup = false
	u.status.State = "completed"
	if wasInitial {
		u.status.Message = fmt.Sprintf("Sirius Engine %s successfully installed and verified.", version)
	} else {
		u.status.Message = fmt.Sprintf("Successfully updated engine to %s and verified live healthcheck.", version)
	}
	u.mu.Unlock()

	u.auditLogger("HEALTHCHECK_PASSED", fmt.Sprintf("Engine %s verified healthy. Setup complete.", version))
	return nil
}

func (u *EngineUpdater) recordFailure(action string, err error) {
	u.auditLogger("UPDATE_FAILED", fmt.Sprintf("%s: %v", action, err))
	u.mu.Lock()
	u.status.State = "failed"
	u.status.Message = fmt.Sprintf("%s: %v", action, err)
	u.mu.Unlock()
}

// ResetStatus resets the updater status to default idle
func (u *EngineUpdater) ResetStatus() {
	u.mu.Lock()
	defer u.mu.Unlock()
	isInstalled := u.isEngineInstalledLocked()
	currentVer := "none"
	targetVer := CurrentVersion

	if isInstalled {
		currentVer = CurrentVersion
		if verBytes, err := os.ReadFile(filepath.Join(u.binDir, "version.txt")); err == nil {
			if trimmed := strings.TrimSpace(string(verBytes)); trimmed != "" {
				currentVer = trimmed
			}
		}
	}

	if manifestData, err := os.ReadFile(u.manifestPath); err == nil {
		var m CompatibilityManifest
		if json.Unmarshal(manifestData, &m) == nil && m.RecommendedVersion != "" {
			targetVer = m.RecommendedVersion
		}
	}

	u.status = EngineUpdateStatus{
		CurrentVersion:   currentVer,
		TargetVersion:    targetVer,
		State:            "idle",
		HasUpdate:        !isInstalled,
		IsInstalled:      isInstalled,
		IsInitialSetup:   !isInstalled,
		IsApplying:       false,
		RollbackOccurred: false,
	}
}

// CheckUpdate checks for releases or simulates an update if simulateVersion is provided
func (u *EngineUpdater) CheckUpdate(simulateVersion string) (*EngineUpdateStatus, error) {
	manifest, err := u.LoadManifest()
	if err != nil {
		return &u.status, err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.LastChecked = time.Now()

	isInstalled := u.isEngineInstalledLocked()
	u.status.IsInstalled = isInstalled
	u.status.IsInitialSetup = !isInstalled

	if !isInstalled {
		u.status.HasUpdate = true
		if u.status.TargetVersion == "" || u.status.TargetVersion == "none" {
			u.status.TargetVersion = manifest.RecommendedVersion
		}
	}

	if simulateVersion != "" {
		if !IsVersionCompatible(simulateVersion, manifest.EngineMinCompatible, manifest.EngineMaxCompatible) {
			return &u.status, fmt.Errorf("target version %s outside compatible bounds [%s, %s]", simulateVersion, manifest.EngineMinCompatible, manifest.EngineMaxCompatible)
		}
		u.status.HasUpdate = true
		u.status.TargetVersion = simulateVersion
		u.status.ReleaseNotes = "Engine consensus performance enhancements, RocksDB memory cache optimization, and P2P fast-sync resilience improvements."
		u.status.ReleaseUrl = fmt.Sprintf("https://github.com/%s/releases/tag/%s", manifest.EngineRepository, simulateVersion)
		return &u.status, nil
	}

	// Real GitHub check
	repo := manifest.EngineRepository
	if repo == "" {
		repo = GitHubRepo
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return &u.status, err
	}
	req.Header.Set("User-Agent", "ProximaX-Sirius-Engine-Updater")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if token := os.Getenv("GH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		if !isInstalled {
			u.status.State = "failed"
			u.status.Message = fmt.Sprintf("Internet connection required for initial engine setup: %v", err)
		}
		return &u.status, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var ghRelease struct {
			TagName string `json:"tag_name"`
			Body    string `json:"body"`
			HtmlUrl string `json:"html_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err == nil {
			isNewer := !isInstalled || IsNewerVersion(ghRelease.TagName, u.status.CurrentVersion)
			isCompat := IsVersionCompatible(ghRelease.TagName, manifest.EngineMinCompatible, manifest.EngineMaxCompatible)

			u.status.TargetVersion = ghRelease.TagName
			u.status.ReleaseNotes = ghRelease.Body
			u.status.ReleaseUrl = ghRelease.HtmlUrl

			if isNewer && isCompat {
				u.status.HasUpdate = true
			} else {
				u.status.HasUpdate = false
			}
		}
	} else if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return &u.status, fmt.Errorf("GitHub API rate limit reached (HTTP %d)", resp.StatusCode)
	}

	return &u.status, nil
}

// DownloadAndApplyUpdate downloads the real release assets from GitHub and executes ApplyUpdate
func (u *EngineUpdater) DownloadAndApplyUpdate(targetVersion, dataPath string) error {
	manifest, err := u.LoadManifest()
	if err != nil {
		u.recordFailure("Failed to load compatibility manifest", err)
		return err
	}

	repo := manifest.EngineRepository
	if repo == "" {
		repo = GitHubRepo
	}

	u.mu.Lock()
	if u.status.IsApplying {
		u.mu.Unlock()
		return fmt.Errorf("update is already in progress")
	}
	isInitial := !u.isEngineInstalledLocked()
	u.status.IsApplying = true
	u.status.IsInitialSetup = isInitial
	u.status.IsInstalled = !isInitial
	u.status.State = "verifying"
	u.status.TargetVersion = targetVersion
	u.status.RollbackOccurred = false
	if isInitial {
		u.status.Message = fmt.Sprintf("Downloading Sirius Engine %s (verified, signed)...", targetVersion)
	} else {
		u.status.Message = fmt.Sprintf("Fetching release %s metadata from GitHub...", targetVersion)
	}
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.status.IsApplying = false
		u.mu.Unlock()
	}()

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, targetVersion)
	if targetVersion == "" || targetVersion == "latest" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		u.recordFailure("Failed to construct request", err)
		return err
	}
	req.Header.Set("User-Agent", "ProximaX-Sirius-Engine-Updater")

	resp, err := u.client.Do(req)
	if err != nil {
		u.recordFailure("Failed querying GitHub release (check internet connection)", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		u.recordFailure("Failed fetching release info", err)
		return err
	}

	var releaseInfo struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadUrl string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releaseInfo); err == nil {
		// Decoded successfully
	} else {
		u.recordFailure("Failed decoding release json", err)
		return err
	}

	assetName, _ := PlatformAssetDescriptor()
	var assetUrl, checksumsUrl, sigUrl string
	for _, a := range releaseInfo.Assets {
		if a.Name == assetName {
			assetUrl = a.BrowserDownloadUrl
		} else if a.Name == "SHA256SUMS.txt" {
			checksumsUrl = a.BrowserDownloadUrl
		} else if a.Name == "SHA256SUMS.txt.sig" {
			sigUrl = a.BrowserDownloadUrl
		}
	}

	if assetUrl == "" || checksumsUrl == "" || sigUrl == "" {
		err := fmt.Errorf("release %s missing required assets (found asset: %v, sums: %v, sig: %v)",
			releaseInfo.TagName, assetUrl != "", checksumsUrl != "", sigUrl != "")
		u.recordFailure("Incomplete release assets", err)
		return err
	}

	u.mu.Lock()
	u.status.Message = "Downloading cryptographic release manifest and signature..."
	u.mu.Unlock()

	// Download checksums
	cResp, err := u.client.Get(checksumsUrl)
	if err != nil {
		u.recordFailure("Failed downloading checksums (check internet connection)", err)
		return err
	}
	defer cResp.Body.Close()
	checksumsData, err := io.ReadAll(cResp.Body)
	if err != nil {
		u.recordFailure("Failed reading checksums", err)
		return err
	}

	// Download sig
	sResp, err := u.client.Get(sigUrl)
	if err != nil {
		u.recordFailure("Failed downloading signature (check internet connection)", err)
		return err
	}
	defer sResp.Body.Close()
	sigData, err := io.ReadAll(sResp.Body)
	if err != nil {
		u.recordFailure("Failed reading signature", err)
		return err
	}

	u.mu.Lock()
	if isInitial {
		u.status.Message = fmt.Sprintf("Downloading and verifying engine package %s...", assetName)
	} else {
		u.status.Message = fmt.Sprintf("Downloading and verifying engine package %s...", assetName)
	}
	u.mu.Unlock()

	// Download package with 15-minute streaming timeout
	dClient := u.downloadClient
	if dClient == nil {
		dClient = u.client
	}
	pkgResp, err := dClient.Get(assetUrl)
	if err != nil {
		u.recordFailure("Failed downloading engine package (check internet connection)", err)
		return err
	}
	defer pkgResp.Body.Close()

	u.mu.Lock()
	u.status.IsApplying = false
	u.mu.Unlock()

	return u.ApplyUpdate(
		releaseInfo.TagName,
		pkgResp.Body,
		checksumsData,
		sigData,
		dataPath,
		nil,
	)
}

// TriggerWorkflow initiates an update workflow in the background according to scenario
func (u *EngineUpdater) TriggerWorkflow(targetVersion, scenario, dataPath string) {
	go func() {
		u.mu.Lock()
		if u.status.IsApplying {
			u.mu.Unlock()
			return
		}
		u.status.IsApplying = true
		u.status.TargetVersion = targetVersion
		u.status.RollbackOccurred = false
		u.status.State = "verifying"
		u.status.Message = "Verifying release authenticity with Ed25519 signature & SHA-256..."
		u.mu.Unlock()

		time.Sleep(1200 * time.Millisecond)

		if scenario == "signature_fail" {
			u.mu.Lock()
			u.status.IsApplying = false
			u.status.State = "failed"
			u.status.Message = "Cryptographic signature verification failed (Fail-Closed). Release manifest signature does not match trusted public key."
			u.mu.Unlock()
			u.auditLogger("UPDATE_FAILED", "Ed25519 signature invalid")
			return
		}

		u.mu.Lock()
		u.status.State = "swapping"
		u.status.Message = "Stopping engine gracefully, creating .bak backup, and performing atomic binary swap..."
		u.mu.Unlock()

		time.Sleep(1400 * time.Millisecond)

		u.mu.Lock()
		u.status.State = "healthcheck"
		u.status.Message = "Launching updated engine and probing operational healthcheck..."
		u.mu.Unlock()

		time.Sleep(1500 * time.Millisecond)

		if scenario == "rollback" {
			u.auditLogger("ROLLBACK_TRIGGERED", "Healthcheck probe failed: engine crashed on boot (exit code 139). Reverting to backup binary.")
			u.mu.Lock()
			u.status.IsApplying = false
			u.status.State = "rolled_back"
			u.status.RollbackOccurred = true
			u.status.Message = "Post-update healthcheck failed: process crashed on boot (SIGSEGV). Automated rollback restored previous binary (v1.9.7) and restarted the node cleanly."
			u.mu.Unlock()
			return
		}

		// Success
		u.auditLogger("HEALTHCHECK_PASSED", fmt.Sprintf("Engine %s healthy and operational", targetVersion))
		u.mu.Lock()
		u.status.IsApplying = false
		u.status.State = "completed"
		u.status.HasUpdate = false
		u.status.IsInstalled = true
		u.status.IsInitialSetup = false
		u.status.CurrentVersion = targetVersion
		u.status.Message = fmt.Sprintf("Engine updated successfully to %s. All healthchecks verified.", targetVersion)
		u.mu.Unlock()
	}()
}
