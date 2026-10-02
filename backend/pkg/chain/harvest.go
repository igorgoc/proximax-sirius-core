package chain

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ValidatedBlock struct {
	Height          int64   `json:"height"`
	Hash            string  `json:"hash"`
	Timestamp       string  `json:"timestamp"`
	FeeXPX          float64 `json:"feeXPX"`
	NumTransactions int     `json:"numTransactions"`
	Signer          string  `json:"signer"`
}

type HarvestStats struct {
	TotalBlocksValidated int64            `json:"totalBlocksValidated"`
	TotalEarnedFeesXPX   float64          `json:"totalEarnedFeesXPX"`
	LastHarvestedHeight  int64            `json:"lastHarvestedHeight,omitempty"`
	LastHarvestedTime    string           `json:"lastHarvestedTime,omitempty"`
	ValidatedBlocks      []ValidatedBlock `json:"validatedBlocks"`
}

type BlockApiResponse struct {
	Meta struct {
		Hash            string    `json:"hash"`
		TotalFee        [2]uint64 `json:"totalFee"`
		NumTransactions int       `json:"numTransactions"`
	} `json:"meta"`
	Block struct {
		Signer    string    `json:"signer"`
		Height    [2]uint64 `json:"height"`
		Timestamp [2]uint64 `json:"timestamp"`
	} `json:"block"`
}

type HarvesterTracker struct {
	mu                sync.RWMutex
	stats             HarvestStats
	statsFilePath     string
	harvestPublicKey  string
	lastScannedHeight int64
	networkValidators *NetworkValidatorTracker
	httpClient        *http.Client
	stopChan          chan struct{}
	stopOnce          sync.Once
	isChecking        atomic.Bool
	wg                sync.WaitGroup
}

func NewHarvesterTracker(resourcesDir string) *HarvesterTracker {
	statsFile := filepath.Join(resourcesDir, "harvest-stats.json")
	tracker := &HarvesterTracker{
		statsFilePath: statsFile,
		stats: HarvestStats{
			ValidatedBlocks: make([]ValidatedBlock, 0),
		},
		networkValidators: NewNetworkValidatorTracker(),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		stopChan: make(chan struct{}),
	}

	tracker.loadStats()
	return tracker
}

func (ht *HarvesterTracker) SetHarvestPublicKey(pubKey string) {
	cleanKey := strings.ToUpper(strings.TrimSpace(pubKey))
	ht.mu.Lock()
	ht.harvestPublicKey = cleanKey
	ht.mu.Unlock()

	if cleanKey != "" && cleanKey != "REMOTE_ACCOUNT_PUBLIC_KEY" {
		ht.networkValidators.triggerBalanceFetch(cleanKey)
	}
}

func (ht *HarvesterTracker) GetNetworkValidatorStats() NetworkValidatorStats {
	ht.mu.RLock()
	selfKey := ht.harvestPublicKey
	ht.mu.RUnlock()
	return ht.networkValidators.GetStats(selfKey)
}

func (ht *HarvesterTracker) loadStats() {
	data, err := os.ReadFile(ht.statsFilePath)
	if err != nil {
		return
	}
	var loaded HarvestStats
	if err := json.Unmarshal(data, &loaded); err == nil {
		ht.stats = loaded
		if ht.stats.ValidatedBlocks == nil {
			ht.stats.ValidatedBlocks = make([]ValidatedBlock, 0)
		} else if len(ht.stats.ValidatedBlocks) > 50 {
			ht.stats.ValidatedBlocks = ht.stats.ValidatedBlocks[:50]
		}
	}
}

