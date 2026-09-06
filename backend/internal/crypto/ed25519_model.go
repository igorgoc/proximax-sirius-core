// Copyright 2018 ProximaX Limited. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Ed25519SeedCryptoEngine wraps a cryptographic engine ed25519 and seed for this engine
type Ed25519SeedCryptoEngine struct {
	seed io.Reader
}

// CreateDsaSigner implemented interface CryptoEngine method
func (ref *Ed25519SeedCryptoEngine) CreateDsaSigner(keyPair *KeyPair) DsaSigner {
	return NewEd25519DsaSigner(keyPair)
}

// CreateKeyGenerator implemented interface CryptoEngine method
func (ref *Ed25519SeedCryptoEngine) CreateKeyGenerator() KeyGenerator {
	return NewEd25519KeyGenerator(ref.seed)
}

// CreateBlockCipher implemented interface CryptoEngine method
func (ref *Ed25519SeedCryptoEngine) CreateBlockCipher(senderKeyPair *KeyPair, recipientKeyPair *KeyPair) BlockCipher {
	return NewEd25519BlockCipher(senderKeyPair, recipientKeyPair, ref.seed)
}

// CreateKeyAnalyzer implemented interface CryptoEngine method
func (ref *Ed25519SeedCryptoEngine) CreateKeyAnalyzer() KeyAnalyzer {
	return NewEd25519KeyAnalyzer()
}

// Ed25519BlockCipher Implementation of the block cipher for Ed25519.
type Ed25519BlockCipher struct {
	senderKeyPair    *KeyPair
	recipientKeyPair *KeyPair
	keyLength        int
	seed             io.Reader
}

// NewEd25519BlockCipher return Ed25519BlockCipher
func NewEd25519BlockCipher(senderKeyPair *KeyPair, recipientKeyPair *KeyPair, seed io.Reader) *Ed25519BlockCipher {
	if seed == nil {
		seed = rand.Reader
	}

	ref := Ed25519BlockCipher{
		senderKeyPair,
		recipientKeyPair,
		len(recipientKeyPair.PublicKey.Raw),
		seed,
	}
	return &ref
}

func (ref *Ed25519BlockCipher) encode(message []byte, sharedKey []byte, ivData []byte) ([]byte, error) {
	c, err := aes.NewCipher(sharedKey)

	if err != nil {
		return nil, err
	}

	messageSize := len(message)
	blockSize := c.BlockSize()
	paddingSize := blockSize - (messageSize % blockSize)
	bufferSize := messageSize + paddingSize

	buf := make([]byte, bufferSize)
	copy(buf[:messageSize], message)

	for i := 0; i < paddingSize; i++ {
		buf[messageSize+i] = uint8(paddingSize)
	}

	enc := cipher.NewCBCEncrypter(c, ivData)
	ciphertext := make([]byte, len(buf))
	enc.CryptBlocks(ciphertext, buf)

	return ciphertext, nil
}

func (ref *Ed25519BlockCipher) decode(ciphertext []byte, sharedKey []byte, ivData []byte) ([]byte, error) {
	c, err := aes.NewCipher(sharedKey)

	if err != nil {
		return nil, err
	}

	dec := cipher.NewCBCDecrypter(c, ivData)
	buf := make([]byte, len(ciphertext))
	dec.CryptBlocks(buf, ciphertext)

	bufferSize := len(buf)
	paddingSize := int(buf[bufferSize-1] & 0xFF)

	if paddingSize == 0 || paddingSize > c.BlockSize() {
		return nil, errors.New("blocks are corrupted, paddingSize is wrong")
	}

	messageSize := bufferSize - paddingSize

	for i := messageSize; i < bufferSize; i++ {
		if int(buf[i]) != paddingSize {
			return nil, errors.New("blocks are corrupted, fake byte is not equal to paddingSize")
		}
	}

	return buf[:messageSize], nil
}

