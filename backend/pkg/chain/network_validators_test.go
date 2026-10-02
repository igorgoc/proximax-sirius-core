package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNetworkValidatorTracker_RecordBlockAndOrdering(t *testing.T) {
	nvt := NewNetworkValidatorTracker()
	nvt.maxBlocks = 5

	now := time.Now().UTC()

	// Insert blocks out of order
	nvt.RecordBlock(NetworkBlockInfo{Height: 10, Signer: "A", Timestamp: now})
	nvt.RecordBlock(NetworkBlockInfo{Height: 8, Signer: "B", Timestamp: now.Add(-30 * time.Second)})
	nvt.RecordBlock(NetworkBlockInfo{Height: 10, Signer: "A-duplicate", Timestamp: now}) // duplicate should be ignored
	nvt.RecordBlock(NetworkBlockInfo{Height: 9, Signer: "C", Timestamp: now.Add(-15 * time.Second)})

	if len(nvt.recentBlocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(nvt.recentBlocks))
	}

	// Verify descending order: 10, 9, 8
	expectedHeights := []int64{10, 9, 8}
	for i, exp := range expectedHeights {
		if nvt.recentBlocks[i].Height != exp {
			t.Errorf("at index %d expected height %d, got %d", i, exp, nvt.recentBlocks[i].Height)
		}
	}

	// Insert more blocks to test ring buffer truncation (maxBlocks = 5)
	nvt.RecordBlock(NetworkBlockInfo{Height: 12, Signer: "D", Timestamp: now.Add(30 * time.Second)})
	nvt.RecordBlock(NetworkBlockInfo{Height: 11, Signer: "E", Timestamp: now.Add(15 * time.Second)})
	nvt.RecordBlock(NetworkBlockInfo{Height: 7, Signer: "F", Timestamp: now.Add(-45 * time.Second)})

	if len(nvt.recentBlocks) != 5 {
		t.Fatalf("expected maxBlocks limit of 5, got %d", len(nvt.recentBlocks))
	}

	expectedAfterTrim := []int64{12, 11, 10, 9, 8}
	for i, exp := range expectedAfterTrim {
		if nvt.recentBlocks[i].Height != exp {
			t.Errorf("at index %d after trim expected %d, got %d", i, exp, nvt.recentBlocks[i].Height)
		}
	}
}

func TestNetworkValidatorTracker_ActiveValidatorsAndCadence(t *testing.T) {
	nvt := NewNetworkValidatorTracker()
	defer nvt.Stop()
	baseTime := time.Now().UTC()

	// Insert 10 blocks spaced exactly 15 seconds apart: 5 by "VAL_ALPHA", 5 by "VAL_BETA"
	for i := 0; i < 10; i++ {
		signer := "VAL_ALPHA"
		if i%2 == 1 {
			signer = "VAL_BETA"
		}
		nvt.RecordBlock(NetworkBlockInfo{
			Height:    int64(100 - i),
			Signer:    signer,
			Timestamp: baseTime.Add(-time.Duration(i*15) * time.Second),
			FeeXPX:    0.25,
		})
	}

	stats := nvt.GetStats("VAL_ALPHA")

	if stats.ActiveValidators4h != 2 {
		t.Errorf("expected 2 active validators in 4h, got %d", stats.ActiveValidators4h)
	}
	if stats.AvgBlockTimeSec < 14.0 || stats.AvgBlockTimeSec > 16.0 {
		t.Errorf("expected avg block time ~15s, got %f", stats.AvgBlockTimeSec)
	}
	if stats.TotalNetworkFees4h != 2.5 {
		t.Errorf("expected total fees 2.5 XPX, got %f", stats.TotalNetworkFees4h)
	}
	if len(stats.TopValidators) != 2 {
		t.Fatalf("expected 2 top validators, got %d", len(stats.TopValidators))
	}

	for _, v := range stats.TopValidators {
		if v.BlocksCount != 5 {
			t.Errorf("expected 5 blocks for %s, got %d", v.PublicKey, v.BlocksCount)
		}
		if v.SharePercent != 50.0 {
			t.Errorf("expected 50%% share for %s, got %f", v.PublicKey, v.SharePercent)
		}
		if v.PublicKey == "VAL_ALPHA" && !v.IsSelf {
			t.Errorf("expected VAL_ALPHA to have IsSelf=true")
		}
	}
}

