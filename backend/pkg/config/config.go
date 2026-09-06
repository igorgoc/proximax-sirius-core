package config

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"proximax-sirius-core/pkg/crypto"
)

type NodeConfig struct {
	FriendlyName        string `json:"friendlyName"`
	Host                string `json:"host"`
	BootKey             string `json:"bootKey,omitempty"`
	BootPublicKey       string `json:"bootPublicKey,omitempty"`
	BootKeySource       string `json:"bootKeySource"` // "harvest" | "generated"
	HarvestKey          string `json:"harvestKey,omitempty"`
	HarvestPublicKey    string `json:"harvestPublicKey,omitempty"`
	HarvestAddress      string `json:"harvestAddress,omitempty"`
	HasBootKey          bool   `json:"hasBootKey"`
	HasHarvestKey       bool   `json:"hasHarvestKey"`
	Beneficiary         string `json:"beneficiary"`
	IsAutoHarvesting    bool   `json:"isAutoHarvesting"`
	MaxUnlockedAccounts int    `json:"maxUnlockedAccounts"`
	Port                int    `json:"port"`
	ApiPort             int    `json:"apiPort"`
	DbrbPort            int    `json:"dbrbPort"`
	DataDirectory       string `json:"dataDirectory"` // container internal directory (/data)
	DataPath            string `json:"dataPath"`      // host/local data directory path
	MigrateData         bool   `json:"migrateData,omitempty"`
	IsConfigured        bool   `json:"isConfigured"`
}

type ConfigManager struct {
	basePath string
	mu       sync.RWMutex
}

func NewConfigManager(basePath string) *ConfigManager {
	cm := &ConfigManager{
		basePath: basePath,
	}
	cm.HardenFilePermissions()
	return cm
}

// HardenFilePermissions sets owner-only permissions (0600) on all sensitive properties files
func (cm *ConfigManager) HardenFilePermissions() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	resDir := cm.GetResourcesPath()
	entries, err := os.ReadDir(resDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".properties") || strings.HasPrefix(name, ".sirius-token") || name == "harvest-stats.json" {
			_ = os.Chmod(filepath.Join(resDir, name), 0600)
		}
	}
}

// GetOrCreateApiToken retrieves or securely generates a persistent random API token stored in .sirius-token (0600)
func (cm *ConfigManager) GetOrCreateApiToken() string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 1. Check environment variable override
	if envToken := strings.TrimSpace(os.Getenv("SIRIUS_API_TOKEN")); envToken != "" {
		return envToken
	}

	tokenPath := filepath.Join(cm.GetResourcesPath(), ".sirius-token")
	// 2. Read existing token file if present
	if data, err := os.ReadFile(tokenPath); err == nil {
		tok := strings.TrimSpace(string(data))
		if len(tok) >= 32 {
			_ = os.Chmod(tokenPath, 0600)
			return tok
		}
	}

	// 3. Generate 32 bytes (64 hex characters) of cryptographically secure randomness
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		tok := fmt.Sprintf("%x%x", time.Now().UnixNano(), os.Getpid())
		_ = atomicWriteFile(tokenPath, []byte(tok), 0600)
		return tok
	}
	tok := hex.EncodeToString(b)
	_ = atomicWriteFile(tokenPath, []byte(tok), 0600)
	return tok
}

func (cm *ConfigManager) GetBasePath() string {
	return cm.basePath
}

func (cm *ConfigManager) GetResourcesPath() string {
	return filepath.Join(cm.basePath, "resources")
}

func (cm *ConfigManager) GetDataPath() string {
	cfg, err := cm.LoadNodeConfig()
	if err == nil && cfg.DataPath != "" {
		if filepath.IsAbs(cfg.DataPath) {
			return cfg.DataPath
		}
		return filepath.Join(cm.basePath, "..", cfg.DataPath)
	}
	return filepath.Join(cm.basePath, "data")
}

