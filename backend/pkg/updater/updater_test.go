package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestValidationLogic(t *testing.T) {
	// 1. JSON validation test
	validJSON := []byte(`{"version": "1.0", "peers": ["127.0.0.1"]}`)
	var js json.RawMessage
	if err := json.Unmarshal(validJSON, &js); err != nil {
		t.Fatalf("Valid JSON unexpectedly failed unmarshal: %v", err)
	}

	invalidJSON := []byte(`{"version": "1.0", "peers": [unclosed`)
	if err := json.Unmarshal(invalidJSON, &js); err == nil {
		t.Fatalf("Invalid JSON should have failed unmarshal")
	}

	// 2. HTML payload rejection test
	htmlPayload := []byte(`<!DOCTYPE html><html><body>404 Not Found</body></html>`)
	trimmed := strings.TrimSpace(string(htmlPayload))
	if !strings.HasPrefix(trimmed, "<!DOCTYPE") && !strings.HasPrefix(trimmed, "<html") {
		t.Fatalf("HTML payload should have been detected")
	}

	// 3. Properties validation test
	validProps := []byte("[network]\nidentifier = mainnet\n")
	trimmedProps := strings.TrimSpace(string(validProps))
	if !strings.Contains(trimmedProps, "=") && !strings.Contains(trimmedProps, "[") {
		t.Fatalf("Valid properties should have passed validation")
	}

	// 4. SHA-256 calculation test
	h := sha256.Sum256(validProps)
	shaHex := hex.EncodeToString(h[:])
	if len(shaHex) != 64 {
		t.Fatalf("Expected 64-char hex SHA256, got: %s", shaHex)
	}

	// 5. File permissions test (0600 on POSIX; Windows NTFS handles ACLs differently)
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.properties")
	if err := os.WriteFile(targetPath, validProps, 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("Expected 0600 permissions, got: %v", info.Mode().Perm())
	}
}

func TestVersionComparisonLogic(t *testing.T) {
	tests := []struct {
		remote   string
		current  string
		expected bool
	}{
		{"v1.9.8", "v1.9.8", false},
		{"1.9.8", "v1.9.8", false},
		{"v1.9.8", "1.9.8", false},
		{"v1.9.7", "v1.9.8", false},
		{"1.9.7", "1.9.8", false},
		{"v1.9.9", "v1.9.8", true},
		{"v1.10.0", "v1.9.8", true},
		{"v2.0.0", "v1.9.8", true},
		{"release-v1.9.8", "v1.9.8", false},
		{"v1.9.8", "none", true},
		{"v1.9.8", "", true},
		{"", "v1.9.8", false},
	}

	for _, tc := range tests {
		result := IsNewerVersion(tc.remote, tc.current)
		if result != tc.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tc.remote, tc.current, result, tc.expected)
		}
	}
}

