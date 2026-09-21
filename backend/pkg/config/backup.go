package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type NodeBackup struct {
	Version      string            `json:"version"`
	ExportedAt   time.Time         `json:"exportedAt"`
	FriendlyName string            `json:"friendlyName"`
	ConfigFiles  map[string]string `json:"configFiles"`
	StatsJson    string            `json:"statsJson,omitempty"`
}

// ExportBackup bundles all property files and stats into a single exportable struct
func (cm *ConfigManager) ExportBackup() (*NodeBackup, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	resourcesDir := cm.GetResourcesPath()
	backup := &NodeBackup{
		Version:     "1.0",
		ExportedAt:  time.Now().UTC(),
		ConfigFiles: make(map[string]string),
	}

	cfg, _ := cm.LoadNodeConfig()
	if cfg != nil {
		backup.FriendlyName = cfg.FriendlyName
	}

	filesToBackup := []string{
		"config-node.properties",
		"config-user.properties",
		"config-harvesting.properties",
		"config-manager.properties",
		"config-network.properties",
		"peers-api.json",
		"peers-p2p.json",
		"replicators.json",
		"supported-entities.json",
	}

	keyPattern := regexp.MustCompile(`(?mi)(^\s*(?:harvestKey|bootKey|key)\s*=\s*)([0-9a-fA-F]{32,64})`)
	for _, fn := range filesToBackup {
		filePath := filepath.Join(resourcesDir, fn)
		if data, err := os.ReadFile(filePath); err == nil {
			content := string(data)
			if strings.HasSuffix(fn, ".properties") {
				content = keyPattern.ReplaceAllString(content, "${1}[REDACTED_PRIVATE_KEY]")
			}
			backup.ConfigFiles[fn] = content
		}
	}

	// Also backup harvest-stats.json if present
	statsPath := filepath.Join(resourcesDir, "harvest-stats.json")
	if data, err := os.ReadFile(statsPath); err == nil {
		backup.StatsJson = string(data)
	}

	return backup, nil
}

// ImportBackup restores property files and stats from a backup struct
func (cm *ConfigManager) ImportBackup(backupData []byte) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var backup NodeBackup
	if err := json.Unmarshal(backupData, &backup); err != nil {
		return fmt.Errorf("invalid backup file format: %w", err)
	}

	if len(backup.ConfigFiles) == 0 {
		return fmt.Errorf("backup file contains no configuration data")
	}

	resourcesDir := cm.GetResourcesPath()
	_ = os.MkdirAll(resourcesDir, 0755)

	// Strict security whitelist of recognized configuration files
	allowedConfigFiles := map[string]bool{
		"config-node.properties":           true,
		"config-user.properties":           true,
		"config-harvesting.properties":     true,
		"config-manager.properties":        true,
		"config-network.properties":        true,
		"config-storage.properties":        true,
		"config-database.properties":       true,
		"config-dbrb.properties":           true,
		"config-extensions-broker.properties": true,
		"config-extensions-recovery.properties": true,
		"config-extensions-server.properties": true,
		"config-immutable.properties":      true,
		"config-inflation.properties":      true,
		"config-logging-broker.properties": true,
		"config-logging-recovery.properties": true,
		"config-logging-server.properties": true,
		"config-messaging.properties":      true,
		"config-networkheight.properties":  true,
		"config-pt.properties":             true,
		"config-task.properties":           true,
		"config-timesync.properties":       true,
		"peers-api.json":                   true,
		"peers-p2p.json":                   true,
		"replicators.json":                 true,
		"supported-entities.json":          true,
	}

	for fn, content := range backup.ConfigFiles {
		cleanFn := filepath.Base(fn)
		// Security: Explicitly reject token files and non-whitelisted files
		if !allowedConfigFiles[cleanFn] || strings.HasPrefix(cleanFn, ".") || strings.Contains(strings.ToLower(cleanFn), "token") {
			continue
		}
		targetPath := filepath.Join(resourcesDir, cleanFn)

		// Preserve existing private keys if redacted placeholder is present
		if strings.Contains(content, "[REDACTED_PRIVATE_KEY]") {
			if existingData, err := os.ReadFile(targetPath); err == nil {
				existingProps := make(map[string]string)
				scanner := bufio.NewScanner(bytes.NewReader(existingData))
				for scanner.Scan() {
					parts := strings.SplitN(strings.TrimSpace(scanner.Text()), "=", 2)
					if len(parts) == 2 {
						existingProps[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
				if k, ok := existingProps["harvestKey"]; ok && k != "" {
					content = strings.ReplaceAll(content, "harvestKey = [REDACTED_PRIVATE_KEY]", "harvestKey = "+k)
					content = strings.ReplaceAll(content, "harvestKey=[REDACTED_PRIVATE_KEY]", "harvestKey="+k)
				}
				if k, ok := existingProps["bootKey"]; ok && k != "" {
					content = strings.ReplaceAll(content, "bootKey = [REDACTED_PRIVATE_KEY]", "bootKey = "+k)
					content = strings.ReplaceAll(content, "bootKey=[REDACTED_PRIVATE_KEY]", "bootKey="+k)
				}
				if k, ok := existingProps["key"]; ok && k != "" {
					content = strings.ReplaceAll(content, "key = [REDACTED_PRIVATE_KEY]", "key = "+k)
					content = strings.ReplaceAll(content, "key=[REDACTED_PRIVATE_KEY]", "key="+k)
				}
			}
		}

		_ = atomicWriteFile(targetPath, []byte(content), 0600)
	}

	if backup.StatsJson != "" {
		statsPath := filepath.Join(resourcesDir, "harvest-stats.json")
		_ = atomicWriteFile(statsPath, []byte(backup.StatsJson), 0600)
	}

	return nil
}
