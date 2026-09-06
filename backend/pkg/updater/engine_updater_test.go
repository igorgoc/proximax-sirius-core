package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// MockLifecycleController simulates supervisor engine lifecycle
type MockLifecycleController struct {
	mu        sync.Mutex
	running   bool
	startErr  error
	stopCalls int
	startCalls int
}

func (m *MockLifecycleController) StopNode() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.stopCalls++
	return nil
}

func (m *MockLifecycleController) StartNode(dataPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		m.running = false
		return m.startErr
	}
	m.running = true
	m.startCalls++
	return nil
}

func (m *MockLifecycleController) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// helper to package a test binary into a tar.gz / zip matching current platform
func createPlatformPackage(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
	return buf.Bytes()
}

func setupTestEnvironment(t *testing.T) (string, string, ed25519.PublicKey, ed25519.PrivateKey, *MockLifecycleController) {
	t.Helper()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	_ = os.MkdirAll(binDir, 0755)

	_, binaryName := PlatformAssetDescriptor()
	origBinPath := filepath.Join(binDir, binaryName)
	if err := os.WriteFile(origBinPath, []byte("ORIGINAL_ENGINE_V1_9_7"), 0755); err != nil {
		t.Fatalf("failed creating initial binary: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed generating test keys: %v", err)
	}

	manifest := CompatibilityManifest{
		EngineRepository:    "proximax-storage/cpp-xpx-chain",
		EngineMinCompatible: "v1.9.0",
		EngineMaxCompatible: "v1.9.99",
		RecommendedVersion:  "v1.9.8",
		ReleasePublicKeyHex: hex.EncodeToString(pub),
	}
	manifestData, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(tmpDir, "engine.compat.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed writing manifest: %v", err)
	}

	mockCtrl := &MockLifecycleController{running: true}
	return tmpDir, binDir, pub, priv, mockCtrl
}

// 1. Adversarial Test: Valid update succeeds and swaps binary cleanly
func TestEngineUpdater_ValidUpdate_Success(t *testing.T) {
	_, binDir, _, priv, mockCtrl := setupTestEnvironment(t)
	manifestPath := filepath.Join(filepath.Dir(binDir), "engine.compat.json")

	var auditLogs []string
	auditLogger := func(action, details string) {
		auditLogs = append(auditLogs, fmt.Sprintf("%s: %s", action, details))
	}

	updater := NewEngineUpdater(binDir, manifestPath, mockCtrl, auditLogger)
	assetName, binaryName := PlatformAssetDescriptor()
	newBinContent := []byte("NEW_VERIFIED_ENGINE_V1_9_8")
	pkgBytes := createPlatformPackage(t, binaryName, newBinContent)

	pkgHash := sha256.Sum256(pkgBytes)
	pkgHashHex := hex.EncodeToString(pkgHash[:])

	checksumsContent := []byte(fmt.Sprintf("%s  %s\n", pkgHashHex, assetName))
	sig := ed25519.Sign(priv, checksumsContent)

	err := updater.ApplyUpdate(
		"v1.9.8",
		bytes.NewReader(pkgBytes),
		checksumsContent,
		sig,
		"/data",
		func() error { return nil }, // healthy probe
	)
	if err != nil {
		t.Fatalf("expected update to succeed, got error: %v", err)
	}

	// Verify active binary has updated content
	activePath := filepath.Join(binDir, binaryName)
	installedContent, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("failed reading active binary: %v", err)
	}
	if string(installedContent) != string(newBinContent) {
		t.Errorf("expected installed content '%s', got '%s'", newBinContent, installedContent)
	}

	// Verify .bak file is removed on success
	if _, err := os.Stat(activePath + ".bak"); !os.IsNotExist(err) {
		t.Errorf("expected backup file to be cleaned up on success")
	}

	status := updater.GetStatus()
	if status.CurrentVersion != "v1.9.8" || status.State != "completed" {
		t.Errorf("expected status completed v1.9.8, got %+v", status)
	}
}

// 2. Adversarial Test: Corrupted signature fails closed without writing to target path
func TestEngineUpdater_CorruptedSignature_FailsClosed(t *testing.T) {
	_, binDir, _, priv, mockCtrl := setupTestEnvironment(t)
	manifestPath := filepath.Join(filepath.Dir(binDir), "engine.compat.json")

	updater := NewEngineUpdater(binDir, manifestPath, mockCtrl, nil)
	assetName, binaryName := PlatformAssetDescriptor()
	pkgBytes := createPlatformPackage(t, binaryName, []byte("MALICIOUS_ENGINE_PAYLOAD"))

	pkgHash := sha256.Sum256(pkgBytes)
	checksumsContent := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(pkgHash[:]), assetName))
	sig := ed25519.Sign(priv, checksumsContent)

	// Tamper with signature bytes
	sig[5] ^= 0xFF
	sig[6] ^= 0xAA

	origContent, _ := os.ReadFile(filepath.Join(binDir, binaryName))

	err := updater.ApplyUpdate(
		"v1.9.8",
		bytes.NewReader(pkgBytes),
		checksumsContent,
		sig,
		"/data",
		func() error { return nil },
	)

	if err == nil {
		t.Fatalf("expected update to fail on corrupted signature, but it succeeded")
	}
	if !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got: %v", err)
	}

	// Invariant: Target binary must remain completely unmodified
	currentContent, _ := os.ReadFile(filepath.Join(binDir, binaryName))
	if string(currentContent) != string(origContent) {
		t.Errorf("binary was modified despite signature corruption!")
	}

	// Invariant: No temporary files left behind
	entries, _ := os.ReadDir(binDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temporary staging file was leaked: %s", e.Name())
		}
	}
}