func (ht *HarvesterTracker) saveStats() {
	data, err := json.MarshalIndent(ht.stats, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(ht.statsFilePath, data, 0644)
}

func (ht *HarvesterTracker) GetStats() HarvestStats {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	// Return a copy
	res := ht.stats
	blocksCopy := make([]ValidatedBlock, len(ht.stats.ValidatedBlocks))
	copy(blocksCopy, ht.stats.ValidatedBlocks)
	res.ValidatedBlocks = blocksCopy
	return res
}

func (ht *HarvesterTracker) ResetStats(target string) HarvestStats {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	switch strings.ToLower(strings.TrimSpace(target)) {
	case "blocks":
		ht.stats.TotalBlocksValidated = 0
		ht.stats.ValidatedBlocks = make([]ValidatedBlock, 0)
		ht.stats.LastHarvestedHeight = 0
		ht.stats.LastHarvestedTime = ""
	case "fees":
		ht.stats.TotalEarnedFeesXPX = 0
	default: // "all" or empty
		ht.stats = HarvestStats{
			TotalBlocksValidated: 0,
			TotalEarnedFeesXPX:   0,
			LastHarvestedHeight:  0,
			LastHarvestedTime:    "",
			ValidatedBlocks:      make([]ValidatedBlock, 0),
		}
	}
	ht.saveStats()
	return ht.stats
}

func (ht *HarvesterTracker) RecordValidatedBlock(block ValidatedBlock) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	// Check if already recorded to avoid duplicates
	for _, b := range ht.stats.ValidatedBlocks {
		if b.Height == block.Height {
			return
		}
	}

	// Prepend new block
	ht.stats.ValidatedBlocks = append([]ValidatedBlock{block}, ht.stats.ValidatedBlocks...)
	// Limit stored recent blocks to 50 max
	if len(ht.stats.ValidatedBlocks) > 50 {
		ht.stats.ValidatedBlocks = ht.stats.ValidatedBlocks[:50]
	}

	ht.stats.TotalBlocksValidated++
	ht.stats.TotalEarnedFeesXPX += block.FeeXPX
	ht.stats.LastHarvestedHeight = block.Height
	ht.stats.LastHarvestedTime = block.Timestamp

	ht.saveStats()
}

// CheckBlock queries the block at given height and records it if validated by this harvester
func (ht *HarvesterTracker) CheckBlock(height int64) error {
	nodes := GetPublicMainnetNodes()
	for _, node := range nodes {
		url := fmt.Sprintf("%s/block/%d", strings.TrimRight(node, "/"), height)
		resp, err := ht.httpClient.Get(url)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var blockData BlockApiResponse
		if err := json.Unmarshal(body, &blockData); err != nil {
			continue
		}

		blockSigner := strings.ToUpper(strings.TrimSpace(blockData.Block.Signer))
		rawFee := uint64(blockData.Meta.TotalFee[0]) | (uint64(blockData.Meta.TotalFee[1]) << 32)
		feeXPX := float64(rawFee) / 1000000.0

		rawTs := uint64(blockData.Block.Timestamp[0]) | (uint64(blockData.Block.Timestamp[1]) << 32)
		var blockTimeUtc time.Time
		if rawTs > 0 {
			// Sirius Mainnet Nemesis Epoch: 1459468800 (2016-04-01 00:00:00 UTC)
			blockTimeUtc = time.Unix(1459468800+int64(rawTs/1000), int64((rawTs%1000)*1000000)).UTC()
		} else {
			blockTimeUtc = time.Now().UTC()
		}

		// Record in network validator tracker for network-wide consensus analytics
		ht.networkValidators.RecordBlock(NetworkBlockInfo{
			Height:          height,
			Signer:          blockSigner,
			Timestamp:       blockTimeUtc,
			FeeXPX:          feeXPX,
			NumTransactions: blockData.Meta.NumTransactions,
		})

		ht.mu.RLock()
		pubKey := ht.harvestPublicKey
		ht.mu.RUnlock()

		if pubKey != "" && pubKey != "REMOTE_ACCOUNT_PUBLIC_KEY" && blockSigner == pubKey {
			ht.RecordValidatedBlock(ValidatedBlock{
				Height:          height,
				Hash:            blockData.Meta.Hash,
				Timestamp:       blockTimeUtc.Format("2006-01-02 15:04:05 UTC"),
				FeeXPX:          feeXPX,
				NumTransactions: blockData.Meta.NumTransactions,
				Signer:          blockSigner,
			})
		}
		return nil
	}

	return fmt.Errorf("failed to fetch block %d from public nodes", height)
}

