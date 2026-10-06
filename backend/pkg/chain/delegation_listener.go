package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"proximax-sirius-core/pkg/crypto"
)

type ConfirmedTransactionsResponse struct {
	Data []struct {
		Meta struct {
			Height [2]uint64 `json:"height"`
			Hash   string    `json:"hash"`
		} `json:"meta"`
		Transaction struct {
			Type      int    `json:"type"`
			Signer    string `json:"signer"`
			Recipient string `json:"recipient"`
			Message   struct {
				Type    int    `json:"type"`
				Payload string `json:"payload"`
			} `json:"message"`
		} `json:"transaction"`
	} `json:"data"`
}

type AccountInfoResponse struct {
	Account struct {
		Address          string `json:"address"`
		LinkedAccountKey string `json:"linkedAccountKey"`
		Mosaics          []struct {
			Id     [2]uint64 `json:"id"`
			Amount [2]uint64 `json:"amount"`
		} `json:"mosaics"`
	} `json:"account"`
}

// DelegationListener runs a background outbound polling loop that scans the ProximaX
// Sirius blockchain for incoming encrypted delegation transactions sent to the node's harvest address.
type DelegationListener struct {
	resourcesDir string
	dataPath     string
	apiNodes     []string
	httpClient   *http.Client

	mu               sync.RWMutex
	harvestKey       string
	nodeAddress      string
	nodePublicKey    string
	processedHashes  map[string]bool
	stateFilePath    string

	stopChan chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewDelegationListener initializes the delegation listener with resources path and optional api endpoints
func NewDelegationListener(resourcesDir string, dataPath string, customApiNodes []string) *DelegationListener {
	nodes := customApiNodes
	if len(nodes) == 0 {
		nodes = crypto.DefaultApiNodes
	}

	statePath := filepath.Join(dataPath, "delegation_listener_state.json")
	if dataPath == "" {
		statePath = filepath.Join(resourcesDir, "delegation_listener_state.json")
	}

	dl := &DelegationListener{
		resourcesDir:    resourcesDir,
		dataPath:        dataPath,
		apiNodes:        nodes,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		processedHashes: make(map[string]bool),
		stateFilePath:   statePath,
		stopChan:        make(chan struct{}),
	}

	dl.loadState()
	dl.reloadHarvestIdentity()
	return dl
}

func (dl *DelegationListener) loadState() {
	data, err := os.ReadFile(dl.stateFilePath)
	if err != nil {
		return
	}
	var hashes []string
	if err := json.Unmarshal(data, &hashes); err == nil {
		dl.mu.Lock()
		for _, h := range hashes {
			dl.processedHashes[strings.ToUpper(h)] = true
		}
		dl.mu.Unlock()
	}
}

func (dl *DelegationListener) saveState() {
	dl.mu.RLock()
	hashes := make([]string, 0, len(dl.processedHashes))
	for h := range dl.processedHashes {
		hashes = append(hashes, h)
	}
	dl.mu.RUnlock()

	// Limit to last 5000 transactions to prevent indefinite growth
	if len(hashes) > 5000 {
		hashes = hashes[len(hashes)-5000:]
	}

	data, err := json.Marshal(hashes)
	if err == nil {
		_ = os.WriteFile(dl.stateFilePath, data, 0600)
	}
}

func (dl *DelegationListener) reloadHarvestIdentity() {
	harvestPropPath := filepath.Join(dl.resourcesDir, "config-harvesting.properties")
	content, err := os.ReadFile(harvestPropPath)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "harvestKey") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[1])
				if len(key) == 64 {
					dl.mu.Lock()
					dl.harvestKey = strings.ToUpper(key)
					if kp, err := crypto.KeyPairFromPrivateKey(dl.harvestKey); err == nil && kp != nil {
						dl.nodeAddress = kp.Address
						dl.nodePublicKey = kp.PublicKey
					}
					dl.mu.Unlock()
				}
			}
		}
	}
}

// Start launches the background listener loop
func (dl *DelegationListener) Start() {
	dl.wg.Add(1)
	go dl.runLoop()
	log.Printf("[Delegation Listener] Background service started (Node Address: %s)", dl.GetNodeAddress())
}

