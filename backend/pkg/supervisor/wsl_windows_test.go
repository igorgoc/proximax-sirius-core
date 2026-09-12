//go:build windows

package supervisor

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessSupervisor_ReconcileChainStateIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)

	s := NewProcessSupervisor(filepath.Join(tmpDir, "chainconfig"))

	// 1. Setup commit_step.dat = 1
	commitStepPath := filepath.Join(dataDir, "commit_step.dat")
	commitStepBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(commitStepBytes, 1)
	if err := os.WriteFile(commitStepPath, commitStepBytes, 0644); err != nil {
		t.Fatalf("failed to write commit_step.dat: %v", err)
	}

	// 2. Setup index.dat = 13902205
	indexPath := filepath.Join(dataDir, "index.dat")
	storageHeight := uint64(13902205)
	indexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(indexBytes, storageHeight)
	if err := os.WriteFile(indexPath, indexBytes, 0644); err != nil {
		t.Fatalf("failed to write index.dat: %v", err)
	}

	// 3. Setup state/supplemental.dat = 13902206 (cache height ahead of storage height)
	suppPath := filepath.Join(stateDir, "supplemental.dat")
	cacheHeight := uint64(13902206)
	suppBytes := make([]byte, 40)
	binary.LittleEndian.PutUint64(suppBytes[32:40], cacheHeight)
	if err := os.WriteFile(suppPath, suppBytes, 0644); err != nil {
		t.Fatalf("failed to write supplemental.dat: %v", err)
	}

	// 4. Setup state/BlockDifficultyCache.dat
	// Header: [Height: uint64][Count: uint64] + Count * 28 bytes records
	diffPath := filepath.Join(stateDir, "BlockDifficultyCache.dat")
	initialCount := uint64(10)
	diffBytes := make([]byte, 16+initialCount*28)
	binary.LittleEndian.PutUint64(diffBytes[0:8], cacheHeight)
	binary.LittleEndian.PutUint64(diffBytes[8:16], initialCount)
	if err := os.WriteFile(diffPath, diffBytes, 0644); err != nil {
		t.Fatalf("failed to write BlockDifficultyCache.dat: %v", err)
	}

	// Run reconciliation
	s.reconcileChainStateIntegrity(dataDir)

	// Verify commit_step.dat was reset to 0
	csData, err := os.ReadFile(commitStepPath)
	if err != nil || len(csData) < 8 {
		t.Fatalf("failed to read commit_step.dat after reconcile: %v", err)
	}
	if val := binary.LittleEndian.Uint64(csData[:8]); val != 0 {
		t.Fatalf("expected commit_step.dat to be 0, got %d", val)
	}

	// Verify supplemental.dat height was aligned to storageHeight (13902205)
	suppDataAfter, err := os.ReadFile(suppPath)
	if err != nil || len(suppDataAfter) < 40 {
		t.Fatalf("failed to read supplemental.dat after reconcile: %v", err)
	}
	if val := binary.LittleEndian.Uint64(suppDataAfter[32:40]); val != storageHeight {
		t.Fatalf("expected supplemental.dat height %d, got %d", storageHeight, val)
	}

	// Verify BlockDifficultyCache.dat height and count
	diffDataAfter, err := os.ReadFile(diffPath)
	if err != nil || len(diffDataAfter) < 16 {
		t.Fatalf("failed to read BlockDifficultyCache.dat after reconcile: %v", err)
	}
	if val := binary.LittleEndian.Uint64(diffDataAfter[0:8]); val != storageHeight {
		t.Fatalf("expected diff height %d, got %d", storageHeight, val)
	}
	if count := binary.LittleEndian.Uint64(diffDataAfter[8:16]); count != initialCount-1 {
		t.Fatalf("expected diff count %d, got %d", initialCount-1, count)
	}
	expectedLen := int(16 + (initialCount-1)*28)
	if len(diffDataAfter) != expectedLen {
		t.Fatalf("expected diff file length %d, got %d", expectedLen, len(diffDataAfter))
	}
}
