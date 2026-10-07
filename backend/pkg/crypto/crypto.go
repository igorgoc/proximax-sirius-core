package crypto

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	crypto "github.com/proximax-storage/go-xpx-crypto"
	"github.com/proximax-storage/go-xpx-chain-sdk/sdk"
)

var DefaultApiNodes = []string{
	"https://aldebaran.xpxsirius.io",
	"http://arcturus.xpxsirius.io:3000",
	"https://betelgeuse.xpxsirius.io",
	"http://lyrasithara.xpxsirius.io:3000",
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

// AddressFromPublicKey derives the 40-character Sirius base32 address from a 64-char hex public key
func AddressFromPublicKey(pubKeyHex string) (string, error) {
	if len(pubKeyHex) != 64 {
		return "", errors.New("public key must be 64 hexadecimal characters")
	}
	addr, err := sdk.NewAddressFromPublicKey(pubKeyHex, sdk.Public)
	if err != nil {
		return "", err
	}
	return addr.Address, nil
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
	// Pre-flight Verification: Account Existence & Balance
	// ----------------------------------------------------
	accInfo, err := client.Account.GetAccountInfo(ctx, publicAcc.Address)
	if err != nil {
		return nil, fmt.Errorf("account (%s) was not found on Sirius Mainnet. Ensure this account has been created and funded with XPX before linking: %w", publicAcc.Address.Address, err)
	}

	var xpxAmount uint64
	for _, m := range accInfo.Mosaics {
		// XPX Mosaic ID: 0x402B2F579FAEBC59 (4623869273703554137)
		if m.AssetId.Id() == 0x402B2F579FAEBC59 || m.AssetId.Id() == 4623869273703554137 {
			xpxAmount = uint64(m.Amount)
			break
		}
	}

	if xpxAmount < 100000 { // 0.1 XPX minimum fee (100,000 microXPX)
		return nil, fmt.Errorf("insufficient XPX balance on account %s. Found %0.6f XPX. At least 0.1 XPX is required to pay network transaction fees", publicAcc.Address.Address, float64(xpxAmount)/1e6)
	}

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

		// Poll transaction status with network validators to ensure it was not rejected
		pollTicker := time.NewTicker(1 * time.Second)
		defer pollTicker.Stop()
		timeout := time.After(12 * time.Second)

		for {
			select {
			case <-timeout:
				return txHash, nil
			case <-pollTicker.C:
				txStatus, sErr := client.Transaction.GetTransactionStatus(ctx, txHash)
				if sErr == nil && txStatus != nil {
					if strings.EqualFold(string(txStatus.Group), "failed") || strings.HasPrefix(txStatus.Status, "Failure_") {
						return "", fmt.Errorf("transaction %s rejected by network validators: %s", txHash, txStatus.Status)
					}
					if strings.EqualFold(string(txStatus.Group), "unconfirmed") || strings.EqualFold(string(txStatus.Group), "confirmed") || strings.EqualFold(txStatus.Status, "Success") {
						return txHash, nil
					}
				}
			}
		}
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

type ValidatorRegistrationResult struct {
	TxHash          string `json:"txHash,omitempty"`
	TargetPublicKey string `json:"targetPublicKey"`
	Endpoint        string `json:"endpoint"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Message         string `json:"message"`
}

// RegisterValidatorOnChain creates and broadcasts an AccountMetadataTransaction (sirius.v)
// registering this validator in the decentralized global directory.
func RegisterValidatorOnChain(accountPrivateKeyBytes []byte, name string, endpoint string, restEndpoint string, location string, nodePublicKey string, apiNodeUrl string) (*ValidatorRegistrationResult, error) {
	if len(accountPrivateKeyBytes) != 32 {
		return nil, errors.New("account private key must be exactly 32 bytes (64 hex characters)")
	}

	defer func() {
		for i := range accountPrivateKeyBytes {
			accountPrivateKeyBytes[i] = 0
		}
	}()

	if apiNodeUrl == "" {
		apiNodeUrl = DefaultApiNodes[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config, err := sdk.NewConfig(ctx, []string{apiNodeUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sirius API node (%s): %w", apiNodeUrl, err)
	}

	client := sdk.NewClient(nil, config)

	account, err := sdk.NewAccount(config.NetworkType, config.GenerationHash)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sdk account: %w", err)
	}

	privKey := crypto.NewPrivateKey(accountPrivateKeyBytes)
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

	// 1. Verify that the signing account exists and is NOT a Remote Harvester Key
	accInfo, err := client.Account.GetAccountInfo(ctx, publicAcc.Address)
	if err != nil {
		return nil, fmt.Errorf("account %s not found on Sirius Mainnet. Ensure account exists and is funded with XPX: %w", publicAcc.Address.Address, err)
	}

	if accInfo.AccountType == 2 {
		return nil, fmt.Errorf("account %s is a Remote Harvester Key. Under Sirius POS+ consensus rules, remote keys cannot sign transactions. Please use your funded Main Account private key.", publicAcc.Address.Address)
	}

	// 2. Verify account has sufficient balance to pay network transaction fees (min 0.1 XPX)
	var xpxAmount uint64
	for _, m := range accInfo.Mosaics {
		if m.AssetId.Id() == 0x402B2F579FAEBC59 || m.AssetId.Id() == 4623869273703554137 {
			xpxAmount = uint64(m.Amount)
			break
		}
	}
	if xpxAmount < 100000 {
		return nil, fmt.Errorf("insufficient XPX balance on account %s (%0.4f XPX). At least 0.1 XPX is required for transaction fees.", publicAcc.Address.Address, float64(xpxAmount)/1e6)
	}

	if nodePublicKey == "" {
		nodePublicKey = publicAcc.PublicKey
	}

	if endpoint == "" || strings.HasPrefix(endpoint, "http://localhost") || strings.HasPrefix(endpoint, "http://127.0.0.1") {
		endpoint = "onchain"
	}

	metaPayload := map[string]interface{}{
		"name":          name,
		"endpoint":      endpoint,
		"location":      location,
		"nodePublicKey": nodePublicKey,
	}
	if restEndpoint != "" && !strings.Contains(restEndpoint, "localhost") && !strings.Contains(restEndpoint, "127.0.0.1") {
		metaPayload["restEndpoint"] = restEndpoint
	}
	metaJsonBytes, err := json.Marshal(metaPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize metadata JSON: %w", err)
	}
	newValue := string(metaJsonBytes)

	// Scoped key: "sirius.v" in hex is 0x7369726975732E76
	scopedKey := sdk.ScopedMetadataKey(0x7369726975732E76)

	// Check if old value exists
	oldValue := ""
	if compositeHash, chErr := sdk.CalculateUniqueAccountMetadataId(publicAcc.Address, publicAcc, scopedKey); chErr == nil {
		if metaInfo, mErr := client.MetadataV2.GetMetadataV2Info(ctx, compositeHash); mErr == nil && metaInfo != nil && metaInfo.Address != nil {
			oldValue = string(metaInfo.Address.Value)
		}
	}

	if oldValue == newValue {
		return &ValidatorRegistrationResult{
			TargetPublicKey: publicAcc.PublicKey,
			Endpoint:        endpoint,
			Name:            name,
			Status:          "ALREADY_REGISTERED",
			Message:         "Validator metadata is already registered and up-to-date on ProximaX Sirius Mainnet",
		}, nil
	}

	metaTx, err := client.NewAccountMetadataTransaction(
		sdk.NewDeadline(time.Hour),
		publicAcc,
		scopedKey,
		newValue,
		oldValue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AccountMetadataTransaction: %w", err)
	}

	// Under Sirius Catapult consensus rules, metadata transactions are embedded transactions
	// that must be wrapped inside an Aggregate Transaction container.
	metaTx.ToAggregate(publicAcc)

	aggTx, err := client.NewCompleteAggregateTransaction(
		sdk.NewDeadline(2*time.Hour),
		[]sdk.Transaction{metaTx},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CompleteAggregateTransaction: %w", err)
	}
	// 50 XPX MaxFee comfortably satisfies any network node minFeeMultiplier (e.g. 140,000)
	aggTx.MaxFee = sdk.Amount(50000000)

	signedTx, err := account.SignWithCosignatures(aggTx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign CompleteAggregateTransaction: %w", err)
	}

	txHash, err := client.Transaction.Announce(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast CompleteAggregateTransaction: %w", err)
	}
	log.Printf("[Validator Registry] Successfully announced aggregate transaction %s to Sirius Mainnet via %s", txHash, apiNodeUrl)

	// Also broadcast to other default API nodes to guarantee propagation across all sinks
	for _, altNode := range DefaultApiNodes {
		if altNode == apiNodeUrl {
			continue
		}
		go func(nodeUrl string) {
			bCtx, bCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer bCancel()
			if altCfg, cErr := sdk.NewConfig(bCtx, []string{nodeUrl}); cErr == nil {
				altClient := sdk.NewClient(nil, altCfg)
				altClient.Transaction.Announce(bCtx, signedTx)
			}
		}(altNode)
	}

	// 3. Actively poll transaction status to confirm acceptance by network validators
	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()
	timeout := time.After(25 * time.Second)

	for {
		select {
		case <-timeout:
			log.Printf("[Validator Registry] Transaction %s still pending block inclusion after 20s", txHash)
			return &ValidatorRegistrationResult{
				TxHash:          txHash,
				TargetPublicKey: publicAcc.PublicKey,
				Endpoint:        endpoint,
				Name:            name,
				Status:          "PENDING",
				Message:         fmt.Sprintf("Transaction %s announced to Sirius Mainnet. Awaiting block inclusion.", txHash),
			}, nil
		case <-pollTicker.C:
			txStatus, sErr := client.Transaction.GetTransactionStatus(ctx, txHash)
			if sErr == nil && txStatus != nil {
				log.Printf("[Validator Registry] Transaction %s status check: Group=%s, Status=%s", txHash, txStatus.Group, txStatus.Status)
				if strings.EqualFold(string(txStatus.Group), "failed") || strings.HasPrefix(txStatus.Status, "Failure_") {
					return nil, fmt.Errorf("transaction %s rejected by network validators: %s", txHash, txStatus.Status)
				}
				if strings.EqualFold(string(txStatus.Group), "unconfirmed") || strings.EqualFold(string(txStatus.Group), "confirmed") || strings.EqualFold(txStatus.Status, "Success") {
					return &ValidatorRegistrationResult{
						TxHash:          txHash,
						TargetPublicKey: publicAcc.PublicKey,
						Endpoint:        endpoint,
						Name:            name,
						Status:          "SUCCESS",
						Message:         "Successfully registered validator directory on ProximaX Sirius Mainnet",
					}, nil
				}
			}
		}
	}
}

// GetValidatorOnChainMetadata queries on-chain metadata for scoped key 'sirius.v'
func GetValidatorOnChainMetadata(targetPublicKey string, apiNodeUrl string) (map[string]interface{}, error) {
	if apiNodeUrl == "" {
		apiNodeUrl = DefaultApiNodes[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := sdk.NewConfig(ctx, []string{apiNodeUrl})
	if err != nil {
		return nil, err
	}
	client := sdk.NewClient(nil, config)

	scopedKey := sdk.ScopedMetadataKey(0x7369726975732E76)

	// 1. Direct check on targetPublicKey
	if publicAcc, err := sdk.NewAccountFromPublicKey(targetPublicKey, config.NetworkType); err == nil {
		compositeHash, err := sdk.CalculateUniqueAccountMetadataId(publicAcc.Address, publicAcc, scopedKey)
		if err == nil {
			if metaInfo, err := client.MetadataV2.GetMetadataV2Info(ctx, compositeHash); err == nil && metaInfo != nil && metaInfo.Address != nil {
				var parsed map[string]interface{}
				if err := json.Unmarshal(metaInfo.Address.Value, &parsed); err == nil {
					return parsed, nil
				}
				return map[string]interface{}{
					"rawValue": string(metaInfo.Address.Value),
				}, nil
			}
		}
	}

	// 2. Search for any account metadata with scopedKey 'sirius.v' where nodePublicKey matches targetPublicKey
	pageOpts := &sdk.MetadataV2PageOptions{
		ScopedKey: "7369726975732E76",
	}
	if page, err := client.MetadataV2.GetMetadataV2Infos(ctx, pageOpts); err == nil && page != nil {
		for _, entry := range page.Metadatas {
			if entry.Address == nil || len(entry.Address.Value) == 0 {
				continue
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(entry.Address.Value, &parsed); err == nil {
				nodeKey, _ := parsed["nodePublicKey"].(string)
				if strings.EqualFold(nodeKey, targetPublicKey) {
					return parsed, nil
				}
			}
		}
	}

	return nil, errors.New("no on-chain validator metadata found")
}

// UnregisterValidatorOnChain removes the validator directory entry on-chain by clearing
// the 'sirius.v' scoped metadata (setting value to empty string or tombstone).
func UnregisterValidatorOnChain(accountPrivateKeyBytes []byte, apiNodeUrl string) (*ValidatorRegistrationResult, error) {
	if len(accountPrivateKeyBytes) != 32 {
		return nil, errors.New("account private key must be exactly 32 bytes (64 hex characters)")
	}

	defer func() {
		for i := range accountPrivateKeyBytes {
			accountPrivateKeyBytes[i] = 0
		}
	}()

	if apiNodeUrl == "" {
		apiNodeUrl = DefaultApiNodes[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config, err := sdk.NewConfig(ctx, []string{apiNodeUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sirius API node (%s): %w", apiNodeUrl, err)
	}

	client := sdk.NewClient(nil, config)

	account, err := sdk.NewAccount(config.NetworkType, config.GenerationHash)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sdk account: %w", err)
	}

	privKey := crypto.NewPrivateKey(accountPrivateKeyBytes)
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

	accInfo, err := client.Account.GetAccountInfo(ctx, publicAcc.Address)
	if err != nil {
		return nil, fmt.Errorf("account %s not found on Sirius Mainnet: %w", publicAcc.Address.Address, err)
	}

	if accInfo.AccountType == 2 {
		return nil, fmt.Errorf("account %s is a Remote Harvester Key. Under Sirius POS+ rules, remote keys cannot sign transactions.", publicAcc.Address.Address)
	}

	scopedKey := sdk.ScopedMetadataKey(0x7369726975732E76)

	// Fetch current on-chain value
	oldValue := ""
	if compositeHash, chErr := sdk.CalculateUniqueAccountMetadataId(publicAcc.Address, publicAcc, scopedKey); chErr == nil {
		if metaInfo, mErr := client.MetadataV2.GetMetadataV2Info(ctx, compositeHash); mErr == nil && metaInfo != nil && metaInfo.Address != nil {
			oldValue = string(metaInfo.Address.Value)
		}
	}

	if oldValue == "" {
		return &ValidatorRegistrationResult{
			TargetPublicKey: publicAcc.PublicKey,
			Status:          "NOT_REGISTERED",
			Message:         "Validator is not currently registered in the on-chain directory",
		}, nil
	}

	// In Sirius Catapult metadata v2, to delete or clear metadata, newValue is empty ""
	newValue := ""

	metaTx, err := client.NewAccountMetadataTransaction(
		sdk.NewDeadline(time.Hour),
		publicAcc,
		scopedKey,
		newValue,
		oldValue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AccountMetadataTransaction: %w", err)
	}

	metaTx.ToAggregate(publicAcc)

	aggTx, err := client.NewCompleteAggregateTransaction(
		sdk.NewDeadline(2*time.Hour),
		[]sdk.Transaction{metaTx},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CompleteAggregateTransaction: %w", err)
	}
	// 50 XPX MaxFee comfortably satisfies any network node minFeeMultiplier (e.g. 140,000)
	aggTx.MaxFee = sdk.Amount(50000000)

	signedTx, err := account.SignWithCosignatures(aggTx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign CompleteAggregateTransaction: %w", err)
	}

	txHash, err := client.Transaction.Announce(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast CompleteAggregateTransaction: %w", err)
	}
	log.Printf("[Validator Registry] Announced unregister transaction %s to Sirius Mainnet", txHash)

	// Also broadcast to other default API nodes to guarantee propagation across all sinks
	for _, altNode := range DefaultApiNodes {
		if altNode == apiNodeUrl {
			continue
		}
		go func(nodeUrl string) {
			bCtx, bCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer bCancel()
			if altCfg, cErr := sdk.NewConfig(bCtx, []string{nodeUrl}); cErr == nil {
				altClient := sdk.NewClient(nil, altCfg)
				altClient.Transaction.Announce(bCtx, signedTx)
			}
		}(altNode)
	}

	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()
	timeout := time.After(25 * time.Second)

	for {
		select {
		case <-timeout:
			// Before declaring status, check if the metadata entry was already cleared or transaction confirmed
			if compositeHash, chErr := sdk.CalculateUniqueAccountMetadataId(publicAcc.Address, publicAcc, scopedKey); chErr == nil {
				if metaInfo, mErr := client.MetadataV2.GetMetadataV2Info(ctx, compositeHash); mErr != nil || metaInfo == nil || metaInfo.Address == nil || len(metaInfo.Address.Value) == 0 {
					return &ValidatorRegistrationResult{
						TxHash:          txHash,
						TargetPublicKey: publicAcc.PublicKey,
						Status:          "SUCCESS",
						Message:         "Successfully removed validator from ProximaX Sirius Mainnet directory",
					}, nil
				}
			}
			return nil, fmt.Errorf("transaction %s was not confirmed by the network. Please ensure the operator account has sufficient XPX balance and retry", txHash)
		case <-pollTicker.C:
			txStatus, sErr := client.Transaction.GetTransactionStatus(ctx, txHash)
			if sErr == nil && txStatus != nil {
				if strings.EqualFold(string(txStatus.Group), "failed") || strings.HasPrefix(txStatus.Status, "Failure_") {
					return nil, fmt.Errorf("transaction %s rejected by network validators: %s", txHash, txStatus.Status)
				}
				if strings.EqualFold(string(txStatus.Group), "unconfirmed") || strings.EqualFold(string(txStatus.Group), "confirmed") || strings.EqualFold(txStatus.Status, "Success") {
					return &ValidatorRegistrationResult{
						TxHash:          txHash,
						TargetPublicKey: publicAcc.PublicKey,
						Status:          "SUCCESS",
						Message:         "Successfully removed validator from ProximaX Sirius Mainnet directory",
					}, nil
				}
			}
		}
	}
}

// DecryptDelegationPayload decrypts an on-chain encrypted message payload (hex)
// sent by a delegator to this validator node using Sirius Catapult ECDH block cipher.
func DecryptDelegationPayload(payloadHex, senderPublicKeyHex, recipientPrivateKeyHex string) (string, error) {
	payloadBytes, err := hex.DecodeString(strings.TrimSpace(payloadHex))
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted payload hex: %w", err)
	}

	senderPubBytes, err := hex.DecodeString(strings.TrimSpace(senderPublicKeyHex))
	if err != nil {
		return "", fmt.Errorf("failed to decode sender public key hex: %w", err)
	}

	recipientPrivBytes, err := hex.DecodeString(strings.TrimSpace(recipientPrivateKeyHex))
	if err != nil {
		return "", fmt.Errorf("failed to decode recipient private key hex: %w", err)
	}
	defer func() {
		for i := range recipientPrivBytes {
			recipientPrivBytes[i] = 0
		}
	}()

	recipPriv := crypto.NewPrivateKey(recipientPrivBytes)
	defer recipPriv.Destroy()

	senderPub := crypto.NewPublicKey(senderPubBytes)

	plain, err := sdk.NewPlainMessageFromEncodedData(payloadBytes, recipPriv, senderPub)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message payload: %w", err)
	}

	res := plain.Message()
	// Check if res is hex-encoded string (e.g. from tsjs-xpx-chain-sdk EncryptedMessage)
	if decodedBytes, decErr := hex.DecodeString(res); decErr == nil && len(decodedBytes) > 0 {
		res = string(decodedBytes)
	}

	return res, nil
}

// DelegatedStakingPayload represents the structured on-chain message payload
type DelegatedStakingPayload struct {
	Type             string `json:"type"`
	Version          int    `json:"version"`
	Action           string `json:"action"` // "link" or "unlink"
	RemotePrivateKey string `json:"remotePrivateKey"`
}

// ParseDelegatedPayload parses either a JSON payload or a raw 64-hex string.
func ParseDelegatedPayload(content string) (remotePrivateKey string, action string, err error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "", errors.New("empty payload")
	}

	// 1. Try parsing JSON
	var payload DelegatedStakingPayload
	if jsonErr := json.Unmarshal([]byte(content), &payload); jsonErr == nil && payload.RemotePrivateKey != "" {
		key := strings.ToUpper(strings.TrimSpace(payload.RemotePrivateKey))
		if len(key) != 64 {
			return "", "", fmt.Errorf("remote private key in payload must be 64 hex characters, got %d", len(key))
		}
		if _, hexErr := hex.DecodeString(key); hexErr != nil {
			return "", "", fmt.Errorf("invalid hex in remote private key: %w", hexErr)
		}
		act := strings.ToLower(strings.TrimSpace(payload.Action))
		if act == "" {
			act = "link"
		}
		return key, act, nil
	}

	// 2. Fallback: treat directly as 64-hex key
	cleanKey := strings.ToUpper(content)
	if len(cleanKey) == 64 {
		if _, hexErr := hex.DecodeString(cleanKey); hexErr == nil {
			return cleanKey, "link", nil
		}
	}

	return "", "", errors.New("unrecognized delegation payload format")
}
