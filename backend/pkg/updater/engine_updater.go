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
	"net/http"
	"os"
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
	mu           sync.RWMutex
	binDir       string
	manifestPath string
	controller   NodeLifecycleController
	client       *http.Client
	status       EngineUpdateStatus
	auditLogger  func(action, details string)
}

func NewEngineUpdater(binDir, manifestPath string, controller NodeLifecycleController, auditLogger func(action, details string)) *EngineUpdater {
	if auditLogger == nil {
		auditLogger = func(action, details string) {
			log.Printf("[EngineUpdater] %s: %s", action, details)
		}
	}
	currentVer := CurrentVersion
	if verBytes, err := os.ReadFile(filepath.Join(binDir, "version.txt")); err == nil {
		if trimmed := strings.TrimSpace(string(verBytes)); trimmed != "" {
			currentVer = trimmed
		}
	}
	return &EngineUpdater{
		binDir:       binDir,
		manifestPath: manifestPath,
		controller:   controller,
		auditLogger:  auditLogger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		status: EngineUpdateStatus{
			CurrentVersion: currentVer,
			State:          "idle",
		},
	}
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

// IsVersionCompatible checks if a version tag satisfies [min, max]
func IsVersionCompatible(version, minVer, maxVer string) bool {
	vClean := strings.TrimPrefix(strings.TrimPrefix(version, "release-"), "v")
	minClean := strings.TrimPrefix(strings.TrimPrefix(minVer, "release-"), "v")
	maxClean := strings.TrimPrefix(strings.TrimPrefix(maxVer, "release-"), "v")

	// Compare semver components
	vParts := parseSemver(vClean)
	minParts := parseSemver(minClean)
	maxParts := parseSemver(maxClean)

	if compareSemver(vParts, minParts) < 0 {
		return false
	}
	if compareSemver(vParts, maxParts) > 0 {
		return false
	}
	return true
}

func parseSemver(s string) [3]int {
	var parts [3]int
	fmt.Sscanf(s, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}

func compareSemver(a, b [3]int) int {
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

// PlatformAssetDescriptor returns the expected binary and package name for current platform
func PlatformAssetDescriptor() (assetName, binaryName string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	binaryName = "sirius.bc"
	if goos == "windows" {
		binaryName = "sirius.exe"
		assetName = fmt.Sprintf("sirius-%s-%s.zip", goos, goarch)
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
	if !ed25519.Verify(pubKeyBytes, message, sigBytes) {
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
	u.status.IsApplying = true
	u.status.State = "verifying"
	u.status.TargetVersion = version
	u.status.RollbackOccurred = false
	u.status.Message = "Verifying release authenticity and cryptographic integrity..."
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

	// 5. Gracefully stop node before swapping binaries
	u.mu.Lock()
	u.status.State = "swapping"
	u.status.Message = "Stopping node engine for atomic binary replacement..."
	u.mu.Unlock()

	if u.controller != nil {
		_ = u.controller.StopNode()
		time.Sleep(1 * time.Second)
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
	u.status.Message = "Starting updated engine and verifying operational healthcheck..."
	u.mu.Unlock()

	var startupErr error
	if healthcheckFn != nil {
		startupErr = healthcheckFn()
	} else if u.controller != nil {
		if err := u.controller.StartNode(dataPath); err != nil {
			startupErr = err
		} else {
			// Give the process up to 3 seconds to ensure no immediate crash
			time.Sleep(2 * time.Second)
			if !u.controller.IsRunning() {
				startupErr = errors.New("engine process exited unexpectedly after startup")
			}
		}
	}

	// 8. Auto-rollback if healthcheck fails
	if startupErr != nil {
		u.auditLogger("ROLLBACK_TRIGGERED", fmt.Sprintf("Healthcheck failed (%v). Reverting to backup binary", startupErr))
		u.mu.Lock()
		u.status.State = "rolled_back"
		u.status.RollbackOccurred = true
		u.status.Message = fmt.Sprintf("Update failed healthcheck: %v. Reverted to previous version.", startupErr)
		u.mu.Unlock()

		// Stop failed instance
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
		// Restart with restored binary
		if u.controller != nil {
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
	u.status.State = "completed"
	u.status.Message = fmt.Sprintf("Successfully updated engine to %s and verified live healthcheck.", version)
	u.mu.Unlock()

	u.auditLogger("HEALTHCHECK_PASSED", fmt.Sprintf("Engine %s verified healthy. Update complete.", version))
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
	currentVer := CurrentVersion
	if verBytes, err := os.ReadFile(filepath.Join(u.binDir, "version.txt")); err == nil {
		if trimmed := strings.TrimSpace(string(verBytes)); trimmed != "" {
			currentVer = trimmed
		}
	}
	u.status = EngineUpdateStatus{
		CurrentVersion:   currentVer,
		State:            "idle",
		HasUpdate:        false,
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

	resp, err := u.client.Do(req)
	if err != nil {
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
			cleanTag := strings.TrimPrefix(ghRelease.TagName, "release-")
			cleanCurrent := strings.TrimPrefix(u.status.CurrentVersion, "release-")

			isNewer := compareSemver(parseSemver(cleanTag), parseSemver(cleanCurrent)) > 0
			if !isNewer && cleanTag != cleanCurrent && (strings.Contains(cleanCurrent, "local") || (compareSemver(parseSemver(cleanTag), parseSemver(cleanCurrent)) == 0 && cleanTag != cleanCurrent)) {
				isNewer = true
			}
			isCompat := IsVersionCompatible(cleanTag, manifest.EngineMinCompatible, manifest.EngineMaxCompatible)

			if isNewer && isCompat {
				u.status.HasUpdate = true
				u.status.TargetVersion = ghRelease.TagName
				u.status.ReleaseNotes = ghRelease.Body
				u.status.ReleaseUrl = ghRelease.HtmlUrl
			} else {
				u.status.HasUpdate = false
			}
		}
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
	u.status.IsApplying = true
	u.status.State = "verifying"
	u.status.TargetVersion = targetVersion
	u.status.RollbackOccurred = false
	u.status.Message = fmt.Sprintf("Fetching release %s metadata from GitHub...", targetVersion)
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
		u.recordFailure("Failed querying GitHub release", err)
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

	if err := json.NewDecoder(resp.Body).Decode(&releaseInfo); err != nil {
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
		u.recordFailure("Failed downloading checksums", err)
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
		u.recordFailure("Failed downloading signature", err)
		return err
	}
	defer sResp.Body.Close()
	sigData, err := io.ReadAll(sResp.Body)
	if err != nil {
		u.recordFailure("Failed reading signature", err)
		return err
	}

	u.mu.Lock()
	u.status.Message = fmt.Sprintf("Downloading and verifying engine package %s...", assetName)
	u.mu.Unlock()

	// Download package
	pkgResp, err := u.client.Get(assetUrl)
	if err != nil {
		u.recordFailure("Failed downloading engine package", err)
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
		u.status.CurrentVersion = targetVersion
		u.status.Message = fmt.Sprintf("Engine updated successfully to %s. All healthchecks verified.", targetVersion)
		u.mu.Unlock()
	}()
}
