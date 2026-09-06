package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	"proximax-sirius-core/pkg/crypto"

	"github.com/proximax-storage/go-xpx-chain-sdk/sdk"
)

type StorageConfig struct {
	Key                string `json:"key,omitempty"`
	PublicKey          string `json:"publicKey"`
	Address            string `json:"address"`
	HasKey             bool   `json:"hasKey"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	StorageDirectory   string `json:"storageDirectory"`
	SandboxDirectory   string `json:"sandboxDirectory"`
	StoragePath        string `json:"storagePath"`
	DefaultStoragePath string `json:"defaultStoragePath"`
	ResolvedPath       string `json:"resolvedPath"`
	UseTcpSocket       bool   `json:"useTcpSocket"`
	UseRpcReplicator   bool   `json:"useRpcReplicator"`
	IsConfigured       bool   `json:"isConfigured"`
}

type StorageMetrics struct {
	DriveSizeBytes   int64  `json:"driveSizeBytes"`
	DriveSizeStr     string `json:"driveSizeStr"`
	SandboxSizeBytes int64  `json:"sandboxSizeBytes"`
	SandboxSizeStr   string `json:"sandboxSizeStr"`
	TotalShardsCount int    `json:"totalShardsCount"`
	IsStorageActive  bool   `json:"isStorageActive"`
}

type ReplicatorAccountInfo struct {
	Address        string  `json:"address"`
	PublicKey      string  `json:"publicKey"`
	BalanceXPX     float64 `json:"balanceXPX"`
	BalanceSO      float64 `json:"balanceSO"`
	BalanceSI      float64 `json:"balanceSI"`
	IsRegistered   bool    `json:"isRegistered"`
	AccountType    string  `json:"accountType"`
	StorageDeposit float64 `json:"storageDeposit,omitempty"`
}

type ReplicatorPeer struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	PublicKey   string `json:"publicKey"`
	LatencyMs   int64  `json:"latencyMs"`
	IsReachable bool   `json:"isReachable"`
}

type StorageStatus struct {
	Config   StorageConfig          `json:"config"`
	Metrics  StorageMetrics         `json:"metrics"`
	OnChain  ReplicatorAccountInfo  `json:"onChain"`
	Peers    []ReplicatorPeer       `json:"peers"`
	LastScan string                 `json:"lastScan"`
}

type OnboardResult struct {
	TxHash     string `json:"txHash"`
	Signer     string `json:"signer"`
	CapacityGB uint64 `json:"capacityGB"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

type StorageManager struct {
	resourcesPath  string
	dataPathGetter func() string
	bootKeyGetter  func() string
	mu             sync.RWMutex
	cachedPeers     []ReplicatorPeer
	lastPeerScan    time.Time
	cachedAccount   ReplicatorAccountInfo
	lastAccountScan time.Time
	httpClient      *http.Client
}

func NewStorageManager(resourcesPath string, dataPathGetter func() string, bootKeyGetter func() string) *StorageManager {
	sm := &StorageManager{
		resourcesPath:  resourcesPath,
		dataPathGetter: dataPathGetter,
		bootKeyGetter:  bootKeyGetter,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
	}
	go sm.startBackgroundPeerChecker()
	return sm
}

// LoadStorageConfig reads config-storage.properties and derives public identities
func (sm *StorageManager) LoadStorageConfig() (*StorageConfig, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	propsPath := filepath.Join(sm.resourcesPath, "config-storage.properties")
	ensurePropertiesFile(propsPath)
	props, err := readPropertiesFile(propsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config-storage.properties: %w", err)
	}

	baseData := sm.dataPathGetter()
	if baseData == "" {
		baseData = filepath.Join(sm.resourcesPath, "..", "data")
	} else if !filepath.IsAbs(baseData) {
		baseData = filepath.Join(sm.resourcesPath, "..", baseData)
	}
	defaultPath := filepath.Join(baseData, "drives")

	cfg := &StorageConfig{
		Host:               "0.0.0.0",
		Port:               7904,
		StorageDirectory:   "/data/drives",
		SandboxDirectory:   "/data/drives/drive-sandboxes",
		DefaultStoragePath: defaultPath,
		ResolvedPath:       defaultPath,
		UseTcpSocket:       true,
		UseRpcReplicator:   true,
	}

	if val, ok := props["key"]; ok {
		cfg.Key = val
	}
	if val, ok := props["host"]; ok && val != "" {
		cfg.Host = val
	}
	if val, ok := props["storageDirectory"]; ok && val != "" {
		cfg.StorageDirectory = val
	}
	if val, ok := props["sandboxDirectory"]; ok && val != "" {
		cfg.SandboxDirectory = val
	}
	if val, ok := props["storage.path"]; ok && strings.TrimSpace(val) != "" {
		cfg.StoragePath = strings.TrimSpace(val)
		cfg.ResolvedPath = cfg.StoragePath
	} else if val, ok := props["storagePath"]; ok && strings.TrimSpace(val) != "" {
		cfg.StoragePath = strings.TrimSpace(val)
		cfg.ResolvedPath = cfg.StoragePath
	}
	if val, ok := props["useTcpSocket"]; ok {
		cfg.UseTcpSocket = strings.ToLower(val) == "true"
	}
	if val, ok := props["useRpcReplicator"]; ok {
		cfg.UseRpcReplicator = strings.ToLower(val) == "true"
	}

	cleanKey := strings.TrimSpace(cfg.Key)
	if cleanKey != "" && cleanKey != "REPLICATOR_PRIVATE_KEY" && len(cleanKey) == 64 {
		cfg.HasKey = true
		if kp, err := crypto.KeyPairFromPrivateKey(cleanKey); err == nil {
			cfg.PublicKey = kp.PublicKey
			cfg.Address = kp.Address
			cfg.IsConfigured = true
		}
	}

	return cfg, nil
}

// GetActiveStoragePath returns the physical host directory path where drives are stored
func (sm *StorageManager) GetActiveStoragePath() string {
	cfg, err := sm.LoadStorageConfig()
	if err == nil && cfg.StoragePath != "" {
		return cfg.StoragePath
	}
	baseData := sm.dataPathGetter()
	if baseData == "" {
		baseData = filepath.Join(sm.resourcesPath, "..", "data")
	} else if !filepath.IsAbs(baseData) {
		baseData = filepath.Join(sm.resourcesPath, "..", baseData)
	}
	return filepath.Join(baseData, "drives")
}

// SaveStorageConfig updates config-storage.properties with a new replicator private key or configuration
func (sm *StorageManager) SaveStorageConfig(newKey string, host string, storagePath string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	propsPath := filepath.Join(sm.resourcesPath, "config-storage.properties")
	props, err := readPropertiesFile(propsPath)
	if err != nil {
		props = make(map[string]string)
	}

	cleanKey := strings.TrimSpace(newKey)
	if cleanKey != "" {
		props["key"] = cleanKey
	}
	if host != "" {
		props["host"] = host
	} else {
		props["host"] = "0.0.0.0"
	}
	if storagePath != "" {
		props["storage.path"] = strings.TrimSpace(storagePath)
	} else {
		delete(props, "storage.path")
		delete(props, "storagePath")
	}
	props["port"] = "7904"
	props["transactionTimeout"] = "1h"
	props["storageDirectory"] = "/data/drives"
	props["sandboxDirectory"] = "/data/drives/drive-sandboxes"
	props["useTcpSocket"] = "true"
	props["useRpcReplicator"] = "true"
	props["rpcHost"] = "127.0.0.1"
	props["rpcPort"] = "7905"
	props["rpcHandleLostConnection"] = "false"
	props["rpcDbgChildCrash"] = "true"

	return writePropertiesFile(propsPath, "[replicator]", props)
}

// GetStorageMetrics calculates disk utilization of the replicated drives and sandboxes
func (sm *StorageManager) GetStorageMetrics() StorageMetrics {
	drivesDir := sm.GetActiveStoragePath()
	sandboxDir := filepath.Join(drivesDir, "drive-sandboxes")

	var driveBytes int64
	var sandboxBytes int64
	var shardCount int

	if fi, err := os.Stat(drivesDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(drivesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if !info.IsDir() {
				if strings.HasPrefix(path, sandboxDir) {
					sandboxBytes += info.Size()
				} else {
					driveBytes += info.Size()
					shardCount++
				}
			}
			return nil
		})
	}

	return StorageMetrics{
		DriveSizeBytes:   driveBytes,
		DriveSizeStr:     formatBytes(driveBytes),
		SandboxSizeBytes: sandboxBytes,
		SandboxSizeStr:   formatBytes(sandboxBytes),
		TotalShardsCount: shardCount,
		IsStorageActive:  true,
	}
}