// LoadNodeConfig reads all primary property files and returns unified struct
func (cm *ConfigManager) LoadNodeConfig() (*NodeConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	defaultDataPath := filepath.Join(cm.basePath, "data")

	cfg := &NodeConfig{
		BootKeySource:       "harvest",
		DataDirectory:       "/data",
		DataPath:            defaultDataPath,
		IsAutoHarvesting:    true,
		MaxUnlockedAccounts: 5,
		Port:                7900,
		ApiPort:             7901,
		DbrbPort:            7903,
	}

	resourcesDir := cm.GetResourcesPath()

	// 1. Read config-manager.properties for manager-specific settings
	managerPropsPath := filepath.Join(resourcesDir, "config-manager.properties")
	ensurePropertiesFile(managerPropsPath)
	if props, err := readPropertiesFile(managerPropsPath); err == nil {
		if val, ok := props["bootkey.source"]; ok && val != "" {
			cfg.BootKeySource = strings.ToLower(val)
		} else if val, ok := props["bootKeySource"]; ok && val != "" {
			cfg.BootKeySource = strings.ToLower(val)
		}

		if val, ok := props["data.path"]; ok && val != "" {
			cfg.DataPath = val
		} else if val, ok := props["dataPath"]; ok && val != "" {
			cfg.DataPath = val
		}
	}

	// 2. Read config-harvesting.properties so we have harvestKey if needed
	harvestPropsPath := filepath.Join(resourcesDir, "config-harvesting.properties")
	ensurePropertiesFile(harvestPropsPath)
	if props, err := readPropertiesFile(harvestPropsPath); err == nil {
		if val, ok := props["harvestKey"]; ok {
			cfg.HarvestKey = val
		}
		if val, ok := props["beneficiary"]; ok {
			cfg.Beneficiary = val
		}
		if val, ok := props["isAutoHarvestingEnabled"]; ok {
			cfg.IsAutoHarvesting = (strings.ToLower(val) == "true")
		}
	}

	// 3. Read config-user.properties (strictly Catapult user properties)
	userPropsPath := filepath.Join(resourcesDir, "config-user.properties")
	ensurePropertiesFile(userPropsPath)
	if props, err := readPropertiesFile(userPropsPath); err == nil {
		if val, ok := props["bootKey"]; ok {
			cfg.BootKey = val
		}

		if val, ok := props["dataDirectory"]; ok && val != "" {
			cfg.DataDirectory = val
		}

		// Backward compatibility & cross-file sync: read bootkey.source & data.path if present
		if val, ok := props["bootkey.source"]; ok && val != "" && cfg.BootKeySource == "harvest" {
			cfg.BootKeySource = strings.ToLower(val)
		}
		if cfg.DataPath == defaultDataPath || cfg.DataPath == "" {
			if val, ok := props["data.path"]; ok && val != "" {
				cfg.DataPath = val
			} else if val, ok := props["dataDirectory"]; ok && val != "" && val != "/data" {
				cfg.DataPath = val
			}
		}
	}

	// In Sirius v1.9.6, bootKey must be unique and not conflict with harvestKey
	if cfg.BootKey == cfg.HarvestKey && cfg.HarvestKey != "" && cfg.HarvestKey != "REMOTE_ACCOUNT_PRIVATE_KEY" {
		if kp, err := crypto.GenerateKeyPair(); err == nil {
			cfg.BootKey = kp.PrivateKey
		}
	}

	// 4. Read config-node.properties
	nodePropsPath := filepath.Join(resourcesDir, "config-node.properties")
	if props, err := readPropertiesFile(nodePropsPath); err == nil {
		if val, ok := props["friendlyName"]; ok {
			cfg.FriendlyName = val
		}
		if val, ok := props["host"]; ok {
			cfg.Host = val
		}
	}

	// Derive Public Keys for display without leaking private keys
	if cfg.BootKey != "" && cfg.BootKey != "BOOTKEY_PRIVATE_KEY" && len(cfg.BootKey) == 64 {
		if kp, err := crypto.KeyPairFromPrivateKey(cfg.BootKey); err == nil {
			cfg.BootPublicKey = kp.PublicKey
		}
	}

	if cfg.HarvestKey != "" && cfg.HarvestKey != "REMOTE_ACCOUNT_PRIVATE_KEY" && len(cfg.HarvestKey) == 64 {
		if kp, err := crypto.KeyPairFromPrivateKey(cfg.HarvestKey); err == nil {
			cfg.HarvestPublicKey = kp.PublicKey
			cfg.HarvestAddress = kp.Address
		}
	}

	// Check if already configured
	isBootKeyValid := cfg.BootKey != "" && cfg.BootKey != "BOOTKEY_PRIVATE_KEY" && len(cfg.BootKey) == 64
	cfg.HasBootKey = isBootKeyValid
	cfg.HasHarvestKey = cfg.HarvestKey != "" && cfg.HarvestKey != "REMOTE_ACCOUNT_PRIVATE_KEY" && len(cfg.HarvestKey) == 64
	cfg.IsConfigured = isBootKeyValid && cfg.FriendlyName != ""

	return cfg, nil
}

