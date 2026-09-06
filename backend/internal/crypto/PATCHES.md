# Patches to `github.com/proximax-storage/go-xpx-crypto` (v0.1.0)

## Why This Package Is Forked / Replaced Locally
In `backend/go.mod`, `github.com/proximax-storage/go-xpx-crypto` is replaced with `./internal/crypto`.

This local fork exists solely to enforce the **local private key memory zeroization guarantee**.

## Upstream Limitation
In upstream `go-xpx-crypto@v0.1.0`:
* `crypto.PrivateKey` has two internal representations of the private key:
  ```go
  type PrivateKey struct {
      value *big.Int  // Unexported field initialized by (&big.Int{}).SetBytes(raw)
      Raw   []byte    // Raw 32-byte slice
  }
  ```
* Because `value *big.Int` is an unexported field, external callers cannot access or wipe it without fragile `unsafe.Pointer` struct-layout assumptions.
* Zeroing `Raw []byte` alone leaves the private key words inside `value.Bits()` (`[]big.Word`) on the heap until GC sweeps it.
* Standard library `math/big.Int.SetInt64(0)` only truncates slice length to `[:0]`, leaving the backing array non-zero in RAM unless its words are overwritten first.

## Patches Added in This Fork

### 1. `key_model.go`
Added exported `Destroy()` and `Zero()` methods on `*PrivateKey`:
```go
// Destroy explicitly zeroes all secret key representations in memory
// (both the raw byte slice and the internal math/big.Int words).
func (ref *PrivateKey) Destroy() {
	if ref == nil {
		return
	}
	// 1. Wipe raw bytes
	for i := range ref.Raw {
		ref.Raw[i] = 0
	}
	ref.Raw = nil

	// 2. Wipe math/big.Int words before reslicing
	if ref.value != nil {
		words := ref.value.Bits()
		for i := range words {
			words[i] = 0
		}
		ref.value.SetInt64(0)
		ref.value = nil
	}
}

// Zero is an alias for Destroy
func (ref *PrivateKey) Zero() {
	ref.Destroy()
}
```

### 2. `key_pair.go`
Added exported `Destroy()` and `Zero()` methods on `*KeyPair`:
```go
// Destroy explicitly zeroes the underlying PrivateKey if present.
func (ref *KeyPair) Destroy() {
	if ref != nil && ref.PrivateKey != nil {
		ref.PrivateKey.Destroy()
	}
}

// Zero is an alias for Destroy.
func (ref *KeyPair) Zero() {
	ref.Destroy()
}
```

## Invariant for Future Maintainers
**DO NOT remove the `replace` directive in `go.mod` without porting or upstreaming `Destroy()` and `Zero()`.**
Any automated dependency update that drops this patch will cause compilation errors or compromise the zero-RAM-retention guarantee of private keys during delegated harvester account linking.
