package crypto

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	internalcrypto "github.com/proximax-storage/go-xpx-crypto"
)

func TestGenerateKeyPair_Valid(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() returned unexpected error: %v", err)
	}

	if kp == nil {
		t.Fatal("expected non-nil KeyPairInfo")
	}

	if len(kp.PrivateKey) != 64 {
		t.Errorf("expected 64-char private key hex, got %d chars", len(kp.PrivateKey))
	}

	if len(kp.PublicKey) != 64 {
		t.Errorf("expected 64-char public key hex, got %d chars", len(kp.PublicKey))
	}

	// Mainnet / Public Sirius addresses begin with 'X' and are 40 chars
	if !strings.HasPrefix(kp.Address, "X") {
		t.Errorf("expected Sirius Public network address starting with 'X', got %s", kp.Address)
	}
	if len(kp.Address) != 40 {
		t.Errorf("expected address length 40, got %d (%s)", len(kp.Address), kp.Address)
	}

	// Deterministic re-derivation check
	derived, err := KeyPairFromPrivateKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("KeyPairFromPrivateKey failed to re-derive from generated private key: %v", err)
	}

	if !strings.EqualFold(derived.PublicKey, kp.PublicKey) {
		t.Errorf("derived public key mismatch: got %s, want %s", derived.PublicKey, kp.PublicKey)
	}
	if derived.Address != kp.Address {
		t.Errorf("derived address mismatch: got %s, want %s", derived.Address, kp.Address)
	}
}

func TestKeyPairFromPrivateKey_Validation(t *testing.T) {
	// Test invalid lengths
	tests := []struct {
		name    string
		keyHex  string
		wantErr bool
	}{
		{"empty string", "", true},
		{"too short (63 chars)", strings.Repeat("a", 63), true},
		{"too long (65 chars)", strings.Repeat("a", 65), true},
		{"non-hex chars", strings.Repeat("z", 64), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := KeyPairFromPrivateKey(tt.keyHex)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyPairFromPrivateKey(%q) err = %v, wantErr = %v", tt.keyHex, err, tt.wantErr)
			}
		})
	}
}

func TestSecretKeyBuffer_UnmarshalJSON(t *testing.T) {
	// 64-hex chars = 32 bytes
	rawHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	jsonStr := `"` + rawHex + `"`

	var buf SecretKeyBuffer
	if err := json.Unmarshal([]byte(jsonStr), &buf); err != nil {
		t.Fatalf("UnmarshalJSON failed for valid 64-hex: %v", err)
	}

	if len(buf) != 32 {
		t.Fatalf("expected buffer length 32, got %d", len(buf))
	}

	expectedBytes, _ := hex.DecodeString(rawHex)
	for i := range buf {
		if buf[i] != expectedBytes[i] {
			t.Fatalf("byte mismatch at %d: got %x, want %x", i, buf[i], expectedBytes[i])
		}
	}

	// Negative tests
	invalidCases := []struct {
		name string
		raw  string
	}{
		{"not quoted", `0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`},
		{"too short", `"0123456789abcdef"`},
		{"too long", `"` + rawHex + `aa"`},
		{"invalid hex characters", `"` + strings.Repeat("x", 64) + `"`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			var b SecretKeyBuffer
			if err := json.Unmarshal([]byte(tc.raw), &b); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestSecretKeyBuffer_Wipe(t *testing.T) {
	buf := make(SecretKeyBuffer, 32)
	for i := range buf {
		buf[i] = byte(i + 1)
	}

	// Ensure it has non-zero bytes before Wipe
	hasNonZero := false
	for _, b := range buf {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Fatal("expected non-zero bytes in buffer before Wipe")
	}

	buf.Wipe()

	// Invariant: every single byte must be 0x00 after Wipe
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("buffer byte at index %d was not zeroed: %x", i, b)
		}
	}
}

func TestPerformDelegatedHarvestingLink_BufferWipedOnReturn(t *testing.T) {
	// 1. Invalid length returns error
	_, err := PerformDelegatedHarvestingLink([]byte{1, 2, 3}, "", "", "")
	if err == nil {
		t.Error("expected error for invalid key length, got nil")
	}

	// 2. 32-byte buffer passed in MUST be zeroed by defer even on early error returns
	keyBuf := make([]byte, 32)
	for i := range keyBuf {
		keyBuf[i] = 0xFF
	}

	// Pass invalid URL to force error exit
	_, _ = PerformDelegatedHarvestingLink(keyBuf, "invalid", "http://invalid.local.nonexistent:9999", "link_and_register")

	for i, b := range keyBuf {
		if b != 0 {
			t.Fatalf("keyBuf byte %d was not zeroed on return: 0x%x", i, b)
		}
	}
}

func TestDestroyAndZero_DirectAndIdempotent(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 0x5A
	}

	priv := internalcrypto.NewPrivateKey(raw)
	if priv == nil || priv.Raw == nil {
		t.Fatal("failed to initialize PrivateKey")
	}

	kp, err := internalcrypto.NewKeyPair(priv, nil, nil)
	if err != nil {
		t.Fatalf("NewKeyPair failed: %v", err)
	}

	// Destroy KeyPair
	kp.Destroy()

	if priv.Raw != nil {
		t.Errorf("expected priv.Raw to be nil after Destroy, got %v", priv.Raw)
	}

	// Calling Destroy and Zero multiple times must be idempotent and never panic
	kp.Destroy()
	kp.Zero()
	priv.Destroy()
	priv.Zero()

	var nilKp *internalcrypto.KeyPair
	nilKp.Destroy()
	nilKp.Zero()

	var nilPriv *internalcrypto.PrivateKey
	nilPriv.Destroy()
	nilPriv.Zero()
}