// CleanSandboxes removes temporary uncommitted replication sandboxes
func (sm *StorageManager) CleanSandboxes() (int64, error) {
	drivesDir := sm.GetActiveStoragePath()
	sandboxDir := filepath.Join(drivesDir, "drive-sandboxes")
	var freedBytes int64

	if entries, err := os.ReadDir(sandboxDir); err == nil {
		for _, e := range entries {
			p := filepath.Join(sandboxDir, e.Name())
			if fi, sErr := os.Stat(p); sErr == nil {
				freedBytes += fi.Size()
			}
			_ = os.RemoveAll(p)
		}
	}

	_ = os.MkdirAll(sandboxDir, 0755)
	return freedBytes, nil
}

// CheckOnChainAccount queries the Sirius REST public API to verify replicator balance and registration
func (sm *StorageManager) CheckOnChainAccount(pubKey string) ReplicatorAccountInfo {
	sm.mu.RLock()
	if sm.cachedAccount.PublicKey == pubKey && time.Since(sm.lastAccountScan) < 30*time.Second {
		acc := sm.cachedAccount
		sm.mu.RUnlock()
		return acc
	}
	sm.mu.RUnlock()

	res := ReplicatorAccountInfo{
		PublicKey: pubKey,
	}

	if pubKey == "" || pubKey == "REPLICATOR_PUBLIC_KEY" {
		return res
	}

	for _, apiNode := range crypto.DefaultApiNodes {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(apiNode, "/"), pubKey)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			cancel()
			continue
		}

		resp, err := sm.httpClient.Do(req)
		if err != nil {
			cancel()
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var accData struct {
				Account struct {
					Address string `json:"address"`
					Mosaics []struct {
						Id     [2]uint64 `json:"id"`
						Amount [2]uint64 `json:"amount"`
					} `json:"mosaics"`
				} `json:"account"`
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()

			if err := json.Unmarshal(body, &accData); err == nil {
				res.Address = accData.Account.Address
				res.IsRegistered = true
				res.AccountType = "Mainnet Account"

				// Separate XPX, SO (Storage Units), and SI (Streaming Units) mosaics
				for _, m := range accData.Account.Mosaics {
					raw := uint64(m.Amount[0]) | (uint64(m.Amount[1]) << 32)
					if m.Id[0] == 2679028825 && m.Id[1] == 1076571991 {
						// XPX Mosaic (divisibility 6)
						res.BalanceXPX = float64(raw) / 1000000.0
					} else if m.Id[0] == 1023420778 && m.Id[1] == 1123098103 {
						// SO Mosaic (Storage Units in MB)
						res.BalanceSO = float64(raw)
					} else if m.Id[0] == 2957921797 && m.Id[1] == 2117282881 {
						// SI Mosaic (Streaming Units in MB)
						res.BalanceSI = float64(raw)
					} else {
						// Generic fallback if not matched
						if res.BalanceXPX == 0 {
							res.BalanceXPX = float64(raw) / 1000000.0
						}
					}
				}
				sm.mu.Lock()
				sm.cachedAccount = res
				sm.lastAccountScan = time.Now()
				sm.mu.Unlock()
				return res
			}
		} else {
			resp.Body.Close()
			cancel()
		}
	}

	// Address derivation fallback if account is not yet on-chain
	if kp, err := crypto.KeyPairFromPrivateKey(pubKey); err == nil {
		res.Address = kp.Address
	}

	sm.mu.Lock()
	sm.cachedAccount = res
	sm.lastAccountScan = time.Now()
	sm.mu.Unlock()

	return res
}

