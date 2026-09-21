// Copyright 2018 ProximaX Limited. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package crypto

// NewBlockCipher creates a block cipher around a sender KeyPair and recipient KeyPair.
// if engine is nil - use CryptoEngines.DefaultEngine instead
// The sender KeyPair. The sender'S private key is required for encryption.
// The recipient KeyPair. The recipient'S private key is required for decryption.
func NewBlockCipher(senderKeyPair *KeyPair, recipientKeyPair *KeyPair, engine CryptoEngine) BlockCipher {
	if engine == nil {
		engine = CryptoEngines.DefaultEngine
	}
	return engine.CreateBlockCipher(senderKeyPair, recipientKeyPair)
}

// BlockCipher Interface for encryption and decryption of data.
type BlockCipher interface {
	// Encrypts an arbitrarily-sized message (input).
	Encrypt(input []byte) ([]byte, error)
	// Decrypts an arbitrarily-sized message.
	Decrypt(input []byte) ([]byte, error)
}
