package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// ZeroBytes explicitly zeroes out a byte slice to prevent sensitive material
// from lingering on the heap for garbage collection.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// DisasterRecoveryPackage represents a fully encrypted and authenticated DR archive.
type DisasterRecoveryPackage struct {
	Magic     string     `json:"magic"`     // "SIRIUS-DR-PACKAGE-V1"
	Version   int        `json:"version"`   // 1
	CreatedAt time.Time  `json:"createdAt"`
	KDF       KDFParams  `json:"kdf"`
	Cipher    CipherData `json:"cipher"`
}

// KDFParams specifies the Argon2id parameters used to derive the encryption key.
type KDFParams struct {
	Algorithm string `json:"algorithm"` // "argon2id"
	Time      uint32 `json:"time"`      // iterations
	Memory    uint32 `json:"memory"`    // memory in KiB
	Threads   uint8  `json:"threads"`   // parallelism
	KeyLen    uint32 `json:"keyLen"`    // key length in bytes
	Salt      string `json:"salt"`      // base64-encoded salt
}

// CipherData specifies the AES-256-GCM ciphertext, nonce, and tag.
type CipherData struct {
	Algorithm  string `json:"algorithm"`  // "aes-256-gcm"
	Nonce      string `json:"nonce"`      // base64-encoded 12-byte nonce
	Ciphertext string `json:"ciphertext"` // base64-encoded ciphertext with tag
}

// RecoveryPayload contains the full unredacted configuration, keys, and certificates.
type RecoveryPayload struct {
	FriendlyName string            `json:"friendlyName"`
	ExportedAt   time.Time         `json:"exportedAt"`
	ConfigFiles  map[string]string `json:"configFiles"`
	CertFiles    map[string]string `json:"certFiles"`
	ApiToken     string            `json:"apiToken,omitempty"`
}

// GetCertificatePath returns the path to chainconfig/certificate.
func (cm *ConfigManager) GetCertificatePath() string {
	return filepath.Join(cm.basePath, "certificate")
}

// ExportDisasterRecoveryPackage exports ALL 21 Catapult config files (with unredacted keys)
// and TLS certificates, encrypted with an operator-supplied passphrase.
// Invariant: The raw passphrase, plaintext payload, and derived encryption key are explicitly zeroed.
func (cm *ConfigManager) ExportDisasterRecoveryPackage(passphrase []byte) ([]byte, error) {
	defer ZeroBytes(passphrase)

	if len(passphrase) < 8 {
		return nil, errors.New("passphrase must be at least 8 characters long")
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	resourcesDir := cm.GetResourcesPath()
	certDir := cm.GetCertificatePath()

	payload := &RecoveryPayload{
		ExportedAt:  time.Now().UTC(),
		ConfigFiles: make(map[string]string),
		CertFiles:   make(map[string]string),
	}

	cfg, _ := cm.LoadNodeConfig()
	if cfg != nil {
		payload.FriendlyName = cfg.FriendlyName
	}

	// 1. Read all files in chainconfig/resources (unredacted properties, JSONs, templates)
	if entries, err := os.ReadDir(resourcesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fn := entry.Name()
			if strings.HasPrefix(fn, ".tmp-") || fn == "server.lock" || fn == "recovery.lock" {
				continue
			}
			filePath := filepath.Join(resourcesDir, fn)
			data, rErr := os.ReadFile(filePath)
			if rErr != nil {
				continue
			}
			if fn == ".sirius-token" {
				payload.ApiToken = strings.TrimSpace(string(data))
			} else {
				payload.ConfigFiles[fn] = string(data)
			}
		}
	} else {
		return nil, fmt.Errorf("failed to read resources directory: %w", err)
	}

	// 2. Read all files in chainconfig/certificate (TLS certificates and private keys)
	if entries, err := os.ReadDir(certDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fn := entry.Name()
			filePath := filepath.Join(certDir, fn)
			data, rErr := os.ReadFile(filePath)
			if rErr != nil {
				continue
			}
			payload.CertFiles[fn] = string(data)
		}
	}

	// 3. Serialize payload to bytes for encryption
	plainBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recovery payload: %w", err)
	}
	defer ZeroBytes(plainBytes)

	// 4. Generate 16 bytes of cryptographically secure random salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed generating random salt: %w", err)
	}

	// 5. Derive 32-byte AES-256 key via Argon2id (memory: 64MB, time: 3, threads: 4)
	const (
		argonTime    = 3
		argonMemory  = 64 * 1024
		argonThreads = 4
		argonKeyLen  = 32
	)
	derivedKey := argon2.IDKey(passphrase, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	defer ZeroBytes(derivedKey)

	// 6. Encrypt plaintext payload with AES-256-GCM
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plainBytes, nil)

	// 7. Construct encrypted DisasterRecoveryPackage
	pkg := &DisasterRecoveryPackage{
		Magic:     "SIRIUS-DR-PACKAGE-V1",
		Version:   1,
		CreatedAt: time.Now().UTC(),
		KDF: KDFParams{
			Algorithm: "argon2id",
			Time:      argonTime,
			Memory:    argonMemory,
			Threads:   argonThreads,
			KeyLen:    argonKeyLen,
			Salt:      base64.StdEncoding.EncodeToString(salt),
		},
		Cipher: CipherData{
			Algorithm:  "aes-256-gcm",
			Nonce:      base64.StdEncoding.EncodeToString(nonce),
			Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		},
	}

	return json.MarshalIndent(pkg, "", "  ")
}