func TestNetworkValidatorTracker_MosaicPriority(t *testing.T) {
	validatorKey := "AAAAAAAAAABBBBBBBBBBCCCCCCCCCCDDDDDDDDDDEEEEEEEEEEFFFFFFFFFF1111"

	// Mock server returning multiple mosaics: a custom high-amount token first, and XPX second
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"account": map[string]interface{}{
				"publicKey":   validatorKey,
				"accountType": 0, // Direct harvester
				"mosaics": []interface{}{
					map[string]interface{}{
						"id":     []interface{}{float64(999999), float64(888888)},             // Custom random token
						"amount": []interface{}{float64(999999999), float64(0)},              // Huge balance
					},
					map[string]interface{}{
						"id":     []interface{}{float64(2679028825), float64(1076571991)},     // XPX Mosaic ID
						"amount": []interface{}{float64(5000000000000 % (1 << 32)), float64(5000000000000 >> 32)}, // 5,000,000 XPX
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	nvt := NewNetworkValidatorTracker(mockServer.URL)
	defer nvt.Stop()
	balance, err := nvt.resolveStakedBalance(validatorKey)
	if err != nil {
		t.Fatalf("resolveStakedBalance failed: %v", err)
	}

	// Should match the XPX mosaic (5,000,000 XPX), NOT the custom token (999,999,999)
	if balance != 5000000.0 {
		t.Errorf("expected XPX mosaic balance 5000000.0, got %f", balance)
	}
}

func TestNetworkValidatorTracker_SelfStakedBalanceOverride(t *testing.T) {
	nvt := NewNetworkValidatorTracker()
	defer nvt.Stop()
	selfKey := "1D339BA5E197D7AB2E4BFA9312B5C115040740F9F00C5E3BD7EA6F911B5827F2"

	nvt.RecordBlock(NetworkBlockInfo{
		Height:    50,
		Signer:    selfKey,
		Timestamp: time.Now(),
		FeeXPX:    1.0,
	})

	// Before setting self balance, set directly
	nvt.SetSelfStakedBalance(selfKey, 7727964.0)

	stats := nvt.GetStats(selfKey)
	if len(stats.TopValidators) == 0 {
		t.Fatalf("expected top validators")
	}

	v := stats.TopValidators[0]
	if !v.IsSelf {
		t.Errorf("expected IsSelf to be true")
	}
	if v.StakedBalanceXPX != 7727964.0 {
		t.Errorf("expected staked balance 7727964.0, got %f", v.StakedBalanceXPX)
	}

	// Attempting to set negative/zero balance should not override
	nvt.SetSelfStakedBalance(selfKey, 0)
	stats2 := nvt.GetStats(selfKey)
	if stats2.TopValidators[0].StakedBalanceXPX != 7727964.0 {
		t.Errorf("expected staked balance to remain 7727964.0, got %f", stats2.TopValidators[0].StakedBalanceXPX)
	}
}

func TestNetworkValidatorTracker_ConcurrentAccess(t *testing.T) {
	nvt := NewNetworkValidatorTracker()
	defer nvt.Stop()
	nvt.maxBlocks = 200

	signers := []string{"NODE_A", "NODE_B", "NODE_C", "NODE_D"}
	now := time.Now()

	// Pre-seed cache to avoid network calls during concurrent test
	for _, s := range signers {
		nvt.balanceCache[s] = validatorBalanceEntry{
			stakedXPX: 1000000.0,
			fetchedAt: now,
		}
	}

	var wg sync.WaitGroup

	// Concurrently record 200 blocks
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				h := int64(1000 + workerId*100 + j)
				nvt.RecordBlock(NetworkBlockInfo{
					Height:    h,
					Signer:    signers[(workerId+j)%len(signers)],
					Timestamp: now.Add(time.Duration(j) * time.Second),
					FeeXPX:    0.1,
				})
			}
		}(i)
	}

	// Concurrently query stats
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = nvt.GetStats(signers[workerId%len(signers)])
			}
		}(i)
	}

	wg.Wait()

	stats := nvt.GetStats("NODE_A")
	if stats.RecentBlocksCount == 0 {
		t.Errorf("expected recorded blocks after concurrency test")
	}
}
