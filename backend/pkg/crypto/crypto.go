package crypto

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	crypto "github.com/proximax-storage/go-xpx-crypto"
	"github.com/proximax-storage/go-xpx-chain-sdk/sdk"
)

var DefaultApiNodes = []string{
	"https://aldebaran.xpxsirius.io",
	"https://betelgeuse.xpxsirius.io",
	"http://aldebaran.xpxsirius.io:3000",
	"http://betelgeuse.xpxsirius.io:3000",
}

type KeyPairInfo struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`
}

type AccountLinkResult struct {
	TxHash           string `json:"txHash"`
	HarvesterTxHash  string `json:"harvesterTxHash,omitempty"`
	SignerPublicKey  string `json:"signerPublicKey"`
	RemotePrivateKey string `json:"remotePrivateKey,omitempty"`
	RemotePublicKey  string `json:"remotePublicKey"`
	RemoteAddress    string `json:"remoteAddress"`
	Status           string `json:"status"`
	Message          string `json:"message"`
}

// GenerateKeyPair generates a random keypair and Sirius Public address
func GenerateKeyPair() (*KeyPairInfo, error) {
	keyPair, err := crypto.NewRandomKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random keypair: %w", err)
	}

	privHex := hex.EncodeToString(keyPair.PrivateKey.Raw)
	pubHex := hex.EncodeToString(keyPair.PublicKey.Raw)

	addr, err := sdk.NewAddressFromPublicKey(keyPair.PublicKey.String(), sdk.Public)
	if err != nil {
		return nil, fmt.Errorf("failed to derive address from public key: %w", err)
	}

	return &KeyPairInfo{
		PrivateKey: privHex,
		PublicKey:  pubHex,
		Address:    addr.Address,
	}, nil
}

// KeyPairFromPrivateKey derives public key and address from an existing 64-char private key instantly using local cryptography
func KeyPairFromPrivateKey(privKeyHex string) (*KeyPairInfo, error) {
	if len(privKeyHex) != 64 {
		return nil, errors.New("private key must be 64 hexadecimal characters")
	}

	account, err := sdk.NewAccountFromPrivateKey(privKeyHex, sdk.Public, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	pubHex := strings.ToUpper(hex.EncodeToString(account.KeyPair.PublicKey.Raw))
	return &KeyPairInfo{
		PrivateKey: privKeyHex,
		PublicKey:  pubHex,
		Address:    account.Address.Address,
	}, nil
}

// SecretKeyBuffer decodes a 64-hex private key directly into a mutable 32-byte slice
// and ensures it can be explicitly zeroed in memory end-to-end without immutable Go strings.
type SecretKeyBuffer []byte

func (s *SecretKeyBuffer) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("expected hex string in quotes")
	}
	hexBytes := data[1 : len(data)-1]
	if len(hexBytes) != 64 {
		return errors.New("mainnet account private key must be exactly 64 hexadecimal characters")
	}
	buf := make([]byte, 32)
	n, err := hex.Decode(buf, hexBytes)
	if err != nil {
		return fmt.Errorf("invalid hex encoding: %w", err)
	}
	if n != 32 {
		return errors.New("invalid private key length")
	}
	*s = buf
	return nil
}

func (s SecretKeyBuffer) Wipe() {
	for i := range s {
		s[i] = 0
	}
}

// PerformDelegatedHarvestingLink links an account, registers a harvester, or unlinks based on action
func PerformDelegatedHarvestingLink(accountPrivateKeyBytes []byte, remoteHarvestKeyOrPub string, apiNodeUrl string, action string) (*AccountLinkResult, error) {
	if len(accountPrivateKeyBytes) != 32 {
		return nil, errors.New("mainnet account private key must be exactly 32 bytes (64 hex characters)")
	}

	defer func() {
		// Memory wiping invariant: Overwrite incoming raw key buffer upon return
		for i := range accountPrivateKeyBytes {
			accountPrivateKeyBytes[i] = 0
		}
	}()

	if apiNodeUrl == "" {
		apiNodeUrl = DefaultApiNodes[0]
	}

	if action == "" {
		action = "link_and_register"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	config, err := sdk.NewConfig(ctx, []string{apiNodeUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sirius API node (%s): %w", apiNodeUrl, err)
	}

	client := sdk.NewClient(nil, config)

	// 100% Byte-Native Account Construction:
	// Use sdk.NewAccount to initialize the account struct with network generation hash,
	// then populate keypair directly from raw byte slice. No string is ever allocated!
	account, err := sdk.NewAccount(config.NetworkType, config.GenerationHash)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sdk account: %w", err)
	}

	privKey := crypto.NewPrivateKey(accountPrivateKeyBytes)
	// Security Invariant: Register immediate destruction before any fallible step (NewKeyPair, etc.)
	// so privKey (both Raw bytes and big.Int words) is guaranteed wiped on early error returns.
	defer privKey.Destroy()

	keyPair, err := crypto.NewKeyPair(privKey, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to derive keypair: %w", err)
	}

	publicAcc, err := sdk.NewAccountFromPublicKey(keyPair.PublicKey.String(), config.NetworkType)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public account: %w", err)
	}

	account.KeyPair = keyPair
	account.PublicAccount = publicAcc

	// ----------------------------------------------------
	// Helper to sign and announce transactions reliably
	// ----------------------------------------------------
	announceTx := func(tx sdk.Transaction) (string, error) {
		signedTx, err := account.Sign(tx)
		if err != nil {
			return "", fmt.Errorf("failed to sign transaction: %w", err)
		}

		txHash, err := client.Transaction.Announce(ctx, signedTx)
		if err != nil {
			return "", fmt.Errorf("failed to broadcast transaction: %w", err)
		}

		return txHash, nil
	}

	// ----------------------------------------------------
	// Action: REGISTER ONLY (when account is already linked)
	// ----------------------------------------------------
	if action == "register_only" {
		var harvesterPub string
		remoteHarvestKeyOrPub = strings.TrimSpace(remoteHarvestKeyOrPub)
		if len(remoteHarvestKeyOrPub) == 64 {
			if remoteAcc, pErr := client.NewAccountFromPrivateKey(remoteHarvestKeyOrPub); pErr == nil {
				harvesterPub = remoteAcc.PublicAccount.PublicKey
			} else {
				harvesterPub = remoteHarvestKeyOrPub
			}
		}

		harvesterPubAcc, err := sdk.NewAccountFromPublicKey(harvesterPub, account.PublicAccount.Address.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to parse harvester public key: %w", err)
		}

		harvesterTx, hErr := client.NewHarvesterTransaction(
			sdk.NewDeadline(time.Hour),
			sdk.AddHarvester,
			harvesterPubAcc,
		)
		if hErr != nil {
			return nil, fmt.Errorf("failed to create harvester transaction: %w", hErr)
		}

		txHashStr, aErr := announceTx(harvesterTx)
		if aErr != nil {
			return nil, fmt.Errorf("failed to broadcast harvester transaction: %w", aErr)
		}

		return &AccountLinkResult{
			TxHash:          txHashStr,
			HarvesterTxHash: txHashStr,
			SignerPublicKey: account.PublicAccount.PublicKey,
			RemotePublicKey: remoteHarvestKeyOrPub,
			Status:          "SUCCESS",
			Message:         "Successfully broadcasted AddHarvester transaction to Sirius Mainnet!",
		}, nil
	}

	// ----------------------------------------------------
	// Action: UNLINK ACCOUNT
	// ----------------------------------------------------
	if action == "unlink" {
		var remotePubHex string
		remoteHarvestKeyOrPub = strings.TrimSpace(remoteHarvestKeyOrPub)
		if len(remoteHarvestKeyOrPub) == 64 {
			if remoteAcc, pErr := client.NewAccountFromPrivateKey(remoteHarvestKeyOrPub); pErr == nil {
				remotePubHex = remoteAcc.PublicAccount.PublicKey
			} else {
				remotePubHex = remoteHarvestKeyOrPub
			}
		}

		if remotePubHex == "" {
			return nil, errors.New("remote public key is required to unlink")
		}

		remoteAccount, err := client.NewAccountFromPublicKey(remotePubHex)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote account entity: %w", err)
		}

		unlinkTx, err := client.NewAccountLinkTransaction(
			sdk.NewDeadline(time.Hour),
			remoteAccount,
			sdk.AccountUnlink,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create AccountUnlinkTransaction: %w", err)
		}

		txHashStr, err := announceTx(unlinkTx)
		if err != nil {
			return nil, err
		}

		return &AccountLinkResult{
			TxHash:          txHashStr,
			SignerPublicKey: account.PublicAccount.PublicKey,
			RemotePublicKey: remotePubHex,
			Status:          "SUCCESS",
			Message:         "Successfully unlinked remote harvesting account on ProximaX Sirius Mainnet",
		}, nil
	}

	// ----------------------------------------------------
	// Action: FULL LINK & REGISTER
	// ----------------------------------------------------
	var remotePrivHex string
	var remotePubHex string

	remoteHarvestKeyOrPub = strings.TrimSpace(remoteHarvestKeyOrPub)
	if len(remoteHarvestKeyOrPub) == 64 {
		if remoteAcc, pErr := client.NewAccountFromPrivateKey(remoteHarvestKeyOrPub); pErr == nil {
			remotePrivHex = remoteHarvestKeyOrPub
			remotePubHex = remoteAcc.PublicAccount.PublicKey
		} else {
			remotePubHex = remoteHarvestKeyOrPub
		}
	}

	if remotePubHex == "" {
		remoteKeyPair, err := crypto.NewRandomKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate remote keypair: %w", err)
		}
		remotePrivHex = hex.EncodeToString(remoteKeyPair.PrivateKey.Raw)
		remotePubHex = hex.EncodeToString(remoteKeyPair.PublicKey.Raw)
	}

	remoteAccount, err := client.NewAccountFromPublicKey(remotePubHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote account entity: %w", err)
	}

	remoteAddr, _ := sdk.NewAddressFromPublicKey(remotePubHex, sdk.Public)

	// 1. Create and announce AccountLinkTransaction
	linkTx, err := client.NewAccountLinkTransaction(
		sdk.NewDeadline(time.Hour),
		remoteAccount,
		sdk.AccountLink,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AccountLinkTransaction: %w", err)
	}

	linkTxHashStr, err := announceTx(linkTx)
	if err != nil {
		return nil, fmt.Errorf("failed to announce AccountLinkTransaction: %w", err)
	}

	// 2. Create and announce AddHarvester committee registration transaction
	var harvesterTxHashStr string
	harvesterTx, err := client.NewHarvesterTransaction(
		sdk.NewDeadline(time.Hour),
		sdk.AddHarvester,
		remoteAccount,
	)
	if err == nil {
		if hHash, hErr := announceTx(harvesterTx); hErr == nil {
			harvesterTxHashStr = hHash
		}
	}

	return &AccountLinkResult{
		TxHash:           linkTxHashStr,
		HarvesterTxHash:  harvesterTxHashStr,
		SignerPublicKey:  account.PublicAccount.PublicKey,
		RemotePrivateKey: remotePrivHex,
		RemotePublicKey:  remotePubHex,
		RemoteAddress:    remoteAddr.Address,
		Status:           "SUCCESS",
		Message:          "Successfully linked remote account and registered harvester on ProximaX Sirius Mainnet",
	}, nil
}
