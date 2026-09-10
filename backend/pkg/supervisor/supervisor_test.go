package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessSupervisor_SnapshotStatusAndCancel(t *testing.T) {
	s := NewProcessSupervisor("/tmp/mock_chainconfig")

	status := s.GetSnapshotStatus()
	if status.Stage != SnapshotStageIdle {
		t.Errorf("expected initial stage 'idle', got '%s'", status.Stage)
	}

	if err := s.CancelSnapshot(); err != nil {
		t.Fatalf("CancelSnapshot failed: %v", err)
	}

	statusAfterCancel := s.GetSnapshotStatus()
	if statusAfterCancel.Stage != SnapshotStageCancelled {
		t.Errorf("expected stage 'cancelled', got '%s'", statusAfterCancel.Stage)
	}
}

func TestProcessSupervisor_ResetChain_FallbackAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "chainconfig")
	dataDir := filepath.Join(tmpDir, "custom_data")

	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(dataDir, 0755)

	s := NewProcessSupervisor(configDir)

	// 1. Reset with empty data and empty config (should download from GitHub fallback)
	if err := s.ResetChain(dataDir); err != nil {
		t.Fatalf("ResetChain failed on empty dir: %v", err)
	}

	datPath := filepath.Join(dataDir, "00000", "00001.dat")
	hashesPath := filepath.Join(dataDir, "00000", "hashes.dat")
	indexPath := filepath.Join(dataDir, "index.dat")

	fiDat, err := os.Stat(datPath)
	if err != nil || fiDat.Size() == 0 {
		t.Fatalf("expected valid 00001.dat, got stat err: %v, size: %d", err, fiDat.Size())
	}

	fiHashes, err := os.Stat(hashesPath)
	if err != nil || fiHashes.Size() == 0 {
		t.Fatalf("expected valid hashes.dat, got stat err: %v, size: %d", err, fiHashes.Size())
	}

	indexBytes, err := os.ReadFile(indexPath)
	if err != nil || len(indexBytes) != 8 || indexBytes[0] != 0x01 {
		t.Fatalf("expected 8-byte index.dat starting with 0x01, got: %v (err: %v)", indexBytes, err)
	}

	// 2. Add extra junk directories/files to simulate synced state
	junkDir := filepath.Join(dataDir, "00002")
	_ = os.MkdirAll(junkDir, 0755)
	_ = os.WriteFile(filepath.Join(junkDir, "00002.dat"), []byte("junk"), 0644)
	_ = os.WriteFile(filepath.Join(dataDir, "server.lock"), []byte("locked"), 0644)

	// 3. Reset again (should backup existing local nemesis and delete junk/locks)
	if err := s.ResetChain(dataDir); err != nil {
		t.Fatalf("ResetChain failed on existing local nemesis: %v", err)
	}

	if _, err := os.Stat(junkDir); !os.IsNotExist(err) {
		t.Errorf("expected junk dir 00002 to be removed, but it exists")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "server.lock")); !os.IsNotExist(err) {
		t.Errorf("expected server.lock to be removed, but it exists")
	}

	if !hasValidNemesis(dataDir) {
		t.Errorf("expected valid nemesis in dataDir after reset")
	}
}

