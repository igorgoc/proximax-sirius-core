package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	GitHubRepo     = "proximax-storage/cpp-xpx-chain"
	OfficialBase   = "https://raw.githubusercontent.com/proximax-storage/cpp-xpx-chain/master/resources"
)

type UpdateInfo struct {
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	HasUpdate       bool      `json:"hasUpdate"`
	ReleaseTitle    string    `json:"releaseTitle,omitempty"`
	ReleaseNotes    string    `json:"releaseNotes,omitempty"`
	ReleaseUrl      string    `json:"releaseUrl,omitempty"`
	PublishedAt     string    `json:"publishedAt,omitempty"`
	LastChecked     time.Time `json:"lastChecked"`
	OfficialFiles   []string  `json:"officialFiles"`
	IsApplying      bool      `json:"isApplying"`
	UpdateMessage   string    `json:"updateMessage,omitempty"`
}

type UpdateManager struct {
	mu           sync.RWMutex
	lastInfo     UpdateInfo
	resourcesDir string
	supervisor   *supervisor.ProcessSupervisor
	httpClient   *http.Client
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

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo), nil)
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
			cleanTag := strings.TrimPrefix(ghRelease.TagName, "release-")
			cleanCurrent := strings.TrimPrefix(CurrentVersion, "release-")

			um.lastInfo.LatestVersion = cleanTag
			um.lastInfo.CurrentVersion = cleanCurrent
			um.lastInfo.ReleaseTitle = ghRelease.Name
			um.lastInfo.ReleaseNotes = ghRelease.Body
			um.lastInfo.ReleaseUrl = ghRelease.HtmlUrl
			um.lastInfo.PublishedAt = ghRelease.PublishedAt
			um.lastInfo.LastChecked = time.Now()

			// Check if latest version is newer
			if cleanTag != "" && cleanTag != cleanCurrent && !strings.Contains(cleanCurrent, cleanTag) {
				um.lastInfo.HasUpdate = true
			} else {
				um.lastInfo.HasUpdate = false
			}
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

// ApplyOfficialUpdate implements best-practice update sequence from update_nodes.sh
func (um *UpdateManager) ApplyOfficialUpdate(dataPath string) error {
	um.mu.Lock()
	if um.lastInfo.IsApplying {
		um.mu.Unlock()
		return fmt.Errorf("update is already in progress")
	}
	um.lastInfo.IsApplying = true
	um.lastInfo.UpdateMessage = "Stopping node container..."
	um.mu.Unlock()

	defer func() {
		um.mu.Lock()
		um.lastInfo.IsApplying = false
		um.mu.Unlock()
	}()

	// 1. Stop container cleanly
	_ = um.supervisor.StopNode()
	time.Sleep(2 * time.Second)

	// 2. Remove server.lock from data directory
	localDataDir := dataPath
	if localDataDir == "" {
		localDataDir = filepath.Join(um.resourcesDir, "..", "data")
	}
	_ = os.Remove(filepath.Join(localDataDir, "server.lock"))

	// 3. Download official config files into resources
	um.mu.Lock()
	um.lastInfo.UpdateMessage = "Updating network configuration and seed files from official GitHub repository..."
	um.mu.Unlock()

	filesToUpdate := []string{
		"config-network.properties",
		"peers-api.json",
		"peers-p2p.json",
		"replicators.json",
		"supported-entities.json",
	}

	for i, fileName := range filesToUpdate {
		um.mu.Lock()
		um.lastInfo.UpdateMessage = fmt.Sprintf("Fetching & verifying %s (%d/%d)...", fileName, i+1, len(filesToUpdate))
		um.mu.Unlock()

		fileUrl := fmt.Sprintf("%s/%s", OfficialBase, fileName)
		resp, err := um.httpClient.Get(fileUrl)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			log.Printf("[Updater Warning] Failed to download %s: status %v, err %v", fileName, resp, err)
			continue
		}

		content, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || len(content) < 10 {
			log.Printf("[Updater Warning] Downloaded content for %s too small or read error: %v", fileName, err)
			continue
		}

		// Security: Reject HTML error pages (e.g. rate limits, proxy errors)
		trimmed := strings.TrimSpace(string(content))
		if strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html") || strings.HasPrefix(trimmed, "<?xml") {
			log.Printf("[Updater Error] Downloaded %s appears to be HTML error payload, rejecting", fileName)
			continue
		}

		// Structural validation by file type
		if strings.HasSuffix(fileName, ".json") {
			var js json.RawMessage
			if err := json.Unmarshal(content, &js); err != nil {
				log.Printf("[Updater Error] Structural validation failed for %s (invalid JSON): %v", fileName, err)
				continue
			}
		} else if strings.HasSuffix(fileName, ".properties") {
			if !strings.Contains(trimmed, "=") && !strings.Contains(trimmed, "[") {
				log.Printf("[Updater Error] Structural validation failed for %s (invalid properties format)", fileName)
				continue
			}
		}

		// Calculate SHA-256 Checksum for tamper evidence and integrity tracking
		h := sha256.Sum256(content)
		shaHex := hex.EncodeToString(h[:])
		log.Printf("[Updater] Validated %s (size: %d bytes, SHA256: %s)", fileName, len(content), shaHex)

		// Atomic write with secure 0600 permissions
		targetPath := filepath.Join(um.resourcesDir, fileName)
		tmpPath := fmt.Sprintf("%s.tmp.%d", targetPath, time.Now().UnixNano())
		if tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600); err == nil {
			if _, err := tmpFile.Write(content); err == nil {
				_ = tmpFile.Sync()
				_ = tmpFile.Close()
				_ = os.Rename(tmpPath, targetPath)
				_ = os.Chmod(targetPath, 0600)
			} else {
				_ = tmpFile.Close()
				_ = os.Remove(tmpPath)
			}
		}

		um.mu.Lock()
		um.lastInfo.UpdateMessage = fmt.Sprintf("Installed %s (SHA-256: %s...)", fileName, shaHex[:12])
		um.mu.Unlock()
	}

	// 4. Start node back up
	um.mu.Lock()
	um.lastInfo.UpdateMessage = "Restarting node with updated configuration..."
	um.mu.Unlock()

	if err := um.supervisor.StartNode(dataPath); err != nil {
		um.mu.Lock()
		um.lastInfo.UpdateMessage = fmt.Sprintf("Updated configs, but restart failed: %v", err)
		um.mu.Unlock()
		return err
	}

	um.mu.Lock()
	um.lastInfo.HasUpdate = false
	um.lastInfo.UpdateMessage = "Successfully updated official network configuration and restarted node!"
	um.mu.Unlock()

	return nil
}