// StartBackgroundScanner periodically and randomly checks for newly validated blocks without spamming explorer/nodes
func (ht *HarvesterTracker) StartBackgroundScanner(cm *ChainMonitor) {
	ht.wg.Add(1)
	go func() {
		defer ht.wg.Done()
		updateSelfBalance := func() {
			ht.mu.RLock()
			pubKey := ht.harvestPublicKey
			ht.mu.RUnlock()
			if pubKey != "" && pubKey != "REMOTE_ACCOUNT_PUBLIC_KEY" {
				if hStatus, err := cm.CheckHarvesterStatus(pubKey, ""); err == nil && hStatus != nil {
					var selfXPX float64
					if hStatus.AccountType == 2 && hStatus.LinkedRawBalanceXPX > 0 {
						selfXPX = float64(hStatus.LinkedRawBalanceXPX) / 1000000.0
					} else if hStatus.RawBalanceXPX > 0 {
						selfXPX = float64(hStatus.RawBalanceXPX) / 1000000.0
					}
					if selfXPX > 0 {
						ht.networkValidators.SetSelfStakedBalance(pubKey, selfXPX)
					}
				}
			}
		}

		// Initial bootstrap: Scan the last 15 blocks on startup to populate network validator metrics immediately
		select {
		case <-ht.stopChan:
			return
		case <-time.After(2 * time.Second):
		}
		updateSelfBalance()

		if netHeight, err := cm.GetNetworkHeight(); err == nil && netHeight > 15 {
			for h := netHeight - 15; h <= netHeight; h++ {
				select {
				case <-ht.stopChan:
					return
				default:
				}
				_ = ht.CheckBlock(h)
				time.Sleep(120 * time.Millisecond)
			}
			ht.mu.Lock()
			ht.lastScannedHeight = netHeight
			ht.mu.Unlock()
		}

		loopCounter := 0
		for {
			select {
			case <-ht.stopChan:
				return
			default:
			}

			loopCounter++
			if loopCounter%3 == 0 {
				updateSelfBalance()
			}

			netHeight, err := cm.GetNetworkHeight()
			if err == nil && netHeight > 0 {
				ht.mu.Lock()
				if ht.lastScannedHeight == 0 {
					ht.lastScannedHeight = netHeight - 5
					if ht.lastScannedHeight < 1 {
						ht.lastScannedHeight = 1
					}
				}
				startH := ht.lastScannedHeight + 1
				ht.mu.Unlock()

				if startH <= netHeight {
					// Only check at most 5 blocks per check to stay lightweight
					endH := netHeight
					if endH-startH > 5 {
						endH = startH + 4
					}

					for h := startH; h <= endH; h++ {
						select {
						case <-ht.stopChan:
							return
						default:
						}
						_ = ht.CheckBlock(h)
						time.Sleep(250 * time.Millisecond) // polite rate limit
					}

					ht.mu.Lock()
					ht.lastScannedHeight = endH
					ht.mu.Unlock()
				}
			}

			// Jitter interval between 15s and 30s
			jitterSec := 15 + time.Now().UnixNano()%15
			timer := time.NewTimer(time.Duration(jitterSec) * time.Second)
			select {
			case <-ht.stopChan:
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (ht *HarvesterTracker) TriggerCheck(cm *ChainMonitor) {
	if !ht.isChecking.CompareAndSwap(false, true) {
		return // check already in progress
	}
	ht.wg.Add(1)
	go func() {
		defer ht.wg.Done()
		defer ht.isChecking.Store(false)

		select {
		case <-ht.stopChan:
			return
		default:
		}

		netHeight, err := cm.GetNetworkHeight()
		if err != nil || netHeight <= 0 {
			return
		}
		for h := netHeight - 2; h <= netHeight; h++ {
			select {
			case <-ht.stopChan:
				return
			default:
			}
			if h > 0 {
				_ = ht.CheckBlock(h)
			}
		}
	}()
}

func (ht *HarvesterTracker) Stop() {
	ht.stopOnce.Do(func() {
		close(ht.stopChan)
		if ht.networkValidators != nil {
			ht.networkValidators.Stop()
		}
	})
	ht.wg.Wait()
}
