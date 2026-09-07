package chain

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var PublicMainnetNodes = []string{
	"https://aldebaran.xpxsirius.io",
	"https://betelgeuse.xpxsirius.io",
	"http://betelgeuse.xpxsirius.io:3000",
	"http://aldebaran.xpxsirius.io:3000",
}

type ChainHeightResponse struct {
	Height [2]uint64 `json:"height"`
}

type PeerInfo struct {
	PublicKey    string `json:"publicKey"`
	Port         int    `json:"port"`
	NetworkId    int    `json:"networkIdentifier"`
	Version      int    `json:"version"`
	Roles        int    `json:"roles"`
	Host         string `json:"host"`
	FriendlyName string `json:"friendlyName"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
}

type harvesterCacheEntry struct {
	status    *HarvesterStatus
	timestamp time.Time
}

type ChainMonitor struct {
	localBaseUrl    string
	httpClient      *http.Client
	harvesterCache  map[string]harvesterCacheEntry
	cacheMu         sync.RWMutex
	cachedNetHeight int64
	netHeightTime   time.Time
	cachedPeers     []PeerInfo
	peersTime       time.Time
}

func NewChainMonitor() *ChainMonitor {
	return &ChainMonitor{
		localBaseUrl: "http://127.0.0.1:3000",
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		harvesterCache: make(map[string]harvesterCacheEntry),
	}
}

// GetNetworkHeight queries reliable public nodes for current mainnet height and takes the highest
func (cm *ChainMonitor) GetNetworkHeight() (int64, error) {
	cm.cacheMu.RLock()
	h := cm.cachedNetHeight
	isFresh := h > 0 && time.Since(cm.netHeightTime) < 30*time.Second
	cm.cacheMu.RUnlock()

	if isFresh {
		return h, nil
	}

	if h > 0 {
		// Non-blocking refresh in background
		go cm.pollNetworkHeight()
		return h, nil
	}

	return cm.pollNetworkHeight()
}

func (cm *ChainMonitor) pollNetworkHeight() (int64, error) {
	var maxHeight int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, node := range PublicMainnetNodes {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			height, err := cm.queryHeightFromUrl(url)
			if err == nil {
				mu.Lock()
				if height > maxHeight {
					maxHeight = height
				}
				mu.Unlock()
			}
		}(node)
	}
	wg.Wait()
	if maxHeight > 0 {
		cm.cacheMu.Lock()
		cm.cachedNetHeight = maxHeight
		cm.netHeightTime = time.Now()
		cm.cacheMu.Unlock()
		return maxHeight, nil
	}

	cm.cacheMu.RLock()
	defer cm.cacheMu.RUnlock()
	if cm.cachedNetHeight > 0 {
		return cm.cachedNetHeight, nil
	}
	return 0, fmt.Errorf("all public nodes unreachable")
}

// GetLocalHeight reads the committed block height directly from chain data index.dat or local REST
func (cm *ChainMonitor) GetLocalHeight(dataPath string) (int64, error) {
	// 1. Direct binary read from index.dat (contains exact 8-byte uint64 height)
	candidates := []string{
		filepath.Join(dataPath, "index.dat"),
		filepath.Join(dataPath, "data", "index.dat"),
		"/app/chainconfig/data/index.dat",
	}

	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil && len(data) >= 8 {
			height := binary.LittleEndian.Uint64(data[:8])
			if height > 0 {
				return int64(height), nil
			}
		}
	}

	// 2. Fallback to local REST API if available
	return cm.queryHeightFromUrl(cm.localBaseUrl)
}

func (cm *ChainMonitor) queryHeightFromUrl(url string) (int64, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/chain/height", strings.TrimRight(url, "/")), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := cm.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var data ChainHeightResponse
	if err := json.Unmarshal(body, &data); err == nil && (data.Height[0] > 0 || data.Height[1] > 0) {
		height := int64(data.Height[0]) | (int64(data.Height[1]) << 32)
		return height, nil
	}

	var simple map[string]interface{}
	if err := json.Unmarshal(body, &simple); err == nil {
		if hVal, ok := simple["height"]; ok {
			if hArr, ok := hVal.([]interface{}); ok && len(hArr) > 0 {
				if low, ok := hArr[0].(float64); ok {
					return int64(low), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("unable to parse height")
}

// GetPeers queries connected peers
func (cm *ChainMonitor) GetPeers() ([]PeerInfo, error) {
	cm.cacheMu.RLock()
	if len(cm.cachedPeers) > 0 && time.Since(cm.peersTime) < 30*time.Second {
		peers := make([]PeerInfo, len(cm.cachedPeers))
		copy(peers, cm.cachedPeers)
		cm.cacheMu.RUnlock()
		return peers, nil
	}
	cm.cacheMu.RUnlock()

	var peers []PeerInfo
	urlsToTry := append([]string{cm.localBaseUrl}, PublicMainnetNodes...)

	for _, url := range urlsToTry {
		resp, err := cm.httpClient.Get(fmt.Sprintf("%s/node/peers", strings.TrimRight(url, "/")))
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err := json.Unmarshal(body, &peers); err == nil && len(peers) > 0 {
					cm.cacheMu.Lock()
					cm.cachedPeers = peers
					cm.peersTime = time.Now()
					cm.cacheMu.Unlock()
					return peers, nil
				}
			} else {
				resp.Body.Close()
			}
		}
	}

	cm.cacheMu.RLock()
	defer cm.cacheMu.RUnlock()
	if len(cm.cachedPeers) > 0 {
		return cm.cachedPeers, nil
	}
	return peers, nil
}

// ExecuteRestQuery proxies REST query for debugging console
func (cm *ChainMonitor) ExecuteRestQuery(endpoint string) (interface{}, error) {
	endpoint = strings.TrimLeft(endpoint, "/")
	url := fmt.Sprintf("%s/%s", PublicMainnetNodes[0], endpoint)
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	return result, nil
}

// GetPublicIP resolves external public IP
func (cm *ChainMonitor) GetPublicIP() (string, error) {
	ipResolvers := []string{
		"https://api.ipify.org?format=text",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
		"http://checkip.amazonaws.com",
	}

	client := &http.Client{Timeout: 4 * time.Second}
	for _, resolver := range ipResolvers {
		resp, err := client.Get(resolver)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				ip := strings.TrimSpace(string(body))
				if len(ip) >= 7 && len(ip) <= 45 && !strings.Contains(ip, "<") {
					return ip, nil
				}
			}
		}
	}
	return "", fmt.Errorf("unable to resolve public IP")
}

type HarvesterStatus struct {
	IsLinked               bool    `json:"isLinked"`
	AccountType            int     `json:"accountType"` // 0: Normal, 1: Main with linked remote, 2: Remote Harvester
	AccountAddress         string  `json:"accountAddress,omitempty"`
	AccountPublicKey       string  `json:"accountPublicKey,omitempty"`
	LinkedPublicKey        string  `json:"linkedPublicKey,omitempty"`
	LinkedAddress          string  `json:"linkedAddress,omitempty"`
	LinkedBalanceXPX       string  `json:"linkedBalanceXPX,omitempty"`
	LinkedRawBalanceXPX    int64   `json:"linkedRawBalanceXPX,omitempty"`
	BalanceXPX             string  `json:"balanceXPX,omitempty"`
	RawBalanceXPX          int64   `json:"rawBalanceXPX,omitempty"`
	IsCommitteeHarvester   bool    `json:"isCommitteeHarvester"`
	CanHarvest             bool    `json:"canHarvest"`
	EffectiveBalance       string  `json:"effectiveBalance,omitempty"`
	LastSigningBlockHeight uint64  `json:"lastSigningBlockHeight,omitempty"`
	Activity               float64 `json:"activity,omitempty"`
	Greed                  float64 `json:"greed,omitempty"`
	IsEligible             bool    `json:"isEligible"`
	StatusText             string  `json:"statusText"`
	ExplorerUrl            string  `json:"explorerUrl,omitempty"`
	LinkedExplorerUrl      string  `json:"linkedExplorerUrl,omitempty"`
}

type accountFetchResult struct {
	Address         string
	PublicKey       string
	AccountType     int
	LinkedKey       string
	BalanceXPX      string
	RawBalanceXPX   int64
	MosaicsFound    bool
}

func (cm *ChainMonitor) fetchAccountData(identifier, apiNode string) (*accountFetchResult, error) {
	url := fmt.Sprintf("%s/account/%s", apiNode, strings.TrimSpace(identifier))
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("account not found (HTTP %d)", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	accMap, ok := raw["account"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid account response structure")
	}

	res := &accountFetchResult{}
	if addr, ok := accMap["address"].(string); ok {
		res.Address = addr
	}
	if pubKey, ok := accMap["publicKey"].(string); ok {
		res.PublicKey = pubKey
	}
	if accType, ok := accMap["accountType"].(float64); ok {
		res.AccountType = int(accType)
	}
	if linkedKey, ok := accMap["linkedAccountKey"].(string); ok {
		isAllZeros := strings.Trim(linkedKey, "0") == ""
		if linkedKey != "" && !isAllZeros && len(linkedKey) == 64 {
			res.LinkedKey = linkedKey
		}
	}

	if mosaics, ok := accMap["mosaics"].([]interface{}); ok {
		for _, mItem := range mosaics {
			if mMap, ok := mItem.(map[string]interface{}); ok {
				var totalAmount uint64
				if amtArr, ok := mMap["amount"].([]interface{}); ok && len(amtArr) >= 2 {
					low := uint64(amtArr[0].(float64))
					high := uint64(amtArr[1].(float64))
					totalAmount = (high << 32) | low
				} else if amtNum, ok := mMap["amount"].(float64); ok {
					totalAmount = uint64(amtNum)
				}

				res.RawBalanceXPX = int64(totalAmount)
				xpxVal := float64(totalAmount) / 1000000.0
				res.BalanceXPX = fmt.Sprintf("%.6f XPX", xpxVal)
				res.MosaicsFound = true
			}
		}
	}
	if res.BalanceXPX == "" {
		res.BalanceXPX = "0.000000 XPX"
	}

	return res, nil
}

// CheckHarvesterStatus queries explorer / public REST API to detect harvesting state & eligibility
func (cm *ChainMonitor) CheckHarvesterStatus(identifier, apiNode string) (*HarvesterStatus, error) {
	if apiNode == "" {
		apiNode = PublicMainnetNodes[0]
	}
	apiNode = strings.TrimRight(apiNode, "/")

	cacheKey := fmt.Sprintf("%s|%s", strings.TrimSpace(identifier), apiNode)
	cm.cacheMu.RLock()
	if entry, found := cm.harvesterCache[cacheKey]; found {
		if time.Since(entry.timestamp) < 20*time.Second {
			cm.cacheMu.RUnlock()
			return entry.status, nil
		}
	}
	cm.cacheMu.RUnlock()

	primary, err := cm.fetchAccountData(identifier, apiNode)
	if err != nil {
		return &HarvesterStatus{
			IsLinked:    false,
			IsEligible:  false,
			StatusText:  fmt.Sprintf("Account %s not found on Sirius Mainnet (%v).", identifier, err),
			ExplorerUrl: fmt.Sprintf("https://explorer.xpxsirius.io/#/account/%s", identifier),
		}, nil
	}

	status := &HarvesterStatus{
		AccountType:      primary.AccountType,
		AccountAddress:   primary.Address,
		AccountPublicKey: primary.PublicKey,
		BalanceXPX:       primary.BalanceXPX,
		RawBalanceXPX:    primary.RawBalanceXPX,
		ExplorerUrl:      fmt.Sprintf("https://explorer.xpxsirius.io/#/account/%s", primary.Address),
	}

	var harvesterKey string

	// Case A: Remote Harvester Account (accountType == 2)
	if primary.AccountType == 2 && primary.LinkedKey != "" {
		status.IsLinked = true
		status.LinkedPublicKey = primary.LinkedKey
		harvesterKey = primary.PublicKey

		// Fetch linked owner/main account data
		if ownerData, oErr := cm.fetchAccountData(primary.LinkedKey, apiNode); oErr == nil {
			status.LinkedAddress = ownerData.Address
			status.LinkedBalanceXPX = ownerData.BalanceXPX
			status.LinkedRawBalanceXPX = ownerData.RawBalanceXPX
			status.LinkedExplorerUrl = fmt.Sprintf("https://explorer.xpxsirius.io/#/account/%s", ownerData.Address)

			// POS+ committee eligibility is evaluated on owner's balance
			if float64(ownerData.RawBalanceXPX)/1000000.0 >= 1000000.0 {
				status.IsEligible = true
			}
		}
	} else if primary.AccountType == 1 && primary.LinkedKey != "" {
		// Case B: Main Account with linked remote harvester
		status.IsLinked = true
		status.LinkedPublicKey = primary.LinkedKey
		harvesterKey = primary.LinkedKey

		if float64(primary.RawBalanceXPX)/1000000.0 >= 1000000.0 {
			status.IsEligible = true
		}

		if remoteData, rErr := cm.fetchAccountData(primary.LinkedKey, apiNode); rErr == nil {
			status.LinkedAddress = remoteData.Address
			status.LinkedBalanceXPX = remoteData.BalanceXPX
			status.LinkedRawBalanceXPX = remoteData.RawBalanceXPX
			status.LinkedExplorerUrl = fmt.Sprintf("https://explorer.xpxsirius.io/#/account/%s", remoteData.Address)
		}
	} else {
		// Case C: Unlinked Account
		harvesterKey = primary.PublicKey
		if float64(primary.RawBalanceXPX)/1000000.0 >= 1000000.0 {
			status.IsEligible = true
		}
	}

	// Query /account/{harvesterKey}/harvesting for POS+ Committee registration
	if harvesterKey != "" {
		hUrl := fmt.Sprintf("%s/account/%s/harvesting", apiNode, harvesterKey)
		hResp, hErr := cm.httpClient.Get(hUrl)
		if hErr == nil && hResp.StatusCode == http.StatusOK {
			var hList []struct {
				Harvester struct {
					Key                    string    `json:"key"`
					Owner                  string    `json:"owner"`
					Address                string    `json:"address"`
					DisabledHeight         [2]uint64 `json:"disabledHeight"`
					LastSigningBlockHeight [2]uint64 `json:"lastSigningBlockHeight"`
					EffectiveBalance       [2]uint64 `json:"effectiveBalance"`
					CanHarvest             bool      `json:"canHarvest"`
					Activity               float64   `json:"activity"`
					Greed                  float64   `json:"greed"`
				} `json:"harvester"`
			}

			if decErr := json.NewDecoder(hResp.Body).Decode(&hList); decErr == nil && len(hList) > 0 {
				hItem := hList[0].Harvester
				status.IsCommitteeHarvester = true
				status.CanHarvest = hItem.CanHarvest
				effAmt := (hItem.EffectiveBalance[1] << 32) | hItem.EffectiveBalance[0]
				status.EffectiveBalance = fmt.Sprintf("%.6f XPX", float64(effAmt)/1000000.0)
				status.LastSigningBlockHeight = (hItem.LastSigningBlockHeight[1] << 32) | hItem.LastSigningBlockHeight[0]
				status.Activity = hItem.Activity
				status.Greed = hItem.Greed
			}
			hResp.Body.Close()
		} else if hResp != nil && hResp.Body != nil {
			hResp.Body.Close()
		}
	}

	// Status text formatting
	if status.IsCommitteeHarvester && status.CanHarvest {
		status.StatusText = "Active Committee Harvester: Registered in POS+ Committee cache, eligible and validating blocks on Sirius Mainnet."
	} else if status.IsLinked {
		if status.IsEligible {
			status.StatusText = "Account is linked with >= 1,000,000 XPX stake. Ready for committee harvester block creation."
		} else {
			status.StatusText = "Account is linked, but the linked staking balance is below 1,000,000 XPX committee requirement."
		}
	} else {
		if status.IsEligible {
			status.StatusText = "Account has >= 1,000,000 XPX stake but has not linked a remote harvesting key yet."
		} else {
			status.StatusText = "Account is unlinked and balance is below harvesting threshold."
		}
	}

	cm.cacheMu.Lock()
	cm.harvesterCache[cacheKey] = harvesterCacheEntry{
		status:    status,
		timestamp: time.Now(),
	}
	cm.cacheMu.Unlock()

	return status, nil
}
