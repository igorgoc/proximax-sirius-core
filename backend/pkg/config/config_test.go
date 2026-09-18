package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_BootKeySourceDefault(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	resourcesDir := filepath.Join(tempDir, "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		t.Fatalf("failed to create resources dir: %v", err)
	}

	harvestKey := "1111111111111111111111111111111111111111111111111111111111111111"
	bootKey := "3333333333333333333333333333333333333333333333333333333333333333"
	harvestContent := "[harvesting]\nharvestKey = " + harvestKey + "\nbeneficiary = 0000000000000000000000000000000000000000000000000000000000000000\nisAutoHarvestingEnabled = true\n"
	if err := os.WriteFile(filepath.Join(resourcesDir, "config-harvesting.properties"), []byte(harvestContent), 0644); err != nil {
		t.Fatalf("failed to write harvest props: %v", err)
	}

	userContent := "[account]\nbootKey = " + bootKey + "\n"
	if err := os.WriteFile(filepath.Join(resourcesDir, "config-user.properties"), []byte(userContent), 0644); err != nil {
		t.Fatalf("failed to write user props: %v", err)
	}

	nodeContent := "[node]\nfriendlyName = test-node\nhost = 127.0.0.1\n"
	if err := os.WriteFile(filepath.Join(resourcesDir, "config-node.properties"), []byte(nodeContent), 0644); err != nil {
		t.Fatalf("failed to write node props: %v", err)
	}

	cm := NewConfigManager(tempDir)
	cfg, err := cm.LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig failed: %v", err)
	}

	if cfg.HarvestKey != harvestKey {
		t.Errorf("expected HarvestKey '%s', got '%s'", harvestKey, cfg.HarvestKey)
	}
	if cfg.BootKey != bootKey {
		t.Errorf("expected BootKey '%s', got '%s'", bootKey, cfg.BootKey)
	}
}

func TestConfigManager_BootKeySourceGenerated(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	resourcesDir := filepath.Join(tempDir, "resources")
	_ = os.MkdirAll(resourcesDir, 0755)

	harvestKey := "1111111111111111111111111111111111111111111111111111111111111111"
	bootKey := "2222222222222222222222222222222222222222222222222222222222222222"

	userContent := "[account]\nbootKey = " + bootKey + "\nbootkey.source = generated\n[storage]\ndata.path = /custom/data\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-user.properties"), []byte(userContent), 0644)

	harvestContent := "[harvesting]\nharvestKey = " + harvestKey + "\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-harvesting.properties"), []byte(harvestContent), 0644)

	cm := NewConfigManager(tempDir)
	cfg, err := cm.LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig failed: %v", err)
	}

	if cfg.BootKeySource != "generated" {
		t.Errorf("expected BootKeySource 'generated', got '%s'", cfg.BootKeySource)
	}
	if cfg.BootKey != bootKey {
		t.Errorf("expected BootKey '%s', got '%s'", bootKey, cfg.BootKey)
	}
	if cfg.DataPath != "/custom/data" {
		t.Errorf("expected DataPath '/custom/data', got '%s'", cfg.DataPath)
	}
}

func TestConfigManager_DataMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_migrate_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "old_data")
	dstDir := filepath.Join(tempDir, "new_data")

	_ = os.MkdirAll(filepath.Join(srcDir, "00000"), 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "00000", "00001.dat"), []byte("nemesis block test data"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "index.dat"), []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0644)

	cm := NewConfigManager(tempDir)
	if err := cm.MigrateDataDirectory(srcDir, dstDir); err != nil {
		t.Fatalf("MigrateDataDirectory failed: %v", err)
	}

	dstBlock := filepath.Join(dstDir, "00000", "00001.dat")
	if data, err := os.ReadFile(dstBlock); err != nil || string(data) != "nemesis block test data" {
		t.Errorf("expected block file in destination: %v", err)
	}

	dstIndex := filepath.Join(dstDir, "index.dat")
	if _, err := os.Stat(dstIndex); err != nil {
		t.Errorf("expected index.dat in destination: %v", err)
	}
}

func TestConfigManager_IsConfigured_RequiresHarvestKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_isconfigured_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	resourcesDir := filepath.Join(tempDir, "resources")
	_ = os.MkdirAll(resourcesDir, 0755)

	bootKey := "3333333333333333333333333333333333333333333333333333333333333333"
	nodeContent := "[node]\nfriendlyName = test-node\nhost = 127.0.0.1\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-node.properties"), []byte(nodeContent), 0644)
	userContent := "[account]\nbootKey = " + bootKey + "\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-user.properties"), []byte(userContent), 0644)

	// Case 1: harvestKey is placeholder REMOTE_ACCOUNT_PRIVATE_KEY
	placeholderHarvest := "[harvesting]\nharvestKey = REMOTE_ACCOUNT_PRIVATE_KEY\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-harvesting.properties"), []byte(placeholderHarvest), 0644)

	cm := NewConfigManager(tempDir)
	cfg, err := cm.LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig failed: %v", err)
	}
	if cfg.HasHarvestKey {
		t.Errorf("expected HasHarvestKey=false for placeholder harvest key")
	}
	if cfg.IsConfigured {
		t.Errorf("expected IsConfigured=false when harvestKey is missing/placeholder")
	}

	// Case 2: valid 64-hex harvestKey
	validHarvest := "[harvesting]\nharvestKey = 1111111111111111111111111111111111111111111111111111111111111111\n"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-harvesting.properties"), []byte(validHarvest), 0644)

	cfg, err = cm.LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig failed: %v", err)
	}
	if !cfg.HasHarvestKey {
		t.Errorf("expected HasHarvestKey=true for valid 64-hex key")
	}
	if !cfg.IsConfigured {
		t.Errorf("expected IsConfigured=true when all keys are valid and friendlyName is set")
	}
}