// Stop cleanly terminates the background listener
func (dl *DelegationListener) Stop() {
	dl.stopOnce.Do(func() {
		close(dl.stopChan)
		dl.wg.Wait()
		dl.saveState()
		log.Println("[Delegation Listener] Background service stopped")
	})
}

func (dl *DelegationListener) GetNodeAddress() string {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	return dl.nodeAddress
}

func (dl *DelegationListener) GetNodePublicKey() string {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	return dl.nodePublicKey
}

func (dl *DelegationListener) runLoop() {
	defer dl.wg.Done()

	// Initial check on boot
	dl.checkIncomingTransactions()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dl.stopChan:
			return
		case <-ticker.C:
			dl.checkIncomingTransactions()
		}
	}
}

func (dl *DelegationListener) checkIncomingTransactions() {
	dl.mu.RLock()
	harvestKey := dl.harvestKey
	nodeAddr := dl.nodeAddress
	dl.mu.RUnlock()

	if harvestKey == "" || nodeAddr == "" {
		dl.reloadHarvestIdentity()
		dl.mu.RLock()
		harvestKey = dl.harvestKey
		nodeAddr = dl.nodeAddress
		dl.mu.RUnlock()
		if harvestKey == "" || nodeAddr == "" {
			return
		}
	}

	cleanAddr := strings.ToUpper(strings.ReplaceAll(nodeAddr, "-", ""))
	if len(cleanAddr) != 40 {
		return
	}

	var txList *ConfirmedTransactionsResponse
	for _, apiNode := range dl.apiNodes {
		url := fmt.Sprintf("%s/transactions/confirmed?recipientAddress=%s&pageSize=20&order=desc", strings.TrimRight(apiNode, "/"), cleanAddr)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := dl.httpClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var res ConfirmedTransactionsResponse
			if jsonErr := json.Unmarshal(body, &res); jsonErr == nil {
				txList = &res
				break
			}
		} else {
			_ = resp.Body.Close()
		}
	}

	if txList == nil || len(txList.Data) == 0 {
		return
	}

	hasNewChanges := false
	for _, item := range txList.Data {
		txHash := strings.ToUpper(strings.TrimSpace(item.Meta.Hash))
		if txHash == "" {
			continue
		}

		dl.mu.RLock()
		alreadyProcessed := dl.processedHashes[txHash]
		dl.mu.RUnlock()

		if alreadyProcessed {
			continue
		}

		// TransferTransaction type is 16724 (0x4154)
		if item.Transaction.Type != 16724 {
			dl.markProcessed(txHash)
			continue
		}

		// Must have an Encrypted Message (Type == 1)
		if item.Transaction.Message.Type != 1 || item.Transaction.Message.Payload == "" {
			dl.markProcessed(txHash)
			continue
		}

		senderPubKey := strings.ToUpper(strings.TrimSpace(item.Transaction.Signer))
		if len(senderPubKey) != 64 {
			dl.markProcessed(txHash)
			continue
		}

		// Decrypt the payload using the node's local harvest key
		decryptedText, err := crypto.DecryptDelegationPayload(item.Transaction.Message.Payload, senderPubKey, harvestKey)
		if err != nil {
			log.Printf("[Delegation Listener] Failed to decrypt message from %s (tx: %s): %v", senderPubKey, txHash, err)
			dl.markProcessed(txHash)
			continue
		}

		// Parse the remote private key and action
		remoteKey, action, err := crypto.ParseDelegatedPayload(decryptedText)
		if err != nil {
			log.Printf("[Delegation Listener] Unrecognized payload format from %s: %v", senderPubKey, err)
			dl.markProcessed(txHash)
			continue
		}

		// Derive sender address
		senderKp, err := crypto.KeyPairFromPrivateKey(strings.Repeat("0", 64)) // dummy for address conversion helper
		var senderAddress string
		if kp, kErr := crypto.GenerateKeyPair(); kErr == nil && kp != nil {
			// Get sender address from public key via helper
			senderAddress = getAddressFromPublicKey(senderPubKey, dl.apiNodes, dl.httpClient)
		}
		_ = senderKp

		if senderAddress == "" {
			log.Printf("[Delegation Listener] Could not resolve address for sender %s", senderPubKey)
			dl.markProcessed(txHash)
			continue
		}

		safeSender := regexp.MustCompile(`[^a-zA-Z0-9_\-]`).ReplaceAllString(senderAddress, "_")
		delegatedDir := filepath.Join(dl.resourcesDir, "delegated_keys")
		_ = os.MkdirAll(delegatedDir, 0700)
		targetKeyPath := filepath.Join(delegatedDir, safeSender+".key")

		if action == "unlink" {
			_ = os.Remove(targetKeyPath)
			log.Printf("[Delegation Listener] Processed UNLINK request for %s (tx: %s)", senderAddress, txHash)
			dl.markProcessed(txHash)
			hasNewChanges = true
			continue
		}

		// Verify on-chain account link and balance for LINK action
		remoteKp, err := crypto.KeyPairFromPrivateKey(remoteKey)
		if err != nil {
			log.Printf("[Delegation Listener] Invalid remote private key from %s: %v", senderAddress, err)
			dl.markProcessed(txHash)
			continue
		}

		isLinked, balanceOk, verifyErr := verifyOnChainAccountLink(senderAddress, remoteKp.PublicKey, dl.apiNodes, dl.httpClient)
		if verifyErr != nil {
			log.Printf("[Delegation Listener] Warning: Verification check failed for %s: %v (will retry next tick)", senderAddress, verifyErr)
			continue
		}

		if !balanceOk {
			log.Printf("[Delegation Listener] Rejected delegation from %s: Insufficient balance (< 100,000 XPX)", senderAddress)
			dl.markProcessed(txHash)
			continue
		}

		if !isLinked {
			log.Printf("[Delegation Listener] Rejected delegation from %s: Remote key %s does not match active on-chain AccountLink", senderAddress, remoteKp.PublicKey)
			dl.markProcessed(txHash)
			continue
		}

		// Write key to delegated_keys/<sender>.key with 0600 permissions
		if err := os.WriteFile(targetKeyPath, []byte(strings.ToUpper(remoteKey)+"\n"), 0600); err != nil {
			log.Printf("[Delegation Listener] Failed to write key file %s: %v", targetKeyPath, err)
			continue
		}

		log.Printf("[Delegation Listener] ✅ Successfully ingested on-chain delegated key for %s (Harvester PubKey: %s, tx: %s)", senderAddress, remoteKp.PublicKey, txHash)
		dl.markProcessed(txHash)
		hasNewChanges = true
	}

	if hasNewChanges {
		dl.saveState()
	}
}

