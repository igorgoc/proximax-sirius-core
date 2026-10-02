package chain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type NetworkBlockInfo struct {
	Height          int64     `json:"height"`
	Signer          string    `json:"signer"`
	Timestamp       time.Time `json:"timestamp"`
	FeeXPX          float64   `json:"feeXPX"`
	NumTransactions int       `json:"numTransactions"`
}

type ActiveValidatorSummary struct {
	PublicKey        string  `json:"publicKey"`
	ShortKey         string  `json:"shortKey"`
	BlocksCount      int     `json:"blocksCount"`
	SharePercent     float64 `json:"sharePercent"`
	StakedBalanceXPX float64 `json:"stakedBalanceXPX"`
	LastSeenHeight   int64   `json:"lastSeenHeight"`
	LastSeenTime     string  `json:"lastSeenTime"`
	IsSelf           bool    `json:"isSelf"`
}

type NetworkValidatorStats struct {
	ActiveValidators4h     int                      `json:"activeValidators4h"`
	ActiveValidators24h    int                      `json:"activeValidators24h"`
	EstimatedStakedPoolXPX float64                  `json:"estimatedStakedPoolXPX"`
	AvgBlockTimeSec        float64                  `json:"avgBlockTimeSec"`
	TotalNetworkFees4h     float64                  `json:"totalNetworkFees4h"`
	RecentBlocksCount      int                      `json:"recentBlocksCount"`
	LatestBlockSigner      string                   `json:"latestBlockSigner"`
	TopValidators          []ActiveValidatorSummary `json:"topValidators"`
}

type validatorBalanceEntry struct {
	stakedXPX float64
	fetchedAt time.Time
}

type NetworkValidatorTracker struct {
	mu              sync.RWMutex
	recentBlocks    []NetworkBlockInfo
	maxBlocks       int
	balanceCache    map[string]validatorBalanceEntry
	inFlight        map[string]bool
	httpClient      *http.Client
	selfStakedXPX   float64
	apiNodes        []string
	isBatchFetching atomic.Bool
	stopChan        chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
}

func NewNetworkValidatorTracker(nodes ...string) *NetworkValidatorTracker {
	var targetNodes []string
	if len(nodes) > 0 {
		targetNodes = make([]string, len(nodes))
		copy(targetNodes, nodes)
	} else {
		targetNodes = GetPublicMainnetNodes()
	}
	return &NetworkValidatorTracker{
		recentBlocks: make([]NetworkBlockInfo, 0, 1000),
		maxBlocks:    1000, // ~4.1 hours of blocks at 15s cadence
		balanceCache: make(map[string]validatorBalanceEntry),
		inFlight:     make(map[string]bool),
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		apiNodes: targetNodes,
		stopChan: make(chan struct{}),
	}
}

// SetNodes updates the public nodes used by this tracker
func (nvt *NetworkValidatorTracker) SetNodes(nodes []string) {
	nvt.mu.Lock()
	defer nvt.mu.Unlock()
	nvt.apiNodes = make([]string, len(nodes))
	copy(nvt.apiNodes, nodes)
}

func (nvt *NetworkValidatorTracker) getNodes() []string {
	nvt.mu.RLock()
	defer nvt.mu.RUnlock()
	if len(nvt.apiNodes) > 0 {
		nodes := make([]string, len(nvt.apiNodes))
		copy(nodes, nvt.apiNodes)
		return nodes
	}
	return GetPublicMainnetNodes()
}

// Stop cleanly terminates in-flight background worker goroutines
func (nvt *NetworkValidatorTracker) Stop() {
	nvt.stopOnce.Do(func() {
		close(nvt.stopChan)
	})
	nvt.wg.Wait()
}

// SetSelfStakedBalance allows setting/overriding self validator balance directly
func (nvt *NetworkValidatorTracker) SetSelfStakedBalance(pubKey string, balance float64) {
	if balance <= 0 {
		return
	}
	nvt.mu.Lock()
	defer nvt.mu.Unlock()
	nvt.selfStakedXPX = balance
	if pubKey != "" {
		cleanKey := strings.ToUpper(strings.TrimSpace(pubKey))
		nvt.balanceCache[cleanKey] = validatorBalanceEntry{
			stakedXPX: balance,
			fetchedAt: time.Now(),
		}
	}
}