func TestProcessSupervisor_DataBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "chainconfig")
	dataDir := filepath.Join(tmpDir, "data")
	backupOutDir := filepath.Join(tmpDir, "backups")

	_ = os.MkdirAll(filepath.Join(dataDir, "00000"), 0755)
	_ = os.WriteFile(filepath.Join(dataDir, "00000", "00001.dat"), []byte("test-data-block"), 0644)
	_ = os.WriteFile(filepath.Join(dataDir, "index.dat"), []byte{0x01, 0x00, 0x00, 0x00}, 0644)

	s := NewProcessSupervisor(configDir)

	// Test 1: Zstd format
	if err := s.CreateDataBackup(dataDir, backupOutDir, "zst"); err != nil {
		t.Fatalf("CreateDataBackup (zst) failed: %v", err)
	}

	var status DataBackupStatus
	for i := 0; i < 50; i++ {
		status = s.GetDataBackupStatus()
		if status.Stage == DataBackupStageComplete || status.Stage == DataBackupStageError {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if status.Stage != DataBackupStageComplete {
		t.Fatalf("expected stage 'complete' for zst, got '%s' (err: %s)", status.Stage, status.ErrorMessage)
	}

	if !strings.HasSuffix(status.TargetFile, ".tar.zst") {
		t.Fatalf("expected .tar.zst extension, got %s", status.TargetFile)
	}

	if _, err := os.Stat(status.TargetFile); err != nil {
		t.Fatalf("zst backup archive was not created: %v", err)
	}

	// Test 2: Gzip format
	s2 := NewProcessSupervisor(configDir)
	if err := s2.CreateDataBackup(dataDir, backupOutDir, "gz"); err != nil {
		t.Fatalf("CreateDataBackup (gz) failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		status = s2.GetDataBackupStatus()
		if status.Stage == DataBackupStageComplete || status.Stage == DataBackupStageError {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if status.Stage != DataBackupStageComplete {
		t.Fatalf("expected stage 'complete' for gz, got '%s' (err: %s)", status.Stage, status.ErrorMessage)
	}

	if !strings.HasSuffix(status.TargetFile, ".tar.gz") {
		t.Fatalf("expected .tar.gz extension, got %s", status.TargetFile)
	}
}

func TestProcessSupervisor_CancelDataBackup(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewProcessSupervisor(filepath.Join(tmpDir, "chainconfig"))

	status := s.GetDataBackupStatus()
	if status.Stage != DataBackupStageIdle {
		t.Errorf("expected initial stage 'idle', got '%s'", status.Stage)
	}

	if err := s.CancelDataBackup(); err != nil {
		t.Fatalf("CancelDataBackup failed: %v", err)
	}

	statusAfter := s.GetDataBackupStatus()
	if statusAfter.Stage != DataBackupStageCancelled {
		t.Errorf("expected stage 'cancelled', got '%s'", statusAfter.Stage)
	}
}

func TestProcessSupervisor_DataIntegrityPreflight(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0755)

	s := NewProcessSupervisor(filepath.Join(tmpDir, "chainconfig"))

	// 1. Corrupted index.dat (< 8 bytes)
	indexPath := filepath.Join(dataDir, "index.dat")
	_ = os.WriteFile(indexPath, []byte{1, 2, 3}, 0644)
	err := s.checkDataIntegrityPreflight(dataDir)
	if err == nil || !strings.Contains(err.Error(), "index.dat is truncated or corrupted") {
		t.Fatalf("expected truncated index.dat error, got: %v", err)
	}

	// 2. index.dat with height 0
	_ = os.WriteFile(indexPath, make([]byte, 8), 0644)
	err = s.checkDataIntegrityPreflight(dataDir)
	if err == nil || !strings.Contains(err.Error(), "invalid block height 0") {
		t.Fatalf("expected invalid block height 0 error, got: %v", err)
	}

	// 3. index.dat with height > 1 but statedb missing
	validHeightBytes := []byte{0x20, 0x4e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00} // height 20000
	_ = os.WriteFile(indexPath, validHeightBytes, 0644)
	err = s.checkDataIntegrityPreflight(dataDir)
	if err == nil || !strings.Contains(err.Error(), "statedb directory is missing") {
		t.Fatalf("expected missing statedb error, got: %v", err)
	}

	// 4. statedb exists but is empty
	statedbDir := filepath.Join(dataDir, "statedb")
	_ = os.MkdirAll(statedbDir, 0755)
	err = s.checkDataIntegrityPreflight(dataDir)
	if err == nil || !strings.Contains(err.Error(), "statedb directory is completely empty") {
		t.Fatalf("expected empty statedb error, got: %v", err)
	}

	// Add mock SST file into statedb
	_ = os.WriteFile(filepath.Join(statedbDir, "000001.sst"), []byte("sst-data"), 0644)

	// 5. Tail block directory with 0-byte blocks.dat
	tailDir := filepath.Join(dataDir, "00050")
	_ = os.MkdirAll(tailDir, 0755)
	_ = os.WriteFile(filepath.Join(tailDir, "blocks.dat"), []byte{}, 0644)
	err = s.checkDataIntegrityPreflight(dataDir)
	if err == nil || !strings.Contains(err.Error(), "tail block file 00050/blocks.dat is 0 bytes") {
		t.Fatalf("expected 0-byte tail block error, got: %v", err)
	}

	// 6. Valid non-zero blocks.dat and hashes.dat
	_ = os.WriteFile(filepath.Join(tailDir, "blocks.dat"), []byte("block-data-content"), 0644)
	_ = os.WriteFile(filepath.Join(tailDir, "hashes.dat"), []byte("hash-data-content"), 0644)

	err = s.checkDataIntegrityPreflight(dataDir)
	if err != nil {
		t.Fatalf("expected preflight to pass for valid data, got error: %v", err)
	}
}

func TestProcessSupervisor_CleanLogsAndStats(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "chainconfig")
	logsDir := filepath.Join(configDir, "logs")
	dataDir := filepath.Join(tmpDir, "data")

	_ = os.MkdirAll(logsDir, 0755)
	_ = os.MkdirAll(dataDir, 0755)

	// Create sample log files
	_ = os.WriteFile(filepath.Join(logsDir, "server_0001.log"), []byte("sample-log-content-1"), 0644)
	_ = os.WriteFile(filepath.Join(logsDir, "recovery_0000.log"), []byte("sample-log-content-2"), 0644)
	_ = os.WriteFile(filepath.Join(logsDir, "manager.log"), []byte("manager-log-content"), 0644)
	_ = os.WriteFile(filepath.Join(dataDir, "server.lock"), []byte("locked"), 0644)

	s := NewProcessSupervisor(configDir)

	// Verify stats before cleaning
	stats := s.GetLogStats(dataDir)
	if stats.LogCount != 3 {
		t.Errorf("expected 3 log files, got %d", stats.LogCount)
	}
	if !stats.ServerLockFound {
		t.Errorf("expected server.lock to be found")
	}
	if stats.TotalBytes <= 0 {
		t.Errorf("expected positive total log bytes, got %d", stats.TotalBytes)
	}

	// Clean logs and cache
	freedBytes, err := s.CleanLogsAndCache(dataDir)
	if err != nil {
		t.Fatalf("CleanLogsAndCache failed: %v", err)
	}
	if freedBytes <= 0 {
		t.Errorf("expected positive freed bytes, got %d", freedBytes)
	}

	// Verify stale server.lock was removed
	if _, err := os.Stat(filepath.Join(dataDir, "server.lock")); !os.IsNotExist(err) {
		t.Errorf("expected server.lock to be removed, but it still exists")
	}

	// Verify rotated logs removed
	if _, err := os.Stat(filepath.Join(logsDir, "server_0001.log")); !os.IsNotExist(err) {
		t.Errorf("expected server_0001.log to be removed")
	}
	if _, err := os.Stat(filepath.Join(logsDir, "recovery_0000.log")); !os.IsNotExist(err) {
		t.Errorf("expected recovery_0000.log to be removed")
	}

	// Verify manager.log was truncated to 0
	info, err := os.Stat(filepath.Join(logsDir, "manager.log"))
	if err != nil {
		t.Fatalf("manager.log should exist (truncated), got error: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected manager.log to be truncated to 0 bytes, got %d", info.Size())
	}

	// Verify updated stats
	afterStats := s.GetLogStats(dataDir)
	if afterStats.ServerLockFound {
		t.Errorf("expected serverLockFound to be false after cleanup")
	}
	if afterStats.LastPurgeTime == "" {
		t.Errorf("expected LastPurgeTime to be set")
	}
}