// GetSharedKey create shared bytes
func (ref *Ed25519BlockCipher) GetSharedKey(privateKey *PrivateKey, publicKey *PublicKey, salt []byte) ([]byte, error) {

	grA, err := NewEd25519EncodedGroupElement(publicKey.Raw)
	if err != nil {
		return nil, err
	}
	senderA, err := grA.Decode()
	if err != nil {
		return nil, err
	}
	senderA.PrecomputeForScalarMultiplication()
	el, err := senderA.scalarMultiply(PrepareForScalarMultiply(privateKey))
	if err != nil {
		return nil, err
	}
	sharedKey, err := el.Encode()
	if err != nil {
		return nil, err
	}
	for i := 0; i < ref.keyLength; i++ {
		sharedKey.Raw[i] ^= salt[i]
	}

	return HashesSha3_256(sharedKey.Raw)
}

// Encrypt slice byte
func (ref *Ed25519BlockCipher) Encrypt(input []byte) ([]byte, error) {
	// Setup salt.
	salt := make([]byte, ref.keyLength)
	_, err := io.ReadFull(ref.seed, salt)
	if err != nil {
		return nil, err
	}

	// Derive shared key.
	sharedKey, err := ref.GetSharedKey(ref.senderKeyPair.PrivateKey, ref.recipientKeyPair.PublicKey, salt)
	if err != nil {
		return nil, err
	}
	// Setup IV.
	ivData := make([]byte, 16)
	_, err = io.ReadFull(ref.seed, ivData)
	if err != nil {
		return nil, err
	}
	// Encode.
	buf, err := ref.encode(input, sharedKey, ivData)
	if err != nil {
		return nil, err
	}

	result := append(append(salt, ivData...), buf...)

	return result, nil
}

// Decrypt slice byte
func (ref *Ed25519BlockCipher) Decrypt(input []byte) ([]byte, error) {
	if len(input) < 64 {
		return nil, errors.New("input is to short for decryption")
	}

	salt := input[:ref.keyLength]
	ivData := input[ref.keyLength:48]
	encData := input[48:]
	// Derive shared key.
	sharedKey, err := ref.GetSharedKey(ref.recipientKeyPair.PrivateKey, ref.senderKeyPair.PublicKey, salt)
	if err != nil {
		return nil, err
	}
	// Decode.
	return ref.decode(encData, sharedKey, ivData)
}

// Ed25519DsaSigner implement DsaSigned interface with Ed25519 algo
type Ed25519DsaSigner struct {
	KeyPair *KeyPair
}

// NewEd25519DsaSigner creates a Ed25519 DSA signer.
func NewEd25519DsaSigner(keyPair *KeyPair) *Ed25519DsaSigner {
	return &Ed25519DsaSigner{keyPair}
}

// Sign message
func (ref *Ed25519DsaSigner) Sign(mess []byte) (*Signature, error) {

	if !ref.KeyPair.HasPrivateKey() {
		return nil, errors.New("cannot sign without private key")
	}

	// Hash the private key to improve randomness.
	hash, err := HashesSha3_512(ref.KeyPair.PrivateKey.Raw)
	if err != nil {
		return nil, err
	}
	// r = H(hash_b,...,hash_2b-1, data) where b=256.
	hashR, err := HashesSha3_512(
		hash[32:], // only include the last 32 bytes of the private key hash
		mess)
	if err != nil {
		return nil, err
	}
	r, err := NewEd25519EncodedFieldElement(hashR)
	if err != nil {
		return nil, err
	}
	// Reduce size of r since we are calculating mod group order anyway
	rModQ := r.modQ()
	// R = rModQ * base point.
	R, err := Ed25519Group.BASE_POINT().scalarMultiply(rModQ)
	if err != nil {
		return nil, err
	}
	encodedR, err := R.Encode()
	if err != nil {
		return nil, err
	}
	// S = (r + H(encodedR, encodedA, data) * a) mod group order where
	// encodedR and encodedA are the little endian encodings of the group element R and the public key A and
	// a is the lower 32 bytes of hash after clamping.
	hashH, err := HashesSha3_512(
		encodedR.Raw,
		ref.KeyPair.PublicKey.Raw,
		mess)
	if err != nil {
		return nil, err
	}
	h, err := NewEd25519EncodedFieldElement(hashH)
	if err != nil {
		return nil, err
	}
	hModQ := h.modQ()
	encodedS := hModQ.multiplyAndAddModQ(PrepareForScalarMultiply(ref.KeyPair.PrivateKey),
		rModQ)
	// Signature is (encodedR, encodedS)
	signature, err := NewSignature(encodedR.Raw, encodedS.Raw)
	if err != nil {
		return nil, err
	}
	if !ref.IsCanonicalSignature(signature) {
		return nil, errors.New("Generated signature is not canonical")
	}

	return signature, nil
}