// RecordBlock adds a block to the ring buffer, deduplicating by height
func (nvt *NetworkValidatorTracker) RecordBlock(block NetworkBlockInfo) {
	nvt.mu.Lock()
	defer nvt.mu.Unlock()

	for _, b := range nvt.recentBlocks {
		if b.Height == block.Height {
			return
		}
	}

	// Insert keeping sorted by height descending (latest first)
	inserted := false
	for i, b := range nvt.recentBlocks {
		if block.Height > b.Height {
			nvt.recentBlocks = append(nvt.recentBlocks[:i], append([]NetworkBlockInfo{block}, nvt.recentBlocks[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		nvt.recentBlocks = append(nvt.recentBlocks, block)
	}

	if len(nvt.recentBlocks) > nvt.maxBlocks {
		nvt.recentBlocks = nvt.recentBlocks[:nvt.maxBlocks]
	}
}

// resolveStakedBalance queries public nodes to fetch true staked XPX (resolving linked owner for remote harvesters)
func (nvt *NetworkValidatorTracker) resolveStakedBalance(signerPubKey string) (float64, error) {
	cleanSigner := NormalizeAccountIdentifier(signerPubKey)
	for _, node := range nvt.getNodes() {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), cleanSigner)
		resp, err := nvt.httpClient.Get(url)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var raw map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			continue
		}

		accMap, ok := raw["account"].(map[string]interface{})
		if !ok {
			continue
		}

		accType := 0
		if at, ok := accMap["accountType"].(float64); ok {
			accType = int(at)
		}

		targetAccMap := accMap

		// In Sirius PoS+, if accountType == 2 (Remote Harvester), staked funds reside in the linked main owner account
		if accType == 2 {
			if linkedKey, ok := accMap["linkedAccountKey"].(string); ok {
				cleanLinked := strings.TrimSpace(linkedKey)
				if len(cleanLinked) == 64 && strings.Trim(cleanLinked, "0") != "" {
					linkedUrl := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), cleanLinked)
					lResp, lErr := nvt.httpClient.Get(linkedUrl)
					if lErr == nil && lResp.StatusCode == http.StatusOK {
						var lRaw map[string]interface{}
						if decErr := json.NewDecoder(lResp.Body).Decode(&lRaw); decErr == nil {
							if lAcc, ok := lRaw["account"].(map[string]interface{}); ok {
								targetAccMap = lAcc
							}
						}
						lResp.Body.Close()
					}
				}
			}
		}

		// Extract XPX balance from targetAccMap mosaics
		if mosaics, ok := targetAccMap["mosaics"].([]interface{}); ok {
			var xpxAmount uint64
			var xpxFound bool
			for _, mItem := range mosaics {
				mMap, ok := mItem.(map[string]interface{})
				if !ok {
					continue
				}
				var isXPX bool
				if idArr, ok := mMap["id"].([]interface{}); ok && len(idArr) >= 2 {
					low := uint64(idArr[0].(float64))
					high := uint64(idArr[1].(float64))
					// XPX Currency Mosaic ID: 0x402B2F579FAEBC59 (low: 2679028825, high: 1076571991)
					if low == 2679028825 && high == 1076571991 {
						isXPX = true
					}
				}

				var totalAmount uint64
				if amtArr, ok := mMap["amount"].([]interface{}); ok && len(amtArr) >= 2 {
					low := uint64(amtArr[0].(float64))
					high := uint64(amtArr[1].(float64))
					totalAmount = (high << 32) | low
				} else if amtNum, ok := mMap["amount"].(float64); ok {
					totalAmount = uint64(amtNum)
				}

				if isXPX {
					xpxAmount = totalAmount
					xpxFound = true
					break
				}
				if !xpxFound && totalAmount > 0 {
					xpxAmount = totalAmount
				}
			}

			if xpxFound || xpxAmount > 0 {
				return float64(xpxAmount) / 1000000.0, nil
			}
		}

		return 0, nil
	}

	return 0, fmt.Errorf("failed to resolve balance from public nodes")
}

// triggerBalanceFetch starts an async background query to fetch and cache validator's true staked XPX
func (nvt *NetworkValidatorTracker) triggerBalanceFetch(signer string) {
	select {
	case <-nvt.stopChan:
		return
	default:
	}

	nvt.mu.Lock()
	if nvt.inFlight[signer] {
		nvt.mu.Unlock()
		return
	}
	nvt.inFlight[signer] = true
	nvt.mu.Unlock()

	nvt.wg.Add(1)
	go func() {
		defer nvt.wg.Done()
		defer func() {
			nvt.mu.Lock()
			delete(nvt.inFlight, signer)
			nvt.mu.Unlock()
		}()

		balance, err := nvt.resolveStakedBalance(signer)
		nvt.mu.Lock()
		if err == nil {
			nvt.balanceCache[signer] = validatorBalanceEntry{
				stakedXPX: balance,
				fetchedAt: time.Now(),
			}
		} else {
			// On error, cache for 1 minute before retrying to prevent rapid spamming under polling
			nvt.balanceCache[signer] = validatorBalanceEntry{
				stakedXPX: 0,
				fetchedAt: time.Now().Add(-4 * time.Minute),
			}
		}
		nvt.mu.Unlock()
	}()
}

