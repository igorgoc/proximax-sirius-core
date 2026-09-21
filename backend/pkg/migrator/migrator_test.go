package migrator

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrator_EmptyDataDir(t *testing.T) {
	tempDir := t.TempDir()
	m := New()

	if status := m.GetStatus(); status.Status != "idle" {
		t.Fatalf("expected idle status, got %s", status.Status)
	}

	err := m.StartMigration(tempDir)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}

	// Wait for completion with polling
	for i := 0; i < 50; i++ {
		st := m.GetStatus()
		if st.Status == "completed" || st.Status == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	status := m.GetStatus()
	if status.Status != "completed" {
		t.Fatalf("expected completed status, got %s (err: %s)", status.Status, status.Error)
	}
	if status.TotalDirs != 0 {
		t.Fatalf("expected 0 total dirs, got %d", status.TotalDirs)
	}
}

func TestMigrator_ProcessDirectory(t *testing.T) {
	tempDir := t.TempDir()
	chunkDir := filepath.Join(tempDir, "00000")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("failed to create chunk dir: %v", err)
	}

	// Create loose dat and stmt files
	blockPayload := []byte("dummy-block-data-height-1")
	stmtPayload := []byte("dummy-statement-data-height-1")

	if err := os.WriteFile(filepath.Join(chunkDir, "00001.dat"), blockPayload, 0644); err != nil {
		t.Fatalf("failed to write 00001.dat: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "00001.stmt"), stmtPayload, 0644); err != nil {
		t.Fatalf("failed to write 00001.stmt: %v", err)
	}

	m := New()
	err := m.StartMigration(tempDir)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}

	// Wait for migration
	for i := 0; i < 20; i++ {
		st := m.GetStatus()
		if st.Status == "completed" || st.Status == "failed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	status := m.GetStatus()
	if status.Status != "completed" {
		t.Fatalf("expected completed status, got %s (err: %s)", status.Status, status.Error)
	}
	if status.ConvertedBlocks != 1 {
		t.Fatalf("expected 1 converted block, got %d", status.ConvertedBlocks)
	}
	if status.DeletedFiles != 2 {
		t.Fatalf("expected 2 deleted files, got %d", status.DeletedFiles)
	}

	// Verify loose files are deleted
	if _, err := os.Stat(filepath.Join(chunkDir, "00001.dat")); !os.IsNotExist(err) {
		t.Fatalf("expected 00001.dat to be deleted")
	}
	if _, err := os.Stat(filepath.Join(chunkDir, "00001.stmt")); !os.IsNotExist(err) {
		t.Fatalf("expected 00001.stmt to be deleted")
	}

	// Verify blocks.dat, statements.dat, blocks.idx exist
	blocksData, err := os.ReadFile(filepath.Join(chunkDir, "blocks.dat"))
	if err != nil || string(blocksData) != string(blockPayload) {
		t.Fatalf("blocks.dat content mismatch: %v", err)
	}

	stmtData, err := os.ReadFile(filepath.Join(chunkDir, "statements.dat"))
	if err != nil || string(stmtData) != string(stmtPayload) {
		t.Fatalf("statements.dat content mismatch: %v", err)
	}

	idxData, err := os.ReadFile(filepath.Join(chunkDir, "blocks.idx"))
	if err != nil {
		t.Fatalf("failed to read blocks.idx: %v", err)
	}
	// Index for height 1 is at offset 1 * 16 = 16
	if len(idxData) < 32 {
		t.Fatalf("blocks.idx size too small: %d", len(idxData))
	}

	blockOffset := binary.LittleEndian.Uint32(idxData[16:20])
	blockSize := binary.LittleEndian.Uint32(idxData[20:24])
	stmtOffset := binary.LittleEndian.Uint32(idxData[24:28])
	stmtSize := binary.LittleEndian.Uint32(idxData[28:32])

	if blockOffset != 0 || blockSize != uint32(len(blockPayload)) {
		t.Fatalf("invalid block index: offset=%d, size=%d", blockOffset, blockSize)
	}
	if stmtOffset != 0 || stmtSize != uint32(len(stmtPayload)) {
		t.Fatalf("invalid stmt index: offset=%d, size=%d", stmtOffset, stmtSize)
	}
}

func TestMigrator_Cancel(t *testing.T) {
	m := New()
	m.Cancel() // Cancel while idle should not panic

	tempDir := t.TempDir()
	chunkDir := filepath.Join(tempDir, "00000")
	_ = os.MkdirAll(chunkDir, 0755)

	_ = m.StartMigration(tempDir)
	m.Cancel()

	for i := 0; i < 50; i++ {
		st := m.GetStatus()
		if st.Status == "cancelled" || st.Status == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	status := m.GetStatus()
	if status.Status != "cancelled" && status.Status != "completed" {
		t.Fatalf("expected cancelled or completed status, got %s", status.Status)
	}
}