// Verify reports whether sig is a valid signature of message 'data' by publicKey. It
// prevent  panic inside ed25519.Verify
func (ref *Ed25519DsaSigner) Verify(mess []byte, signature *Signature) (res bool) {

	if !ref.IsCanonicalSignature(signature) {
		return false
	}

	if isEqualConstantTime(ref.KeyPair.PublicKey.Raw, make([]byte, 32)) {
		return false
	}

	// h = H(encodedR, encodedA, data).
	rawEncodedR := signature.R
	rawEncodedA := ref.KeyPair.PublicKey.Raw
	hashR, err := HashesSha3_512(
		rawEncodedR,
		rawEncodedA,
		mess)
	if err != nil {
		fmt.Println(err)
		return false
	}
	h, err := NewEd25519EncodedFieldElement(hashR)
	if err != nil {
		fmt.Println(err)
		return false
	}
	// hReduced = h mod group order
	hModQ := h.modQ()
	// Must compute A.
	A, err := (&Ed25519EncodedGroupElement{rawEncodedA}).Decode()
	if err != nil {
		fmt.Println(err)
		return false
	}
	A.PrecomputeForDoubleScalarMultiplication()
	// R = encodedS * B - H(encodedR, encodedA, data) * A
	calculatedR, err := Ed25519Group.BASE_POINT().doubleScalarMultiplyVariableTime(
		A,
		hModQ,
		&Ed25519EncodedFieldElement{Ed25519FieldZeroShort(), signature.S})
	if err != nil {
		fmt.Println(err)
		return false
	}
	// Compare calculated R to given R.
	encodedCalculatedR, err := calculatedR.Encode()
	if err != nil {
		fmt.Println(err)
		return false
	}

	return isEqualConstantTime(encodedCalculatedR.Raw, rawEncodedR)
}

// IsCanonicalSignature check signature on canonical
func (ref *Ed25519DsaSigner) IsCanonicalSignature(signature *Signature) bool {

	sgnS := signature.GetS().Uint64()
	return sgnS != Ed25519Group.GROUP_ORDER.Uint64() && sgnS > 0
}

// MakeSignatureCanonical return canonical signature
func (ref *Ed25519DsaSigner) MakeSignatureCanonical(signature *Signature) (*Signature, error) {

	sign := make([]byte, 64)
	copy(sign, signature.S)
	s, err := NewEd25519EncodedFieldElement(sign)
	if err != nil {
		return nil, err
	}
	sModQ := s.modQ()
	return NewSignature(signature.R, sModQ.Raw)
}

// Ed25519KeyGenerator Implementation of the key generator for Ed25519.
type Ed25519KeyGenerator struct {
	seed io.Reader
}

// NewEd25519KeyGenerator return new Ed25519KeyGenerator
func NewEd25519KeyGenerator(seed io.Reader) *Ed25519KeyGenerator {
	if seed == nil {
		seed = rand.Reader
	}

	ref := Ed25519KeyGenerator{seed}
	return &ref
}

// GenerateKeyPair generate key pair use ed25519.GenerateKey
func (ref *Ed25519KeyGenerator) GenerateKeyPair() (*KeyPair, error) {
	seed := make([]byte, 32)
	_, err := io.ReadFull(ref.seed, seed[:])
	if err != nil {
		return nil, err
	} // seed is the private key.

	// seed is the private key.
	privateKey := NewPrivateKey(seed)
	publicKey := ref.DerivePublicKey(privateKey)
	return NewKeyPair(privateKey, publicKey, CryptoEngines.Ed25519Engine)
}

// DerivePublicKey return public key based on Ed25519Group.BASE_POINT
func (ref *Ed25519KeyGenerator) DerivePublicKey(privateKey *PrivateKey) *PublicKey {

	a := PrepareForScalarMultiply(privateKey)
	// a * base point is the public key.
	pubKey, err := Ed25519Group.BASE_POINT().scalarMultiply(a)
	if err != nil {
		panic(err)
	}
	el, _ := pubKey.Encode()
	return NewPublicKey(el.Raw)
}
