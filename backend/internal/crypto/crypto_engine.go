// Copyright 2018 ProximaX Limited. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package crypto

// CryptoEngine represents a cryptographic engine that is a factory of crypto-providers.
type CryptoEngine interface {
	// Creates a DSA signer.
	CreateDsaSigner(keyPair *KeyPair) DsaSigner
	// Creates a key generator.
	CreateKeyGenerator() KeyGenerator
	// Creates a block cipher.
	CreateBlockCipher(senderKeyPair *KeyPair, recipientKeyPair *KeyPair) BlockCipher
	// Creates a key analyzer.
	CreateKeyAnalyzer() KeyAnalyzer
}

// cryptoEngines Static class that exposes crypto engines.
type cryptoEngines struct {
	Ed25519Engine *Ed25519SeedCryptoEngine
	DefaultEngine *Ed25519SeedCryptoEngine
}

// CryptoEngines has cryptographic engines
var CryptoEngines = cryptoEngines{
	&Ed25519SeedCryptoEngine{nil},
	&Ed25519SeedCryptoEngine{nil},
}
