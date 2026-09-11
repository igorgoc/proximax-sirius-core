package updater

import (
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
	"strings"
	"sync"
	"time"

	"proximax-sirius-core/pkg/supervisor"
)

const (
	CurrentVersion = "v1.9.8"
	GitHubRepo     = "igorgoc/cpp-xpx-chain"
	OfficialBase   = "https://raw.githubusercontent.com/igorgoc/cpp-xpx-chain/master/resources"
)

type ConfigFileDiff struct {
	FileName    string `json:"fileName"`
	Category    string `json:"category"`
	Purpose     string `json:"purpose"`
	Status      string `json:"status"` // "identical", "different", "missing", "error"
	LocalHash   string `json:"localHash,omitempty"`
	RemoteHash  string `json:"remoteHash,omitempty"`
	LocalBytes  int64  `json:"localBytes,omitempty"`
	RemoteBytes int64  `json:"remoteBytes,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ConfigDiffReport struct {
	Repository     string           `json:"repository"`
	OfficialBase   string           `json:"officialBase"`
	CheckedAt      time.Time        `json:"checkedAt"`
	TotalFiles     int              `json:"totalFiles"`
	DifferentCount int              `json:"differentCount"`
	IdenticalCount int              `json:"identicalCount"`
	MissingCount   int              `json:"missingCount"`
	HasDifferences bool             `json:"hasDifferences"`
	Files          []ConfigFileDiff `json:"files"`
}

type UpdateInfo struct {
	CurrentVersion   string            `json:"currentVersion"`
	LatestVersion    string            `json:"latestVersion"`
	HasUpdate        bool              `json:"hasUpdate"`
	ReleaseTitle     string            `json:"releaseTitle,omitempty"`
	ReleaseNotes     string            `json:"releaseNotes,omitempty"`
	ReleaseUrl       string            `json:"releaseUrl,omitempty"`
	PublishedAt      string            `json:"publishedAt,omitempty"`
	LastChecked      time.Time         `json:"lastChecked"`
	OfficialFiles    []string          `json:"officialFiles"`
	IsApplying       bool              `json:"isApplying"`
	UpdateMessage    string            `json:"updateMessage,omitempty"`
	RollbackOccurred bool              `json:"rollbackOccurred"`
	BackupPath       string            `json:"backupPath,omitempty"`
	DiffReport       *ConfigDiffReport `json:"diffReport,omitempty"`
}

type UpdateManager struct {
	mu           sync.RWMutex
	lastInfo     UpdateInfo
	resourcesDir string
	supervisor   *supervisor.ProcessSupervisor
	httpClient   *http.Client
}

func (um *UpdateManager) getRepo() string {
	compatPath := filepath.Join(filepath.Dir(um.resourcesDir), "engine.compat.json")
	if compatBytes, err := os.ReadFile(compatPath); err == nil {
		var m struct {
			EngineRepository string `json:"engineRepository"`
		}
		if json.Unmarshal(compatBytes, &m) == nil && m.EngineRepository != "" {
			return m.EngineRepository
		}
	}
	return GitHubRepo
}

func (um *UpdateManager) getOfficialBase() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/master/resources", um.getRepo())
}

func NewUpdateManager(resourcesDir string, supervisor *supervisor.ProcessSupervisor) *UpdateManager {
	return &UpdateManager{
		resourcesDir: resourcesDir,
		supervisor:   supervisor,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		lastInfo: UpdateInfo{
			CurrentVersion: CurrentVersion,
			LatestVersion:  CurrentVersion,
			HasUpdate:      false,
			OfficialFiles: []string{
				"config-network.properties",
				"peers-api.json",
				"peers-p2p.json",
				"replicators.json",
				"supported-entities.json",
			},
		},
	}
}

// CheckUpdate queries GitHub releases for the latest mainnet onboarding package
func (um *UpdateManager) CheckUpdate() (*UpdateInfo, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	repo := um.getRepo()
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return &um.lastInfo, err
	}
	req.Header.Set("User-Agent", "ProximaX-Sirius-Core-Manager")

	resp, err := um.httpClient.Do(req)
	if err != nil {
		um.lastInfo.LastChecked = time.Now()
		return &um.lastInfo, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var ghRelease struct {
			TagName     string `json:"tag_name"`
			Name        string `json:"name"`
			Body        string `json:"body"`
			HtmlUrl     string `json:"html_url"`
			PublishedAt string `json:"published_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err == nil {
			cleanTag := CleanVersion(ghRelease.TagName)
			tagDisplay := ghRelease.TagName
			if !strings.HasPrefix(tagDisplay, "v") && cleanTag != "" {
				tagDisplay = "v" + cleanTag
			}

			um.lastInfo.LatestVersion = tagDisplay
			um.lastInfo.CurrentVersion = CurrentVersion
			um.lastInfo.ReleaseTitle = ghRelease.Name
			um.lastInfo.ReleaseNotes = ghRelease.Body
			um.lastInfo.ReleaseUrl = ghRelease.HtmlUrl
			um.lastInfo.PublishedAt = ghRelease.PublishedAt
			um.lastInfo.LastChecked = time.Now()

			// Check if latest version is strictly newer using semver
			um.lastInfo.HasUpdate = IsNewerVersion(ghRelease.TagName, CurrentVersion)
		}
	} else {
		um.lastInfo.LastChecked = time.Now()
	}

	return &um.lastInfo, nil
}