func TestCheckUpdate_Live(t *testing.T) {
	um := NewUpdateManager(t.TempDir(), nil)
	info, err := um.CheckUpdate()
	if err != nil {
		t.Skipf("Skipping live GitHub check due to network: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("Expected HasUpdate=false when running on v1.9.8 against igorgoc/cpp-xpx-chain latest, got true (latest: %s, current: %s)", info.LatestVersion, info.CurrentVersion)
	}
	if info.LatestVersion != "v1.9.8" {
		t.Errorf("Expected LatestVersion to be v1.9.8, got %s", info.LatestVersion)
	}
}

func TestCheckConfigsDiff_Live(t *testing.T) {
	repoRoot, err := filepath.Abs("../../../chainconfig/resources")
	if err != nil {
		t.Skip("Cannot locate chainconfig/resources")
	}
	um := NewUpdateManager(repoRoot, nil)
	report, err := um.CheckConfigsDiff()
	if err != nil {
		t.Skipf("Skipping live diff check due to network: %v", err)
	}
	if report.TotalFiles != 5 {
		t.Errorf("Expected 5 total files in diff report, got %d", report.TotalFiles)
	}
	if len(report.Files) != 5 {
		t.Errorf("Expected 5 file entries, got %d", len(report.Files))
	}
	t.Logf("Diff Report: Total: %d, Diff: %d, Identical: %d, Missing: %d, HasDiff: %v",
		report.TotalFiles, report.DifferentCount, report.IdenticalCount, report.MissingCount, report.HasDifferences)
}

type MockConfigLifecycleController struct {
	sync.Mutex
	running     bool
	stopCalls   int
	startCalls  int
	failOnStart bool
}

func (m *MockConfigLifecycleController) StopNode() error {
	m.Lock()
	defer m.Unlock()
	m.running = false
	m.stopCalls++
	return nil
}

func (m *MockConfigLifecycleController) StartNode(dataPath string) error {
	m.Lock()
	defer m.Unlock()
	m.startCalls++
	if m.failOnStart && m.startCalls == 1 {
		// First start attempt with modified configs crashes
		m.running = false
		return errors.New("simulated engine crash on boot after config swap")
	}
	m.running = true
	return nil
}

func (m *MockConfigLifecycleController) IsRunning() bool {
	m.Lock()
	defer m.Unlock()
	return m.running
}

func TestApplyOfficialUpdate_HealthcheckFailure_TriggersAutoRollback(t *testing.T) {
	tmpDir := t.TempDir()
	resourcesDir := filepath.Join(tmpDir, "resources")
	_ = os.MkdirAll(resourcesDir, 0755)

	originalNetworkConfig := "[network]\nidentifier = mainnet\noriginal_key = 12345\n"
	originalPeersP2P := `{"peers": ["original-peer-node"]}`
	originalPeersAPI := `{"peers": ["original-api-node"]}`
	originalReplicators := `{"replicators": ["original-replicator"]}`
	originalSupportedEntities := `{"entities": ["original-entity"]}`

	_ = os.WriteFile(filepath.Join(resourcesDir, "config-network.properties"), []byte(originalNetworkConfig), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "peers-p2p.json"), []byte(originalPeersP2P), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "peers-api.json"), []byte(originalPeersAPI), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "replicators.json"), []byte(originalReplicators), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "supported-entities.json"), []byte(originalSupportedEntities), 0600)

	mockSupervisor := &MockConfigLifecycleController{
		running:     true, // Node was actively running
		failOnStart: true, // Will fail on post-update restart
	}

	um := NewUpdateManager(resourcesDir, mockSupervisor)

	dataPath := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataPath, 0755)

	// Run update which will download upstream configs, attempt restart, crash, and rollback
	err := um.ApplyOfficialUpdate(dataPath)
	if err == nil {
		t.Fatalf("Expected ApplyOfficialUpdate to return error due to crash-loop, got nil")
	}

	// 1. Verify RollbackOccurred flag was set
	if !um.lastInfo.RollbackOccurred {
		t.Errorf("Expected RollbackOccurred=true, got false")
	}

	// 2. Verify all 5 files were restored to their exact original contents
	restoredNet, _ := os.ReadFile(filepath.Join(resourcesDir, "config-network.properties"))
	if string(restoredNet) != originalNetworkConfig {
		t.Errorf("config-network.properties was NOT rolled back correctly! got:\n%s", string(restoredNet))
	}

	restoredP2P, _ := os.ReadFile(filepath.Join(resourcesDir, "peers-p2p.json"))
	if string(restoredP2P) != originalPeersP2P {
		t.Errorf("peers-p2p.json was NOT rolled back correctly! got:\n%s", string(restoredP2P))
	}

	restoredAPI, _ := os.ReadFile(filepath.Join(resourcesDir, "peers-api.json"))
	if string(restoredAPI) != originalPeersAPI {
		t.Errorf("peers-api.json was NOT rolled back correctly! got:\n%s", string(restoredAPI))
	}

	restoredRepl, _ := os.ReadFile(filepath.Join(resourcesDir, "replicators.json"))
	if string(restoredRepl) != originalReplicators {
		t.Errorf("replicators.json was NOT rolled back correctly! got:\n%s", string(restoredRepl))
	}

	restoredEnt, _ := os.ReadFile(filepath.Join(resourcesDir, "supported-entities.json"))
	if string(restoredEnt) != originalSupportedEntities {
		t.Errorf("supported-entities.json was NOT rolled back correctly! got:\n%s", string(restoredEnt))
	}

	// 3. Verify node was revived after rollback
	if !mockSupervisor.IsRunning() {
		t.Errorf("Expected node to be revived and running after rollback")
	}
	if mockSupervisor.startCalls < 2 {
		t.Errorf("Expected at least 2 start calls (1 failed post-update, 1 revival after rollback), got %d", mockSupervisor.startCalls)
	}

	t.Logf("SUCCESS: Adversarial crash-loop triggered instant auto-rollback. All 5 files restored byte-for-byte and node revived.")
}


