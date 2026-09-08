package snapshot

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyEd25519Signature validates that signature matches data using the hex-encoded Ed25519 public key.
func VerifyEd25519Signature(pubKeyHex string, data, signature []byte) error {
	pubKeyBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid release public key format: %w", err)
	}
	if !ed25519.Verify(pubKeyBytes, data, signature) {
		return ErrInvalidSignature
	}
	return nil
}

// ParseSha256Sums extracts the expected SHA-256 hash for targetFilename from standard SHA256SUMS format.
func ParseSha256Sums(checksumsText, targetFilename string) (string, error) {
	lines := strings.Split(checksumsText, "\n")
	targetBase := filepath.Base(targetFilename)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			hash := strings.ToLower(fields[0])
			filename := filepath.Base(fields[1])
			if filename == targetBase || filename == targetFilename {
				return hash, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", ErrMissingChecksum, targetFilename)
}

// ComputeFileSha256 returns the hex-encoded SHA-256 hash of a local file.
func ComputeFileSha256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