// GetStats computes active validators, cadence, and staking pool estimates
func (nvt *NetworkValidatorTracker) GetStats(selfPubKey string) NetworkValidatorStats {
	nvt.mu.RLock()
	defer nvt.mu.RUnlock()

	selfClean := strings.ToUpper(strings.TrimSpace(selfPubKey))
	now := time.Now().UTC()
	cutoff4h := now.Add(-4 * time.Hour)
	cutoff24h := now.Add(-24 * time.Hour)

	signers4h := make(map[string]int)
	signers24h := make(map[string]int)
	signerLastHeight := make(map[string]int64)
	signerLastTime := make(map[string]time.Time)

	var totalFees4h float64
	var count4h int
	var intervalsSum float64
	var intervalsCount int

	for i, b := range nvt.recentBlocks {
		if b.Signer == "" {
			continue
		}

		signer := strings.ToUpper(b.Signer)
		is4h := b.Timestamp.After(cutoff4h) || i < 480 // fallback to last 480 blocks if timestamps skewed

		if is4h {
			signers4h[signer]++
			totalFees4h += b.FeeXPX
			count4h++
		}

		if b.Timestamp.After(cutoff24h) || i < 1000 {
			signers24h[signer]++
		}

		if h, exists := signerLastHeight[signer]; !exists || b.Height > h {
			signerLastHeight[signer] = b.Height
			signerLastTime[signer] = b.Timestamp
		}

		// Calculate block time intervals between adjacent blocks
		if i < len(nvt.recentBlocks)-1 {
			prev := nvt.recentBlocks[i+1]
			if b.Height == prev.Height+1 && !b.Timestamp.IsZero() && !prev.Timestamp.IsZero() {
				diffSec := b.Timestamp.Sub(prev.Timestamp).Seconds()
				if diffSec > 0 && diffSec < 120 {
					intervalsSum += diffSec
					intervalsCount++
				}
			}
		}
	}

	active4h := len(signers4h)
	active24h := len(signers24h)
	if active24h < active4h {
		active24h = active4h
	}

	// Fallback realistic defaults if scanner just started
	if active4h == 0 {
		active4h = 1
		active24h = 1
	}

	avgBlockTime := 15.0
	if intervalsCount > 0 {
		avgBlockTime = intervalsSum / float64(intervalsCount)
		if avgBlockTime < 5.0 || avgBlockTime > 45.0 {
			avgBlockTime = 15.0
		}
	}

	// Build Top Validators list
	topList := make([]ActiveValidatorSummary, 0, len(signers4h))
	var signersToFetch []string

	for signer, count := range signers4h {
		shortKey := signer
		if len(shortKey) >= 4 {
			shortKey = shortKey[:4]
		}

		share := 0.0
		if count4h > 0 {
			share = (float64(count) / float64(count4h)) * 100.0
		}

		lastTimeStr := "Recently"
		if t, ok := signerLastTime[signer]; ok && !t.IsZero() {
			lastTimeStr = t.UTC().Format("2006-01-02 15:04:05 UTC")
		}

		isSelf := signer == selfClean
		stakedBalance := 0.0

		if isSelf && nvt.selfStakedXPX > 0 {
			stakedBalance = nvt.selfStakedXPX
		} else if entry, ok := nvt.balanceCache[signer]; ok {
			stakedBalance = entry.stakedXPX
		}

		// Trigger background fetch if not in cache or cached > 5m ago
		if entry, ok := nvt.balanceCache[signer]; !ok || time.Since(entry.fetchedAt) > 5*time.Minute {
			signersToFetch = append(signersToFetch, signer)
		}

		topList = append(topList, ActiveValidatorSummary{
			PublicKey:        signer,
			ShortKey:         shortKey,
			BlocksCount:      count,
			SharePercent:     share,
			StakedBalanceXPX: stakedBalance,
			LastSeenHeight:   signerLastHeight[signer],
			LastSeenTime:     lastTimeStr,
			IsSelf:           isSelf,
		})
	}

	sort.Slice(topList, func(i, j int) bool {
		return topList[i].BlocksCount > topList[j].BlocksCount
	})

	// Limit to top 8 active signers
	if len(topList) > 8 {
		topList = topList[:8]
	}

	// Asynchronously trigger balance queries for needed signers outside the lock
	if len(signersToFetch) > 0 {
		if nvt.isBatchFetching.CompareAndSwap(false, true) {
			nvt.wg.Add(1)
			go func(keys []string) {
				defer nvt.wg.Done()
				defer nvt.isBatchFetching.Store(false)
				for _, k := range keys {
					select {
					case <-nvt.stopChan:
						return
					default:
					}
					nvt.triggerBalanceFetch(k)
					time.Sleep(50 * time.Millisecond) // gentle rate pacing
				}
			}(signersToFetch)
		}
	}

	// Calculate total known staked pool
	var totalKnownStaked float64
	for _, v := range topList {
		totalKnownStaked += v.StakedBalanceXPX
	}

	estimatedPoolXPX := float64(active4h) * 6000000.0
	if totalKnownStaked > estimatedPoolXPX {
		estimatedPoolXPX = totalKnownStaked
	}

	latestBlockSigner := ""
	if len(nvt.recentBlocks) > 0 {
		latestBlockSigner = strings.ToUpper(nvt.recentBlocks[0].Signer)
	}

	return NetworkValidatorStats{
		ActiveValidators4h:     active4h,
		ActiveValidators24h:    active24h,
		EstimatedStakedPoolXPX: estimatedPoolXPX,
		AvgBlockTimeSec:        avgBlockTime,
		TotalNetworkFees4h:     totalFees4h,
		RecentBlocksCount:      len(nvt.recentBlocks),
		LatestBlockSigner:      latestBlockSigner,
		TopValidators:          topList,
	}
}