func (um *UpdateManager) GetUpdateInfo() UpdateInfo {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.lastInfo
}

// copyFile copies a single file preserving 0600 permissions
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// CheckConfigsDiff inspects differences between local machine configuration files and official GitHub repository
func (um *UpdateManager) CheckConfigsDiff() (*ConfigDiffReport, error) {
	filesToDiff := []struct {
		name     string
		category string
		purpose  string
	}{
		{"config-network.properties", "Consensus Rules", "Network fees, block generation times, and nemesis hash definitions"},
		{"peers-p2p.json", "P2P Networking", "Active Mainnet P2P validator bootstrap peers and seed nodes"},
		{"peers-api.json", "REST Gateway", "Public REST API node gateway seed directory"},
		{"replicators.json", "Storage Replicators", "DFMS distributed storage replicator bootstrap node network"},
		{"supported-entities.json", "Transaction Types", "Supported blockchain transaction entity types and plugin definitions"},
	}

	repo := um.getRepo()
	officialBase := um.getOfficialBase()

	report := &ConfigDiffReport{
		Repository:   repo,
		OfficialBase: officialBase,
		CheckedAt:    time.Now(),
		TotalFiles:   len(filesToDiff),
		Files:        make([]ConfigFileDiff, 0, len(filesToDiff)),
	}

	type result struct {
		diff ConfigFileDiff
		err  error
	}

	resChan := make(chan result, len(filesToDiff))

	for _, item := range filesToDiff {
		go func(f struct{ name, category, purpose string }) {
			d := ConfigFileDiff{
				FileName: f.name,
				Category: f.category,
				Purpose:  f.purpose,
				Status:   "identical",
			}

			// 1. Check local file
			localPath := filepath.Join(um.resourcesDir, f.name)
			localData, err := os.ReadFile(localPath)
			localExists := err == nil
			if localExists {
				h := sha256.Sum256(localData)
				d.LocalHash = hex.EncodeToString(h[:])
				d.LocalBytes = int64(len(localData))
			} else {
				d.Status = "missing"
			}

			// 2. Fetch remote file
			remoteUrl := fmt.Sprintf("%s/%s", officialBase, f.name)
			req, rErr := http.NewRequest("GET", remoteUrl, nil)
			if rErr != nil {
				d.Status = "error"
				d.Error = rErr.Error()
				resChan <- result{diff: d, err: rErr}
				return
			}
			req.Header.Set("User-Agent", "ProximaX-Sirius-Core-Manager")

			resp, doErr := um.httpClient.Do(req)
			if doErr != nil {
				d.Status = "error"
				d.Error = doErr.Error()
				resChan <- result{diff: d, err: doErr}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				d.Status = "error"
				d.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				resChan <- result{diff: d, err: fmt.Errorf("remote returned %d", resp.StatusCode)}
				return
			}

			remoteData, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				d.Status = "error"
				d.Error = readErr.Error()
				resChan <- result{diff: d, err: readErr}
				return
			}

			// Check not HTML error
			trimmed := strings.TrimSpace(string(remoteData))
			if strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html") || strings.HasPrefix(trimmed, "<?xml") {
				d.Status = "error"
				d.Error = "Remote returned HTML error page"
				resChan <- result{diff: d, err: errors.New("HTML error")}
				return
			}

			rh := sha256.Sum256(remoteData)
			d.RemoteHash = hex.EncodeToString(rh[:])
			d.RemoteBytes = int64(len(remoteData))

			// 3. Compare hashes
			if !localExists {
				d.Status = "missing"
			} else if d.LocalHash == d.RemoteHash {
				d.Status = "identical"
			} else {
				d.Status = "different"
			}

			resChan <- result{diff: d, err: nil}
		}(item)
	}

	for i := 0; i < len(filesToDiff); i++ {
		res := <-resChan
		report.Files = append(report.Files, res.diff)
		switch res.diff.Status {
		case "different":
			report.DifferentCount++
		case "missing":
			report.MissingCount++
		case "identical":
			report.IdenticalCount++
		}
	}

	report.HasDifferences = (report.DifferentCount > 0 || report.MissingCount > 0)
	return report, nil
}