// RestoreDisasterRecoveryPackage decrypts and restores all Catapult configurations,
// keys, and TLS certificates from a DisasterRecoveryPackage.
// Invariant: The raw passphrase, decrypted payload, and derived encryption key are explicitly zeroed.
func (cm *ConfigManager) RestoreDisasterRecoveryPackage(packageData []byte, passphrase []byte) error {
	defer ZeroBytes(passphrase)

	var pkg DisasterRecoveryPackage
	if err := json.Unmarshal(packageData, &pkg); err != nil {
		return fmt.Errorf("invalid disaster recovery package format: %w", err)
	}

	if pkg.Magic != "SIRIUS-DR-PACKAGE-V1" || pkg.Version != 1 {
		return errors.New("unsupported recovery package: invalid magic header or version")
	}

	if pkg.KDF.Algorithm != "argon2id" || pkg.Cipher.Algorithm != "aes-256-gcm" {
		return fmt.Errorf("unsupported encryption suite: kdf=%s, cipher=%s", pkg.KDF.Algorithm, pkg.Cipher.Algorithm)
	}

	salt, err := base64.StdEncoding.DecodeString(pkg.KDF.Salt)
	if err != nil {
		return fmt.Errorf("failed decoding salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(pkg.Cipher.Nonce)
	if err != nil {
		return fmt.Errorf("failed decoding nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(pkg.Cipher.Ciphertext)
	if err != nil {
		return fmt.Errorf("failed decoding ciphertext: %w", err)
	}

	// Derive key with Argon2id using parameters specified in package
	derivedKey := argon2.IDKey(passphrase, salt, pkg.KDF.Time, pkg.KDF.Memory, pkg.KDF.Threads, pkg.KDF.KeyLen)
	defer ZeroBytes(derivedKey)

	// Decrypt payload
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return fmt.Errorf("cipher initialization error: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm initialization error: %w", err)
	}

	plainBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return errors.New("decryption failed: incorrect passphrase or corrupted recovery package")
	}
	defer ZeroBytes(plainBytes)

	var payload RecoveryPayload
	if err := json.Unmarshal(plainBytes, &payload); err != nil {
		return fmt.Errorf("failed parsing decrypted recovery payload: %w", err)
	}

	if len(payload.ConfigFiles) == 0 {
		return errors.New("recovery package contains no configuration files")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	resourcesDir := cm.GetResourcesPath()
	certDir := cm.GetCertificatePath()

	// Ensure destination directories exist with hardened permissions
	if err := os.MkdirAll(resourcesDir, 0700); err != nil {
		return fmt.Errorf("failed creating resources directory: %w", err)
	}
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("failed creating certificate directory: %w", err)
	}

	// Restore all configuration files atomically with 0600 permissions
	for fn, content := range payload.ConfigFiles {
		cleanFn := filepath.Base(fn)
		if cleanFn == "." || cleanFn == "/" || cleanFn == ".." {
			continue
		}
		targetPath := filepath.Join(resourcesDir, cleanFn)
		if err := atomicWriteFile(targetPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed restoring %s: %w", cleanFn, err)
		}
	}

	// Restore certificate files atomically with 0600 permissions
	for fn, content := range payload.CertFiles {
		cleanFn := filepath.Base(fn)
		if cleanFn == "." || cleanFn == "/" || cleanFn == ".." {
			continue
		}
		targetPath := filepath.Join(certDir, cleanFn)
		if err := atomicWriteFile(targetPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed restoring certificate %s: %w", cleanFn, err)
		}
	}

	// Restore API token if present
	if payload.ApiToken != "" {
		tokenPath := filepath.Join(resourcesDir, ".sirius-token")
		_ = atomicWriteFile(tokenPath, []byte(payload.ApiToken), 0600)
	}

	return nil
}
