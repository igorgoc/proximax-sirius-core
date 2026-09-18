package crypto

import (
	"crypto/rand"
	"testing"
)

func TestPrivateKey_Destroy_WipesMemory(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}

	priv := NewPrivateKey(raw)
	if priv == nil {
		t.Fatal("expected non-nil PrivateKey")
	}
	if priv.Raw == nil || len(priv.Raw) != 32 {
		t.Fatal("expected 32-byte Raw slice")
	}
	if priv.value == nil {
		t.Fatal("expected non-nil value (*big.Int)")
	}

	// Capture the slice of big.Word backing array before Destroy
	words := priv.value.Bits()
	if len(words) == 0 {
		t.Fatal("expected non-empty big.Word slice")
	}

	// Verify that words originally contain non-zero data
	hasNonZero := false
	for _, w := range words {
		if w != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Fatal("expected non-zero big.Word data prior to Destroy")
	}

	// Call Destroy
	priv.Destroy()

	// Verify Raw is nil
	if priv.Raw != nil {
		t.Errorf("expected priv.Raw to be nil after Destroy, got %v", priv.Raw)
	}

	// Verify value is nil
	if priv.value != nil {
		t.Errorf("expected priv.value to be nil after Destroy, got %v", priv.value)
	}

	// Invariant Check: The backing array of big.Word that was captured MUST be zeroed
	for i, w := range words {
		if w != 0 {
			t.Errorf("big.Word backing array at index %d was not zeroed: %x", i, w)
		}
	}

	// Idempotence: calling Destroy or Zero again must not panic
	priv.Destroy()
	priv.Zero()
}

func TestPrivateKey_NilReceiver_DoesNotPanic(t *testing.T) {
	var priv *PrivateKey
	// Must not panic on nil receiver
	priv.Destroy()
	priv.Zero()
}

func TestKeyPair_Destroy_WipesPrivateKey(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}

	priv := NewPrivateKey(raw)
	words := priv.value.Bits()

	kp, err := NewKeyPair(priv, nil, nil)
	if err != nil {
		t.Fatalf("NewKeyPair failed: %v", err)
	}

	if !kp.HasPrivateKey() {
		t.Fatal("expected KeyPair to have private key")
	}

	// Call Destroy on KeyPair
	kp.Destroy()

	if priv.Raw != nil {
		t.Errorf("expected priv.Raw to be nil after kp.Destroy()")
	}
	if priv.value != nil {
		t.Errorf("expected priv.value to be nil after kp.Destroy()")
	}

	// Verify big.Word backing array is zeroed
	for i, w := range words {
		if w != 0 {
			t.Errorf("backing big.Word at index %d was not zeroed: %x", i, w)
		}
	}

	// Idempotency
	kp.Destroy()
	kp.Zero()

	var nilKp *KeyPair
	nilKp.Destroy()
	nilKp.Zero()
}