// SaveNodeConfig updates properties files safely, handling bootkey source and data migration
func (cm *ConfigManager) SaveNodeConfig(cfg *NodeConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	resourcesDir := cm.GetResourcesPath()
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %w", err)
	}

	// Ensure bootKey does not conflict with harvestKey
	if (cfg.BootKey == "" || cfg.BootKey == cfg.HarvestKey) && cfg.HarvestKey != "" {
		if kp, err := crypto.GenerateKeyPair(); err == nil {
			cfg.BootKey = kp.PrivateKey
		}
	}

	// Check if data migration is requested
	if cfg.MigrateData && cfg.DataPath != "" {
		currentDataPath := filepath.Join(cm.basePath, "data")
		userPropsPath := filepath.Join(resourcesDir, "config-user.properties")
		if props, err := readPropertiesFile(userPropsPath); err == nil {
			if oldPath, ok := props["data.path"]; ok && oldPath != "" {
				currentDataPath = oldPath
			} else if oldPath, ok := props["dataPath"]; ok && oldPath != "" {
				currentDataPath = oldPath
			}
		}

		if currentDataPath != cfg.DataPath {
			if err := cm.migrateDataDirectory(currentDataPath, cfg.DataPath); err != nil {
				return fmt.Errorf("failed to migrate data directory: %w", err)
			}
		}
	}

	// 1. Update config-manager.properties (manager-specific settings)
	managerPropsPath := filepath.Join(resourcesDir, "config-manager.properties")
	managerUpdates := map[string]string{
		"bootkey.source": cfg.BootKeySource,
		"data.path":      cfg.DataPath,
	}
	managerTemplate := "[manager]\n\nbootkey.source = %s\ndata.path = %s\n"
	if err := updatePropertiesFile(managerPropsPath, managerUpdates, managerTemplate, cfg.BootKeySource, cfg.DataPath); err != nil {
		return fmt.Errorf("failed to save config-manager.properties: %w", err)
	}

	// 2. Update config-user.properties (strictly Catapult daemon properties: bootKey, dataDirectory, pluginsDirectory, certificateDirectory)
	userPropsPath := filepath.Join(resourcesDir, "config-user.properties")
	userUpdates := map[string]string{}
	if cfg.BootKey != "" {
		userUpdates["bootKey"] = cfg.BootKey
	}
	if cfg.DataDirectory != "" {
		userUpdates["dataDirectory"] = cfg.DataDirectory
	}
	if cfg.DataPath != "" {
		userUpdates["dataDirectory"] = cfg.DataPath
	}

	userTemplate := "[account]\n\nbootKey = %s\n\n[storage]\n\ndataDirectory = %s\npluginsDirectory = \ncertificateDirectory = /certificate\n"
	if err := updatePropertiesFile(userPropsPath, userUpdates, userTemplate, cfg.BootKey, cfg.DataPath); err != nil {
		return fmt.Errorf("failed to save config-user.properties: %w", err)
	}
	// Catapult strictly enforces property bag limits (VerifyBagSizeLte); strip any accidental foreign keys from config-user.properties
	sanitizeCatapultUserProperties(userPropsPath)

	// 3. Update config-node.properties
	nodePropsPath := filepath.Join(resourcesDir, "config-node.properties")
	nodeUpdates := map[string]string{}
	if cfg.FriendlyName != "" {
		nodeUpdates["friendlyName"] = cfg.FriendlyName
	}
	if cfg.Host != "" {
		nodeUpdates["host"] = cfg.Host
	}
	if err := updatePropertiesFile(nodePropsPath, nodeUpdates, ""); err != nil {
		return fmt.Errorf("failed to save config-node.properties: %w", err)
	}

	// 4. Update config-harvesting.properties
	harvestPropsPath := filepath.Join(resourcesDir, "config-harvesting.properties")
	harvestUpdates := map[string]string{}
	if cfg.HarvestKey != "" {
		harvestUpdates["harvestKey"] = cfg.HarvestKey
	}
	if cfg.Beneficiary != "" {
		harvestUpdates["beneficiary"] = cfg.Beneficiary
	}
	autoHarvest := cfg.IsAutoHarvesting
	if cfg.HarvestKey != "" && cfg.HarvestKey != "REMOTE_ACCOUNT_PRIVATE_KEY" {
		autoHarvest = true
	}
	harvestUpdates["isAutoHarvestingEnabled"] = fmt.Sprintf("%t", autoHarvest)
	if cfg.MaxUnlockedAccounts > 0 {
		harvestUpdates["maxUnlockedAccounts"] = fmt.Sprintf("%d", cfg.MaxUnlockedAccounts)
	}
	if err := updatePropertiesFile(harvestPropsPath, harvestUpdates, "[harvesting]\n\nharvestKey = %s\nbeneficiary = %s\nisAutoHarvestingEnabled = true\nmaxUnlockedAccounts = 5\n", cfg.HarvestKey, cfg.Beneficiary); err != nil {
		return fmt.Errorf("failed to save config-harvesting.properties: %w", err)
	}

	// 5. Ensure official Sirius mainnet consensus extensions are set (fastfinality handles POS+ harvesting; extension.harvesting must be false)
	extServerPropsPath := filepath.Join(resourcesDir, "config-extensions-server.properties")
	_ = updatePropertiesFile(extServerPropsPath, map[string]string{
		"extension.harvesting":   "false",
		"extension.fastfinality": "true",
	}, "")

	// 6. Read-After-Write Verification: confirm on-disk persistence before returning success
	verifiedProps, err := readPropertiesFile(managerPropsPath)
	if err != nil {
		return fmt.Errorf("read-after-write verification failed: cannot re-read %s: %w", managerPropsPath, err)
	}
	if cfg.DataPath != "" && verifiedProps["data.path"] != cfg.DataPath {
		return fmt.Errorf("read-after-write verification mismatch: on-disk data.path is %q, expected %q", verifiedProps["data.path"], cfg.DataPath)
	}

	return nil
}

