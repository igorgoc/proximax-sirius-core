// Copyright 2018 ProximaX Limited. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package crypto

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/pkg/errors"
	"github.com/proximax-storage/go-xpx-utils"
)

var ErrInvalidSizePrivateKey = errors.New("the length of private key is not 32")
var ErrInvalidSizePublicKey = errors.New("the length of public key is not 32")

// KeyAnalyzer Interface to analyze keys.
type KeyAnalyzer interface {
	// Gets a Value indicating whether or not the public key is compressed.
	IsKeyCompressed(publicKey *PublicKey) bool
}

// KeyGenerator Interface for generating keys.
type KeyGenerator interface {
	// Creates a random key pair.
	GenerateKeyPair() (*KeyPair, error)
	// Derives a public key from a private key.
	DerivePublicKey(privateKey *PrivateKey) *PublicKey
}

// PrivateKey Represents a private key.
type PrivateKey struct {
	// I have kept this field for compatibility
	value *big.Int
	Raw   []byte
}

// NewPrivateKey creates a new private key from []byte
func NewPrivateKey(raw []byte) *PrivateKey {
	return &PrivateKey{(&big.Int{}).SetBytes(raw), raw}
}

// NewPrivateKeyFromBigInt creates a new private key from []byte
func NewPrivateKeyFromBigInt(val *big.Int) *PrivateKey {
	return &PrivateKey{val, val.Bytes()}
}

// NewPrivateKeyfromHexString creates a private key from a hex strings.
func NewPrivateKeyfromHexString(sHex string) (*PrivateKey, error) {
	raw, err := utils.HexDecodeStringOdd(sHex)
	if err != nil {
		return nil, err
	}

	if len(raw) != 32 {
		return nil, ErrInvalidSizePrivateKey
	}

	return NewPrivateKey(raw), nil
}

func (ref *PrivateKey) String() string {
	return hex.EncodeToString(ref.Raw)
}

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

// PublicKey represents a public key.
type PublicKey struct {
	Raw []byte
}

// NewPublicKey creates a new public key.
func NewPublicKey(raw []byte) *PublicKey {
	return &PublicKey{raw}
}

// NewPublicKeyfromHex create public key from hex string
func NewPublicKeyfromHex(hStr string) (*PublicKey, error) {
	raw, err := utils.HexDecodeStringOdd(hStr)
	if err != nil {
		return nil, err
	}

	if len(raw) != 32 {
		return nil, ErrInvalidSizePublicKey
	}

	return NewPublicKey(raw), nil
}

// Creates a public key from a hex strings.
func (ref *PublicKey) hex() string {
	return string(hex.EncodeToString(ref.Raw))
}

func (ref *PublicKey) String() string {
	return strings.ToUpper(hex.EncodeToString(ref.Raw))
}
