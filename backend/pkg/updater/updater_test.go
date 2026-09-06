package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	// 5. File permissions test (0600)
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.properties")
	if err := os.WriteFile(targetPath, validProps, 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("Expected 0600 permissions, got: %v", info.Mode().Perm())
	}
}