// MigrateDataDirectory moves blockchain data from source to target path
func (cm *ConfigManager) MigrateDataDirectory(srcPath, dstPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.migrateDataDirectory(srcPath, dstPath)
}

func (cm *ConfigManager) migrateDataDirectory(srcPath, dstPath string) error {
	if srcPath == "" || dstPath == "" || srcPath == dstPath {
		return nil
	}

	// Resolve absolute paths
	srcAbs := srcPath
	if !filepath.IsAbs(srcAbs) {
		srcAbs = filepath.Join(cm.basePath, "..", srcPath)
	}
	dstAbs := dstPath
	if !filepath.IsAbs(dstAbs) {
		dstAbs = filepath.Join(cm.basePath, "..", dstPath)
	}

	if srcAbs == dstAbs {
		return nil
	}

	// Security filter: Prevent migrating sensitive system directories
	forbiddenPaths := []string{"/", "/etc", "/bin", "/sbin", "/usr", "/var", "/proc", "/sys", "/dev", "/root", "/app", "/private", "/System", "/Library"}
	cleanSrc := filepath.Clean(srcAbs)
	cleanDst := filepath.Clean(dstAbs)
	for _, fp := range forbiddenPaths {
		if cleanSrc == fp || cleanDst == fp {
			return fmt.Errorf("migration of protected system path %s is forbidden", fp)
		}
	}

	srcInfo, err := os.Stat(srcAbs)
	if os.IsNotExist(err) {
		// Source does not exist, just create destination
		return os.MkdirAll(dstAbs, 0755)
	}
	if err != nil {
		return fmt.Errorf("cannot inspect source data path %s: %w", srcAbs, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source data path %s is not a directory", srcAbs)
	}

	if err := os.MkdirAll(dstAbs, 0755); err != nil {
		return fmt.Errorf("cannot create destination data path %s: %w", dstAbs, err)
	}

	// Copy all entries from src to dst
	entries, err := os.ReadDir(srcAbs)
	if err != nil {
		return fmt.Errorf("cannot read source data directory: %w", err)
	}

	for _, entry := range entries {
		s := filepath.Join(srcAbs, entry.Name())
		d := filepath.Join(dstAbs, entry.Name())

		// Try renaming (atomic move)
		if err := os.Rename(s, d); err != nil {
			// Cross-device fallback: recursive copy & remove
			if err := copyDirOrFile(s, d); err != nil {
				return fmt.Errorf("failed to move %s to %s: %w", s, d, err)
			}
			_ = os.RemoveAll(s)
		}
	}

	return nil
}

func copyDirOrFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyDirOrFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// GetRawConfigFile reads any file in resources directory with all private keys redacted
func (cm *ConfigManager) GetRawConfigFile(filename string) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cleanFilename := filepath.Base(filename)
	// Security filter: Prevent reading hidden/token files
	if strings.HasPrefix(cleanFilename, ".") || strings.Contains(strings.ToLower(cleanFilename), "token") {
		return "", fmt.Errorf("access denied to protected file")
	}

	filePath := filepath.Join(cm.GetResourcesPath(), cleanFilename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Redact private keys (bootKey, harvestKey, key) completely
	keyPattern := regexp.MustCompile(`(?mi)(^\s*(?:harvestKey|bootKey|key)\s*=\s*)([0-9a-fA-F]{32,64})`)
	redacted := keyPattern.ReplaceAllString(string(data), "${1}[REDACTED_PRIVATE_KEY]")

	return redacted, nil
}

// SaveRawConfigFile writes any file in resources directory atomically with fsync and secure 0600 permissions
func (cm *ConfigManager) SaveRawConfigFile(filename string, content string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cleanFilename := filepath.Base(filename)
	if strings.HasPrefix(cleanFilename, ".") || strings.Contains(strings.ToLower(cleanFilename), "token") {
		return fmt.Errorf("access denied to protected file")
	}

	filePath := filepath.Join(cm.GetResourcesPath(), cleanFilename)

	// If saving a file containing redacted placeholders, preserve the existing private keys on disk
	if strings.Contains(content, "[REDACTED_PRIVATE_KEY]") {
		if existingData, err := os.ReadFile(filePath); err == nil {
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

	return atomicWriteFile(filePath, []byte(content), 0600)
}

// ListConfigFiles returns a list of all configuration files (excluding tokens and hidden files)
func (cm *ConfigManager) ListConfigFiles() ([]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entries, err := os.ReadDir(cm.GetResourcesPath())
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && !strings.Contains(strings.ToLower(entry.Name()), "token") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// Helper functions for property manipulation
func ensurePropertiesFile(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		templatePath := filePath + ".template"
		if data, errT := os.ReadFile(templatePath); errT == nil {
			_ = atomicWriteFile(filePath, data, 0600)
		}
	}
}

func readPropertiesFile(filePath string) (map[string]string, error) {
	props := make(map[string]string)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return props, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			props[k] = v
		}
	}
	return props, nil
}

func updatePropertiesFile(filePath string, updates map[string]string, defaultTemplate string, templateArgs ...interface{}) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) && defaultTemplate != "" {
		content := fmt.Sprintf(defaultTemplate, templateArgs...)
		return atomicWriteFile(filePath, []byte(content), 0600)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	matchedKeys := make(map[string]bool)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			if newVal, ok := updates[k]; ok {
				lines[i] = fmt.Sprintf("%s = %s", k, newVal)
				matchedKeys[k] = true
			}
		}
	}

	for k, v := range updates {
		if !matchedKeys[k] && v != "" {
			lines = append(lines, fmt.Sprintf("%s = %s", k, v))
		}
	}

	return atomicWriteFile(filePath, []byte(strings.Join(lines, "\n")), 0600)
}