// ApplyOfficialUpdate implements best-practice update sequence: pre-backup, atomic swap, and automated rollback on failure
func (um *UpdateManager) ApplyOfficialUpdate(dataPath string) error {
	um.mu.Lock()
	if um.lastInfo.IsApplying {
		um.mu.Unlock()
		return fmt.Errorf("configuration synchronization is already in progress")
	}
	um.lastInfo.IsApplying = true
	um.lastInfo.RollbackOccurred = false
	um.lastInfo.UpdateMessage = "Starting configuration synchronization pipeline..."
	um.mu.Unlock()

	defer func() {
		um.mu.Lock()
		um.lastInfo.IsApplying = false
		um.mu.Unlock()
	}()

	filesToUpdate := []struct {
		name     string
		category string
		purpose  string
	}{
		{"config-network.properties", "Consensus Rules", "Network fees and consensus parameters"},
		{"peers-p2p.json", "P2P Networking", "Active validator seed nodes"},
		{"peers-api.json", "REST Gateway", "REST API gateway seeds"},
		{"replicators.json", "Storage Replicators", "DFMS replicator bootstrap nodes"},
		{"supported-entities.json", "Transaction Types", "Supported entity plugins"},
	}

	// 1. Pre-flight download and validate ALL files in memory before touching local filesystem
	um.mu.Lock()
	um.lastInfo.UpdateMessage = "Downloading and verifying official configuration assets in memory..."
	um.mu.Unlock()

	downloadedContents := make(map[string][]byte)
	for i, f := range filesToUpdate {
		um.mu.Lock()
		um.lastInfo.UpdateMessage = fmt.Sprintf("Fetching & verifying %s (%d/%d)...", f.name, i+1, len(filesToUpdate))
		um.mu.Unlock()

		fileUrl := fmt.Sprintf("%s/%s", um.getOfficialBase(), f.name)
		resp, err := um.httpClient.Get(fileUrl)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			err := fmt.Errorf("failed downloading %s from server (status: %v, err: %v)", f.name, resp, err)
			um.mu.Lock()
			um.lastInfo.UpdateMessage = err.Error()
			um.mu.Unlock()
			return err
		}

		content, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || len(content) < 10 {
			err := fmt.Errorf("remote content for %s too small or empty", f.name)
			um.mu.Lock()
			um.lastInfo.UpdateMessage = err.Error()
			um.mu.Unlock()
			return err
		}

		// Security: Reject HTML error pages
		trimmed := strings.TrimSpace(string(content))
		if strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html") || strings.HasPrefix(trimmed, "<?xml") {
			err := fmt.Errorf("remote file %s returned HTML error payload instead of valid configuration", f.name)
			um.mu.Lock()
			um.lastInfo.UpdateMessage = err.Error()
			um.mu.Unlock()
			return err
		}

		// Structural validation by file type
		if strings.HasSuffix(f.name, ".json") {
			var js json.RawMessage
			if err := json.Unmarshal(content, &js); err != nil {
				err := fmt.Errorf("structural JSON validation failed for %s: %w", f.name, err)
				um.mu.Lock()
				um.lastInfo.UpdateMessage = err.Error()
				um.mu.Unlock()
				return err
			}
		} else if strings.HasSuffix(f.name, ".properties") {
			if !strings.Contains(trimmed, "=") && !strings.Contains(trimmed, "[") {
				err := fmt.Errorf("properties syntax validation failed for %s", f.name)
				um.mu.Lock()
				um.lastInfo.UpdateMessage = err.Error()
				um.mu.Unlock()
				return err
			}
		}

		downloadedContents[f.name] = content
	}

	// 2. Make complete backup of existing local configs before modifying anything
	backupTimestamp := time.Now().Unix()
	backupDirName := fmt.Sprintf(".backup-configs-%d", backupTimestamp)
	backupDirPath := filepath.Join(um.resourcesDir, backupDirName)
	if err := os.MkdirAll(backupDirPath, 0700); err != nil {
		err := fmt.Errorf("failed to create backup directory: %w", err)
		um.mu.Lock()
		um.lastInfo.UpdateMessage = err.Error()
		um.mu.Unlock()
		return err
	}

	backedUpFiles := make([]string, 0, len(filesToUpdate))
	for _, f := range filesToUpdate {
		src := filepath.Join(um.resourcesDir, f.name)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(backupDirPath, f.name)
			if err := copyFile(src, dst); err == nil {
				_ = copyFile(src, src+".bak")
				backedUpFiles = append(backedUpFiles, f.name)
			}
		}
	}

	um.mu.Lock()
	um.lastInfo.BackupPath = backupDirPath
	um.lastInfo.UpdateMessage = fmt.Sprintf("Created secure backup of %d local config files in %s", len(backedUpFiles), backupDirName)
	um.mu.Unlock()

	// 3. Gracefully stop node if running before swapping
	wasRunning := um.supervisor.IsRunning()
	if wasRunning {
		um.mu.Lock()
		um.lastInfo.UpdateMessage = "Stopping node engine gracefully before applying verified configurations..."
		um.mu.Unlock()
		_ = um.supervisor.StopNode()
		time.Sleep(1500 * time.Millisecond)
	}

	// Remove server.lock
	localDataDir := dataPath
	if localDataDir == "" {
		localDataDir = filepath.Join(um.resourcesDir, "..", "data")
	}
	_ = os.Remove(filepath.Join(localDataDir, "server.lock"))

	// Rollback helper to restore original configs on any failure
	rollbackFn := func(reason error) error {
		log.Printf("[ConfigUpdater] Sync failure: %v. Rolling back to %s...", reason, backupDirPath)
		for _, name := range backedUpFiles {
			src := filepath.Join(backupDirPath, name)
			dst := filepath.Join(um.resourcesDir, name)
			_ = copyFile(src, dst)
		}
		um.mu.Lock()
		um.lastInfo.RollbackOccurred = true
		um.lastInfo.UpdateMessage = fmt.Sprintf("Node healthcheck failed (%v). Automatically rolled back to previous configuration from backup.", reason)
		um.mu.Unlock()

		if wasRunning {
			_ = um.supervisor.StartNode(dataPath)
		}
		return fmt.Errorf("configuration sync failed, rolled back: %w", reason)
	}

	// 4. Atomic replacement of configuration files
	for _, f := range filesToUpdate {
		content := downloadedContents[f.name]
		targetPath := filepath.Join(um.resourcesDir, f.name)
		tmpPath := fmt.Sprintf("%s.tmp.%d", targetPath, time.Now().UnixNano())

		if tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600); err == nil {
			if _, err := tmpFile.Write(content); err == nil {
				_ = tmpFile.Sync()
				_ = tmpFile.Close()
				if err := os.Rename(tmpPath, targetPath); err != nil {
					_ = os.Remove(tmpPath)
					return rollbackFn(fmt.Errorf("failed to atomically swap %s: %w", f.name, err))
				}
				_ = os.Chmod(targetPath, 0600)
			} else {
				_ = tmpFile.Close()
				_ = os.Remove(tmpPath)
				return rollbackFn(fmt.Errorf("failed to write %s: %w", f.name, err))
			}
		} else {
			return rollbackFn(fmt.Errorf("failed creating temporary file for %s: %w", f.name, err))
		}
	}

	// 5. Post-update node restart & operational healthcheck probe
	if wasRunning {
		um.mu.Lock()
		um.lastInfo.UpdateMessage = "Starting node with updated configurations and verifying healthcheck..."
		um.mu.Unlock()

		if err := um.supervisor.StartNode(dataPath); err != nil {
			return rollbackFn(fmt.Errorf("node failed to start: %w", err))
		}

		// Allow 2.5 seconds to verify no crash-loop / immediate exit
		time.Sleep(2500 * time.Millisecond)
		if !um.supervisor.IsRunning() {
			return rollbackFn(errors.New("node process crashed or exited unexpectedly after loading updated configuration"))
		}
	}

	// Success!
	um.mu.Lock()
	um.lastInfo.HasUpdate = false
	um.lastInfo.RollbackOccurred = false
	um.lastInfo.UpdateMessage = fmt.Sprintf("Successfully synchronized 5 official configuration files! (Backup saved: %s)", backupDirName)
	um.mu.Unlock()

	return nil
}
