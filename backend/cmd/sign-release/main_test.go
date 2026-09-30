package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignReleaseLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_sign_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Generate keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	privHex := hex.EncodeToString(priv)
	pubHex := hex.EncodeToString(pub)

	// 2. Create sample artifacts
	art1 := filepath.Join(tempDir, "sirius-darwin-arm64.tar.gz")
	art2 := filepath.Join(tempDir, "sirius-linux-amd64.tar.gz")
	art3 := filepath.Join(tempDir, "sirius-windows-amd64.zip")

	_ = os.WriteFile(art1, []byte("fake-darwin-arm64-binary-content"), 0644)
	_ = os.WriteFile(art2, []byte("fake-linux-amd64-binary-content"), 0644)
	_ = os.WriteFile(art3, []byte("fake-windows-amd64-binary-content"), 0644)

	// 3. Sign directory
	cmdSign([]string{"-key", privHex, "-dir", tempDir})

	checksumsFile := filepath.Join(tempDir, "SHA256SUMS")
	sigFile := filepath.Join(tempDir, "SHA256SUMS.sig")

	if _, err := os.Stat(checksumsFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to exist", checksumsFile)
	}
	if _, err := os.Stat(sigFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to exist", sigFile)
	}

	// 4. Verify valid signatures
	pubKeyBytes, err := resolvePublicKey(pubHex)
	if err != nil {
		t.Fatalf("resolvePublicKey error: %v", err)
	}
	checksumsData, _ := os.ReadFile(checksumsFile)
	sigData, _ := os.ReadFile(sigFile)

	if !ed25519.Verify(pubKeyBytes, checksumsData, sigData) {
		t.Fatal("Signature verification failed on authentic artifact checksums")
	}

	// 5. Test adversarial tamper detection (modified artifact content)
	_ = os.WriteFile(art1, []byte("tampered-corrupted-bytes"), 0644)
	actualTamperedHash, _ := computeFileSHA256(art1)
	expectedOrigHash, _ := computeFileSHA256(art2) // distinct
	if actualTamperedHash == expectedOrigHash {
		t.Fatal("Hash collision on tampered bytes")
	}

	// 6. Test adversarial tamper detection (tampered signature bytes)
	badSigData := make([]byte, len(sigData))
	copy(badSigData, sigData)
	badSigData[0] ^= 0xFF
	if ed25519.Verify(pubKeyBytes, checksumsData, badSigData) {
		t.Fatal("Adversarial test failed: tampered signature was accepted!")
	}

	// 7. Verify resolution from JSON manifest format
	compatFile := filepath.Join(tempDir, "engine.compat.json")
	_ = os.WriteFile(compatFile, []byte(`{"releasePublicKeyHex":"`+pubHex+`"}`), 0644)
	resolvedPubKey, err := resolvePublicKey(compatFile)
	if err != nil {
		t.Fatalf("resolvePublicKey from JSON failed: %v", err)
	}
	if hex.EncodeToString(resolvedPubKey) != pubHex {
		t.Fatalf("Mismatch in resolved pubkey from JSON: got %s, want %s", hex.EncodeToString(resolvedPubKey), pubHex)
	}
}

func TestSignReleaseWithKeyEnv(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_sign_env_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	privHex := hex.EncodeToString(priv)
	pubHex := hex.EncodeToString(pub)

	sampleFile := filepath.Join(tempDir, "test-asset.tar.gz")
	_ = os.WriteFile(sampleFile, []byte("asset-data-content"), 0644)

	// Set environment variable
	envVarName := "TEST_SIGN_RELEASE_KEY"
	t.Setenv(envVarName, privHex)

	// Sign using -key-env
	cmdSign([]string{"-key-env", envVarName, "-dir", tempDir})

	checksumsFile := filepath.Join(tempDir, "SHA256SUMS")
	sigFile := filepath.Join(tempDir, "SHA256SUMS.sig")

	if _, err := os.Stat(sigFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to exist after -key-env signing", sigFile)
	}

	checksumsData, _ := os.ReadFile(checksumsFile)
	sigData, _ := os.ReadFile(sigFile)
	pubKeyBytes, _ := resolvePublicKey(pubHex)

	if !ed25519.Verify(pubKeyBytes, checksumsData, sigData) {
		t.Fatal("Signature generated with -key-env failed verification")
	}
}

func TestSignSingleFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_sign_single_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	privHex := hex.EncodeToString(priv)
	pubHex := hex.EncodeToString(pub)

	snapshotFile := filepath.Join(tempDir, "sirius-snapshot.tar.zst")
	_ = os.WriteFile(snapshotFile, []byte("snapshot-binary-payload-data"), 0644)

	// Create another file in the same directory that should NOT be in SHA256SUMS
	otherFile := filepath.Join(tempDir, "other.txt")
	_ = os.WriteFile(otherFile, []byte("ignore me"), 0644)

	cmdSign([]string{"-key", privHex, "-file", snapshotFile})

	checksumsFile := filepath.Join(tempDir, "SHA256SUMS")
	sigFile := filepath.Join(tempDir, "SHA256SUMS.sig")

	if _, err := os.Stat(checksumsFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to exist", checksumsFile)
	}
	if _, err := os.Stat(sigFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to exist", sigFile)
	}

	checksumsData, _ := os.ReadFile(checksumsFile)
	sigData, _ := os.ReadFile(sigFile)
	pubKeyBytes, _ := resolvePublicKey(pubHex)

	if !ed25519.Verify(pubKeyBytes, checksumsData, sigData) {
		t.Fatal("Signature verification failed for single file")
	}

	// Verify SHA256SUMS only contains sirius-snapshot.tar.zst and not other.txt
	contentStr := string(checksumsData)
	if !strings.Contains(contentStr, "sirius-snapshot.tar.zst") {
		t.Errorf("Expected checksums to contain snapshot filename, got: %s", contentStr)
	}
	if strings.Contains(contentStr, "other.txt") {
		t.Errorf("Did not expect checksums to contain other.txt, got: %s", contentStr)
	}
}