// sanitizeCatapultUserProperties ensures config-user.properties strictly contains ONLY
// the exact properties expected by Catapult (bootKey, dataDirectory, pluginsDirectory, certificateDirectory).
// Catapult verifies property count via VerifyBagSizeLte and will abort if unexpected properties exist.
func sanitizeCatapultUserProperties(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	allowedKeys := map[string]bool{
		"bootKey":              true,
		"dataDirectory":        true,
		"pluginsDirectory":     true,
		"certificateDirectory": true,
	}

	lines := strings.Split(string(data), "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "[") {
			cleanedLines = append(cleanedLines, line)
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			if allowedKeys[k] {
				cleanedLines = append(cleanedLines, line)
			}
		}
	}
	_ = atomicWriteFile(filePath, []byte(strings.Join(cleanedLines, "\n")), 0600)
}

// atomicWriteFile safely writes data to a temporary file, calls fsync, closes, and atomically renames over target
func atomicWriteFile(filePath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Security Invariant: Enforce 0600 permissions on all config and properties files
	if perm == 0644 || perm == 0 {
		perm = 0600
	}

	tmpFile, err := os.CreateTemp(dir, fmt.Sprintf(".tmp-%s-*", filepath.Base(filePath)))
	if err != nil {
		return fmt.Errorf("failed to create temporary config file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write config bytes to %s: %w", tmpPath, err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", tmpPath, err)
	}
	// Force storage controller commit to physical disk/SSD
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to fsync %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", tmpPath, err)
	}

	// Atomic rename over target file
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to atomically rename %s to %s: %w", tmpPath, filePath, err)
	}

	// Persist directory entry to physical media on POSIX
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}