func (dl *DelegationListener) markProcessed(txHash string) {
	dl.mu.Lock()
	dl.processedHashes[txHash] = true
	dl.mu.Unlock()
}

func getAddressFromPublicKey(pubKey string, apiNodes []string, client *http.Client) string {
	for _, node := range apiNodes {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), pubKey)
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var acc AccountInfoResponse
			if json.NewDecoder(resp.Body).Decode(&acc) == nil && acc.Account.Address != "" {
				return acc.Account.Address
			}
		}
	}
	return ""
}

func verifyOnChainAccountLink(address, expectedRemotePubKey string, apiNodes []string, client *http.Client) (isLinked bool, balanceOk bool, err error) {
	for _, node := range apiNodes {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), address)
		resp, errFetch := client.Get(url)
		if errFetch != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var acc AccountInfoResponse
		if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
			continue
		}

		// Check active AccountLink
		linkedMatch := strings.EqualFold(acc.Account.LinkedAccountKey, expectedRemotePubKey)

		// Check XPX balance (asset id 0x402B2F579FAEBC59 or mosaic namespace)
		// 100,000 XPX = 100,000 * 1,000,000 = 100,000,000,000 micro-XPX
		hasSufficientXPX := false
		for _, m := range acc.Account.Mosaics {
			// In Sirius, amount is uint64 represented as [low, high]
			low := m.Amount[0]
			high := m.Amount[1]
			amount := (uint64(high) << 32) | uint64(low)
			if amount >= 100000000000 {
				hasSufficientXPX = true
				break
			}
		}

		return linkedMatch, hasSufficientXPX, nil
	}

	return false, false, fmt.Errorf("could not query account info from any API node")
}
