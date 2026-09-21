package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageManager_ConfigAndKeyMasking(t *testing.T) {
	tempDir := t.TempDir()
	resourcesDir := filepath.Join(tempDir, "resources")
	dataDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(resourcesDir, 0755)
	_ = os.MkdirAll(dataDir, 0755)

	sm := NewStorageManager(
		resourcesDir,
		func() string { return dataDir },
		func() string { return "1111111111111111111111111111111111111111111111111111111111111111" },
	)

	// Valid 64-character dummy private key
	testKey := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	customStoragePath := filepath.Join(tempDir, "custom-drives")

	err := sm.SaveStorageConfig(testKey, "127.0.0.1", customStoragePath)
	if err != nil {
		t.Fatalf("SaveStorageConfig failed: %v", err)
	}

	// Load directly
	cfg, err := sm.LoadStorageConfig()
	if err != nil {
		t.Fatalf("LoadStorageConfig failed: %v", err)
	}

	if cfg.Key != testKey {
		t.Fatalf("expected key %s, got %s", testKey, cfg.Key)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.StoragePath != customStoragePath {
		t.Fatalf("expected storage path %s, got %s", customStoragePath, cfg.StoragePath)
	}
	if !cfg.HasKey || !cfg.IsConfigured {
		t.Fatalf("expected HasKey and IsConfigured to be true")
	}

	// Invariant: GetStatus() MUST NEVER return raw private keys
	status := sm.GetStatus()
	if status.Config.Key != "" {
		t.Fatalf("CRITICAL SECURITY INVARIANT VIOLATION: GetStatus() exposed raw private key: %s", status.Config.Key)
	}
	if status.Config.PublicKey == "" {
		t.Fatalf("expected derived public key in status")
	}
}

func TestStorageManager_MetricsAndSandboxes(t *testing.T) {
	tempDir := t.TempDir()
	resourcesDir := filepath.Join(tempDir, "resources")
	dataDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(resourcesDir, 0755)

	sm := NewStorageManager(
		resourcesDir,
		func() string { return dataDir },
		nil,
	)

	drivesPath := sm.GetActiveStoragePath()
	sandboxPath := filepath.Join(drivesPath, "drive-sandboxes")
	_ = os.MkdirAll(sandboxPath, 0755)

	// Write mock shard file in drives
	shardData := make([]byte, 1024*10) // 10 KB
	if err := os.WriteFile(filepath.Join(drivesPath, "shard_1.bin"), shardData, 0644); err != nil {
		t.Fatalf("failed to write shard: %v", err)
	}

	// Write mock sandbox file
	sandboxData := make([]byte, 1024*5) // 5 KB
	if err := os.WriteFile(filepath.Join(sandboxPath, "temp_sandbox.bin"), sandboxData, 0644); err != nil {
		t.Fatalf("failed to write sandbox file: %v", err)
	}

	metrics := sm.GetStorageMetrics()
	if metrics.DriveSizeBytes != int64(len(shardData)) {
		t.Fatalf("expected %d drive bytes, got %d", len(shardData), metrics.DriveSizeBytes)
	}
	if metrics.SandboxSizeBytes != int64(len(sandboxData)) {
		t.Fatalf("expected %d sandbox bytes, got %d", len(sandboxData), metrics.SandboxSizeBytes)
	}
	if metrics.TotalShardsCount != 1 {
		t.Fatalf("expected 1 shard count, got %d", metrics.TotalShardsCount)
	}

	// Test CleanSandboxes
	freed, err := sm.CleanSandboxes()
	if err != nil {
		t.Fatalf("CleanSandboxes failed: %v", err)
	}
	if freed != int64(len(sandboxData)) {
		t.Fatalf("expected %d freed bytes, got %d", len(sandboxData), freed)
	}

	// Verify metrics after clean
	metricsAfter := sm.GetStorageMetrics()
	if metricsAfter.SandboxSizeBytes != 0 {
		t.Fatalf("expected 0 sandbox bytes after clean, got %d", metricsAfter.SandboxSizeBytes)
	}
	if metricsAfter.DriveSizeBytes != int64(len(shardData)) {
		t.Fatalf("drive bytes should remain intact")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024 * 5, "5.00 MB"},
		{1024 * 1024 * 1024 * 2, "2.00 GB"},
	}

	for _, tt := range tests {
		res := formatBytes(tt.bytes)
		if res != tt.expected {
			t.Errorf("formatBytes(%d): expected %s, got %s", tt.bytes, tt.expected, res)
		}
	}
}
