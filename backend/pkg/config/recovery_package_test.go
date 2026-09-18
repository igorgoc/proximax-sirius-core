package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisasterRecoveryPackage_EndToEnd(t *testing.T) {
	// Create temporary mock chainconfig directory
	tmpDir, err := os.MkdirTemp("", "sirius_dr_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	certDir := filepath.Join(tmpDir, "certificate")
	_ = os.MkdirAll(resourcesDir, 0700)
	_ = os.MkdirAll(certDir, 0700)

	// Write mock configuration files, including secret keys
	bootKeyExpected := "4af42eff3165f796a34be0bc58f80eb9888708ad22c0803323236de9952ca51a"
	harvestKeyExpected := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-user.properties"), []byte("[account]\nbootKey = "+bootKeyExpected+"\n"), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-harvesting.properties"), []byte("[harvesting]\nharvestKey = "+harvestKeyExpected+"\n"), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-node.properties"), []byte("[node]\nfriendlyName = dr-test-node\nport = 7900\n"), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, "config-immutable.properties"), []byte("[network]\nidentifier = mainnet\n"), 0600)
	_ = os.WriteFile(filepath.Join(resourcesDir, ".sirius-token"), []byte("secure-api-token-1234567890abcdef"), 0600)

	// Write mock certificates
	certExpected := "-----BEGIN CERTIFICATE-----\nMOCK_CERT_DATA\n-----END CERTIFICATE-----"
	_ = os.WriteFile(filepath.Join(certDir, "node.crt.pem"), []byte(certExpected), 0600)

	cm := NewConfigManager(tmpDir)

	passphrase := []byte("SuperSecurePassphrase2026!")

	// 1. Export package
	pkgData, err := cm.ExportDisasterRecoveryPackage(append([]byte(nil), passphrase...))
	if err != nil {
		t.Fatalf("ExportDisasterRecoveryPackage failed: %v", err)
	}

	// Verify plaintext keys are NOT in package JSON
	if strings.Contains(string(pkgData), bootKeyExpected) {
		t.Fatalf("Security violation: bootKey appears unencrypted in disaster recovery package")
	}
	if strings.Contains(string(pkgData), harvestKeyExpected) {
		t.Fatalf("Security violation: harvestKey appears unencrypted in disaster recovery package")
	}

	// 2. Simulate fresh machine: wipe resources and certificates completely
	_ = os.RemoveAll(resourcesDir)
	_ = os.RemoveAll(certDir)

	// 3. Attempt restore with wrong passphrase
	wrongPass := []byte("WrongPassword123!")
	if err := cm.RestoreDisasterRecoveryPackage(pkgData, wrongPass); err == nil {
		t.Fatalf("Expected restore to fail with wrong passphrase, but it succeeded")
	}

	// 4. Restore with correct passphrase
	if err := cm.RestoreDisasterRecoveryPackage(pkgData, append([]byte(nil), passphrase...)); err != nil {
		t.Fatalf("RestoreDisasterRecoveryPackage failed with correct passphrase: %v", err)
	}

	// 5. Verify restored files and exact unredacted keys
	userProps, err := os.ReadFile(filepath.Join(resourcesDir, "config-user.properties"))
	if err != nil {
		t.Fatalf("Failed reading restored config-user.properties: %v", err)
	}
	if !strings.Contains(string(userProps), bootKeyExpected) {
		t.Fatalf("Restored config-user.properties missing original bootKey: %s", string(userProps))
	}

	harvProps, err := os.ReadFile(filepath.Join(resourcesDir, "config-harvesting.properties"))
	if err != nil {
		t.Fatalf("Failed reading restored config-harvesting.properties: %v", err)
	}
	if !strings.Contains(string(harvProps), harvestKeyExpected) {
		t.Fatalf("Restored config-harvesting.properties missing original harvestKey: %s", string(harvProps))
	}

	restoredCert, err := os.ReadFile(filepath.Join(certDir, "node.crt.pem"))
	if err != nil {
		t.Fatalf("Failed reading restored cert: %v", err)
	}
	if string(restoredCert) != certExpected {
		t.Fatalf("Restored cert content mismatch: got %s, expected %s", string(restoredCert), certExpected)
	}

	restoredToken, err := os.ReadFile(filepath.Join(resourcesDir, ".sirius-token"))
	if err != nil {
		t.Fatalf("Failed reading restored .sirius-token: %v", err)
	}
	if string(restoredToken) != "secure-api-token-1234567890abcdef" {
		t.Fatalf("Restored token mismatch: %s", string(restoredToken))
	}
}
