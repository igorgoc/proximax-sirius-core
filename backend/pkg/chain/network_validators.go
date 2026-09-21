package chain

import (
	"sort"
	"strings"
	"sync"
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
	PublicKey      string  `json:"publicKey"`
	ShortKey       string  `json:"shortKey"`
	BlocksCount    int     `json:"blocksCount"`
	SharePercent   float64 `json:"sharePercent"`
	LastSeenHeight int64   `json:"lastSeenHeight"`
	LastSeenTime   string  `json:"lastSeenTime"`
	IsSelf         bool    `json:"isSelf"`
}

type NetworkValidatorStats struct {
	ActiveValidators4h     int                      `json:"activeValidators4h"`
	ActiveValidators24h    int                      `json:"activeValidators24h"`
	EstimatedStakedPoolXPX float64                  `json:"estimatedStakedPoolXPX"`
	AvgBlockTimeSec        float64                  `json:"avgBlockTimeSec"`
	TotalNetworkFees4h     float64                  `json:"totalNetworkFees4h"`
	RecentBlocksCount      int                      `json:"recentBlocksCount"`
	TopValidators          []ActiveValidatorSummary `json:"topValidators"`
}

type NetworkValidatorTracker struct {
	mu           sync.RWMutex
	recentBlocks []NetworkBlockInfo
	maxBlocks    int
}

func NewNetworkValidatorTracker() *NetworkValidatorTracker {
	return &NetworkValidatorTracker{
		recentBlocks: make([]NetworkBlockInfo, 0, 1000),
		maxBlocks:    1000, // ~4.1 hours of blocks at 15s cadence
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

	// In Sirius Mainnet PoS+, active committee validators stake ~60M XPX total (~6M XPX avg / active validator)
	estimatedPoolXPX := float64(active4h) * 6000000.0
	if active4h >= 10 {
		estimatedPoolXPX = 60000000.0
	}

	// Build Top Validators list
	topList := make([]ActiveValidatorSummary, 0, len(signers4h))
	for signer, count := range signers4h {
		shortKey := signer
		if len(shortKey) > 12 {
			shortKey = shortKey[:6] + "..." + shortKey[len(shortKey)-4:]
		}

		share := 0.0
		if count4h > 0 {
			share = (float64(count) / float64(count4h)) * 100.0
		}

		lastTimeStr := "Recently"
		if t, ok := signerLastTime[signer]; ok && !t.IsZero() {
			lastTimeStr = t.UTC().Format("2006-01-02 15:04:05 UTC")
		}

		topList = append(topList, ActiveValidatorSummary{
			PublicKey:      signer,
			ShortKey:       shortKey,
			BlocksCount:    count,
			SharePercent:   share,
			LastSeenHeight: signerLastHeight[signer],
			LastSeenTime:   lastTimeStr,
			IsSelf:         signer == selfClean,
		})
	}

	sort.Slice(topList, func(i, j int) bool {
		return topList[i].BlocksCount > topList[j].BlocksCount
	})

	// Limit to top 8 active signers
	if len(topList) > 8 {
		topList = topList[:8]
	}

	return NetworkValidatorStats{
		ActiveValidators4h:     active4h,
		ActiveValidators24h:    active24h,
		EstimatedStakedPoolXPX: estimatedPoolXPX,
		AvgBlockTimeSec:        avgBlockTime,
		TotalNetworkFees4h:     totalFees4h,
		RecentBlocksCount:      len(nvt.recentBlocks),
		TopValidators:          topList,
	}
}