// GetBootstrapReplicators reads replicators.json and returns the peers with live ping latencies
func (sm *StorageManager) GetBootstrapReplicators() []ReplicatorPeer {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	res := make([]ReplicatorPeer, len(sm.cachedPeers))
	copy(res, sm.cachedPeers)
	return res
}

func (sm *StorageManager) startBackgroundPeerChecker() {
	ticker := time.NewTicker(30 * time.Second)
	sm.refreshReplicatorPeers()

	for range ticker.C {
		sm.refreshReplicatorPeers()
	}
}

func (sm *StorageManager) refreshReplicatorPeers() {
	replicatorsFile := filepath.Join(sm.resourcesPath, "replicators.json")
	data, err := os.ReadFile(replicatorsFile)
	if err != nil {
		return
	}

	var parsed struct {
		KnownPeers []struct {
			PublicKey string `json:"publicKey"`
			Endpoint  struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"endpoint"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"knownPeers"`
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		return
	}

	var wg sync.WaitGroup
	peers := make([]ReplicatorPeer, len(parsed.KnownPeers))

	for i, p := range parsed.KnownPeers {
		wg.Add(1)
		go func(idx int, kp struct {
			PublicKey string `json:"publicKey"`
			Endpoint  struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"endpoint"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}) {
			defer wg.Done()
			addr := net.JoinHostPort(kp.Endpoint.Host, strconv.Itoa(kp.Endpoint.Port))
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			var latency int64
			reachable := false
			if err == nil {
				latency = time.Since(start).Milliseconds()
				reachable = true
				_ = conn.Close()
			}

			peers[idx] = ReplicatorPeer{
				Name:        kp.Metadata.Name,
				Host:        kp.Endpoint.Host,
				Port:        kp.Endpoint.Port,
				PublicKey:   kp.PublicKey,
				LatencyMs:   latency,
				IsReachable: reachable,
			}
		}(i, p)
	}

	wg.Wait()

	sm.mu.Lock()
	sm.cachedPeers = peers
	sm.lastPeerScan = time.Now()
	sm.mu.Unlock()
}

func (sm *StorageManager) GetStatus() StorageStatus {
	cfg, _ := sm.LoadStorageConfig()
	if cfg == nil {
		cfg = &StorageConfig{}
	}

	// Security: Never return raw private keys over the API
	cfgCopy := *cfg
	cfgCopy.Key = ""

	metrics := sm.GetStorageMetrics()
	onChain := sm.CheckOnChainAccount(cfg.PublicKey)
	peers := sm.GetBootstrapReplicators()

	return StorageStatus{
		Config:   cfgCopy,
		Metrics:  metrics,
		OnChain:  onChain,
		Peers:    peers,
		LastScan: time.Now().Format("2006-01-02 15:04:05 MST"),
	}
}

// OnboardReplicator constructs, signs, and announces a ReplicatorOnboardingTransaction using go-xpx-chain-sdk
func (sm *StorageManager) OnboardReplicator(ctx context.Context, capacityGB uint64, feeStrategyStr string, customApiNode string) (*OnboardResult, error) {
	cfg, err := sm.LoadStorageConfig()
	if err != nil || !cfg.IsConfigured || cfg.Key == "" {
		return nil, fmt.Errorf("replicator private key is not configured in config-storage.properties")
	}

	bootKey := ""
	if sm.bootKeyGetter != nil {
		bootKey = sm.bootKeyGetter()
	}
	bootKey = strings.TrimSpace(bootKey)
	if bootKey == "" || bootKey == "BOOTKEY_PRIVATE_KEY" || len(bootKey) != 64 {
		return nil, fmt.Errorf("valid node boot private key is required in config-user.properties to prove node identity")
	}

	if capacityGB == 0 {
		capacityGB = 50
	}

	feeStrategy := sdk.MiddleCalculationStrategy
	switch strings.ToLower(feeStrategyStr) {
	case "low":
		feeStrategy = sdk.LowCalculationStrategy
	case "high":
		feeStrategy = sdk.HighCalculationStrategy
	}

	apiNodes := crypto.DefaultApiNodes
	if customApiNode != "" {
		apiNodes = []string{customApiNode}
	}

	// Pre-flight Check: Replicator Balance
	onChain := sm.CheckOnChainAccount(cfg.PublicKey)
	if onChain.BalanceXPX < 1.0 {
		return nil, fmt.Errorf("insufficient XPX balance: Replicator address %s has %.3f XPX. Please send at least 100 XPX to this address to cover transaction fees and proof-of-space collateral before onboarding", cfg.Address, onChain.BalanceXPX)
	}

	var lastErr error
	for _, apiNode := range apiNodes {
		sdkCfg, err := sdk.NewConfig(ctx, []string{apiNode})
		if err != nil {
			lastErr = err
			continue
		}
		sdkCfg.FeeCalculationStrategy = feeStrategy
		client := sdk.NewClient(http.DefaultClient, sdkCfg)

		replicatorAccount, err := client.NewAccountFromPrivateKey(cfg.Key)
		if err != nil {
			lastErr = err
			continue
		}

		nodeAccount, err := client.NewAccountFromPrivateKey(bootKey)
		if err != nil {
			lastErr = err
			continue
		}

		var message sdk.Hash
		if _, err := rand.Read(message[:]); err != nil {
			lastErr = err
			continue
		}

		messageSignature, err := nodeAccount.SignData(message[:])
		if err != nil {
			lastErr = err
			continue
		}

		// Capacity is stored in Megabytes
		capacityMB := capacityGB * 1024
		replicatorOnboardingTx, err := client.NewReplicatorOnboardingTransaction(
			sdk.NewDeadline(time.Hour),
			sdk.Amount(capacityMB),
			nodeAccount.PublicAccount,
			&message,
			messageSignature,
		)
		if err != nil {
			lastErr = err
			continue
		}

		signedTx, err := replicatorAccount.Sign(replicatorOnboardingTx)
		if err != nil {
			lastErr = err
			continue
		}

		_, err = client.Transaction.Announce(ctx, signedTx)
		if err != nil {
			lastErr = err
			continue
		}

		// Poll transaction status to verify network acceptance
		for i := 0; i < 4; i++ {
			time.Sleep(1 * time.Second)
			statusUrl := fmt.Sprintf("%s/transaction/%s/status", strings.TrimRight(apiNode, "/"), signedTx.Hash.String())
			sReq, _ := http.NewRequestWithContext(ctx, "GET", statusUrl, nil)
			if sResp, sErr := sm.httpClient.Do(sReq); sErr == nil {
				var statusData struct {
					Group  string `json:"group"`
					Status string `json:"status"`
					Hash   string `json:"hash"`
				}
				_ = json.NewDecoder(sResp.Body).Decode(&statusData)
				sResp.Body.Close()
				if statusData.Status != "" {
					if strings.HasPrefix(statusData.Status, "Failure_") {
						return nil, fmt.Errorf("transaction rejected by Sirius network: %s", statusData.Status)
					}
					if statusData.Group == "unconfirmed" || statusData.Group == "confirmed" || statusData.Status == "Success" {
						break
					}
				}
			}
		}

		return &OnboardResult{
			TxHash:     signedTx.Hash.String(),
			Signer:     replicatorAccount.PublicAccount.PublicKey,
			CapacityGB: capacityGB,
			Status:     "Confirmed & Announced to Sirius Mainnet",
			Message:    fmt.Sprintf("ReplicatorOnboardingTransaction for %d GB announced successfully to Sirius Mainnet!", capacityGB),
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to announce replicator onboarding transaction: %w", lastErr)
	}
	return nil, fmt.Errorf("could not connect to any Sirius API node")
}

// Helpers for properties format
func ensurePropertiesFile(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		templatePath := filePath + ".template"
		if data, errT := os.ReadFile(templatePath); errT == nil {
			_ = os.WriteFile(filePath, data, 0644)
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

func writePropertiesFile(filePath string, section string, props map[string]string) error {
	var buf bytes.Buffer
	if section != "" {
		buf.WriteString(section + "\n\n")
	}

	for k, v := range props {
		buf.WriteString(fmt.Sprintf("%s = %s\n", k, v))
	}

	return os.WriteFile(filePath, buf.Bytes(), 0600)
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
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