// 3. Adversarial Test: Checksum mismatch fails closed immediately
func TestEngineUpdater_ChecksumMismatch_FailsClosed(t *testing.T) {
	_, binDir, _, priv, mockCtrl := setupTestEnvironment(t)
	manifestPath := filepath.Join(filepath.Dir(binDir), "engine.compat.json")

	updater := NewEngineUpdater(binDir, manifestPath, mockCtrl, nil)
	assetName, binaryName := PlatformAssetDescriptor()
	pkgBytes := createPlatformPackage(t, binaryName, []byte("TAMPERED_ENGINE_BITS"))

	// Valid signature over a fake checksum
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumsContent := []byte(fmt.Sprintf("%s  %s\n", fakeHash, assetName))
	sig := ed25519.Sign(priv, checksumsContent)

	origContent, _ := os.ReadFile(filepath.Join(binDir, binaryName))

	err := updater.ApplyUpdate(
		"v1.9.8",
		bytes.NewReader(pkgBytes),
		checksumsContent,
		sig,
		"/data",
		func() error { return nil },
	)

	if err == nil {
		t.Fatalf("expected update to fail on checksum mismatch, but it succeeded")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("expected ErrChecksumMismatch, got: %v", err)
	}

	// Invariant: Original binary remains untouched
	currentContent, _ := os.ReadFile(filepath.Join(binDir, binaryName))
	if string(currentContent) != string(origContent) {
		t.Errorf("binary was modified despite checksum mismatch!")
	}
}

// 4. Adversarial Test: Crash-loop / healthcheck failure triggers automated rollback
func TestEngineUpdater_CrashLoop_TriggersAutoRollback(t *testing.T) {
	_, binDir, _, priv, mockCtrl := setupTestEnvironment(t)
	manifestPath := filepath.Join(filepath.Dir(binDir), "engine.compat.json")

	var auditLogs []string
	auditLogger := func(action, details string) {
		auditLogs = append(auditLogs, fmt.Sprintf("%s: %s", action, details))
	}

	updater := NewEngineUpdater(binDir, manifestPath, mockCtrl, auditLogger)
	assetName, binaryName := PlatformAssetDescriptor()
	newBinContent := []byte("CRASHING_ENGINE_V1_9_8")
	pkgBytes := createPlatformPackage(t, binaryName, newBinContent)

	pkgHash := sha256.Sum256(pkgBytes)
	checksumsContent := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(pkgHash[:]), assetName))
	sig := ed25519.Sign(priv, checksumsContent)

	origContent, _ := os.ReadFile(filepath.Join(binDir, binaryName))

	// Healthcheck fails: simulates engine crash on startup
	failingHealthcheck := func() error {
		return errors.New("SIGSEGV / crash-loop: invalid memory reference in RocksDB engine")
	}

	err := updater.ApplyUpdate(
		"v1.9.8",
		bytes.NewReader(pkgBytes),
		checksumsContent,
		sig,
		"/data",
		failingHealthcheck,
	)

	if err == nil {
		t.Fatalf("expected update to fail due to healthcheck error, but got nil")
	}
	if !errors.Is(err, ErrHealthcheckFailed) {
		t.Errorf("expected ErrHealthcheckFailed, got: %v", err)
	}

	// Invariant: Active binary must be automatically restored to ORIGINAL_ENGINE_V1_9_7
	activePath := filepath.Join(binDir, binaryName)
	restoredContent, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("failed reading active binary after rollback: %v", err)
	}
	if string(restoredContent) != string(origContent) {
		t.Errorf("expected rolled back content '%s', got '%s'", origContent, restoredContent)
	}

	status := updater.GetStatus()
	if !status.RollbackOccurred {
		t.Errorf("expected RollbackOccurred = true")
	}
	if status.State != "rolled_back" {
		t.Errorf("expected status 'rolled_back', got '%s'", status.State)
	}

	// Verify audit log captured the rollback
	foundRollbackAudit := false
	for _, l := range auditLogs {
		if strings.Contains(l, "ROLLBACK_TRIGGERED") {
			foundRollbackAudit = true
			break
		}
	}
	if !foundRollbackAudit {
		t.Errorf("expected audit log to record ROLLBACK_TRIGGERED event, logs: %v", auditLogs)
	}
}

// 5. Test: Version outside compatibility manifest fails closed before downloading
func TestEngineUpdater_IncompatibleVersion_FailsClosed(t *testing.T) {
	_, binDir, _, priv, mockCtrl := setupTestEnvironment(t)
	manifestPath := filepath.Join(filepath.Dir(binDir), "engine.compat.json")

	updater := NewEngineUpdater(binDir, manifestPath, mockCtrl, nil)
	assetName, binaryName := PlatformAssetDescriptor()
	pkgBytes := createPlatformPackage(t, binaryName, []byte("INCOMPATIBLE_V2_0_0"))

	pkgHash := sha256.Sum256(pkgBytes)
	checksumsContent := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(pkgHash[:]), assetName))
	sig := ed25519.Sign(priv, checksumsContent)

	// v2.0.0 is above engineMaxCompatible (v1.9.99)
	err := updater.ApplyUpdate(
		"v2.0.0",
		bytes.NewReader(pkgBytes),
		checksumsContent,
		sig,
		"/data",
		func() error { return nil },
	)

	if err == nil {
		t.Fatalf("expected update to fail on incompatible version, but it succeeded")
	}
	if !errors.Is(err, ErrIncompatibleVersion) {
		t.Errorf("expected ErrIncompatibleVersion, got: %v", err)
	}
}
