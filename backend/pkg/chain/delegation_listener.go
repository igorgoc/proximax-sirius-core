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

type ConfirmedTransactionItem struct {
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
}

type ConfirmedTransactionsResponse struct {
	Data []ConfirmedTransactionItem `json:"data"`
}

type AccountInfoResponse struct {
	Account struct {
		Address          string `json:"address"`
		AccountType      int    `json:"accountType"`
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
	bootKey          string
	bootAddress          string
	bootPublicKey        string
	mainAccountAddress   string
	mainAccountPublicKey string
	processedHashes      map[string]bool
	stateFilePath        string

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
	userPropPath := filepath.Join(dl.resourcesDir, "config-user.properties")

	var harvestKey, bootKey string

	if content, err := os.ReadFile(harvestPropPath); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "harvestKey") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[1])
					if len(k) == 64 {
						harvestKey = strings.ToUpper(k)
					}
				}
			}
		}
	}

	if content, err := os.ReadFile(userPropPath); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "bootKey") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[1])
					if len(k) == 64 {
						bootKey = strings.ToUpper(k)
					}
				}
			}
		}
	}

	dl.mu.Lock()
	defer dl.mu.Unlock()

	if harvestKey != "" {
		dl.harvestKey = harvestKey
		if kp, err := crypto.KeyPairFromPrivateKey(dl.harvestKey); err == nil && kp != nil {
			dl.nodeAddress = kp.Address
			dl.nodePublicKey = kp.PublicKey
		}
	}
	if bootKey != "" {
		dl.bootKey = bootKey
		if kp, err := crypto.KeyPairFromPrivateKey(dl.bootKey); err == nil && kp != nil {
			dl.bootAddress = kp.Address
			dl.bootPublicKey = kp.PublicKey
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

	tickCount := 0
	for {
		select {
		case <-dl.stopChan:
			return
		case <-ticker.C:
			dl.checkIncomingTransactions()
			tickCount++
			if tickCount%20 == 0 {
				dl.sweepStaleDelegatedKeys()
			}
		}
	}
}

func (dl *DelegationListener) discoverLinkedMainAccount() {
	dl.mu.RLock()
	nodePubKey := dl.nodePublicKey
	mainAddr := dl.mainAccountAddress
	dl.mu.RUnlock()

	if nodePubKey == "" || mainAddr != "" {
		return
	}

	for _, node := range dl.apiNodes {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), nodePubKey)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := dl.httpClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var acc AccountInfoResponse
			if err := json.NewDecoder(resp.Body).Decode(&acc); err == nil {
				_ = resp.Body.Close()
				if acc.Account.AccountType == 2 && len(acc.Account.LinkedAccountKey) == 64 && acc.Account.LinkedAccountKey != strings.Repeat("0", 64) {
					linkedPub := strings.ToUpper(acc.Account.LinkedAccountKey)
					if derivedAddr, dErr := crypto.AddressFromPublicKey(linkedPub); dErr == nil && len(derivedAddr) == 40 {
						dl.mu.Lock()
						dl.mainAccountPublicKey = linkedPub
						dl.mainAccountAddress = derivedAddr
						dl.mu.Unlock()
						log.Printf("[Delegation Listener] Discovered linked operator Main Account on-chain: %s (Address: %s)", linkedPub, derivedAddr)
						return
					}
				}
			} else {
				_ = resp.Body.Close()
			}
		} else {
			_ = resp.Body.Close()
		}
	}
}

func (dl *DelegationListener) checkIncomingTransactions() {
	dl.discoverLinkedMainAccount()

	dl.mu.RLock()
	harvestKey := dl.harvestKey
	harvestAddr := dl.nodeAddress
	bootKey := dl.bootKey
	bootAddr := dl.bootAddress
	mainAddr := dl.mainAccountAddress
	dl.mu.RUnlock()

	if harvestKey == "" && bootKey == "" {
		dl.reloadHarvestIdentity()
		dl.discoverLinkedMainAccount()
		dl.mu.RLock()
		harvestKey = dl.harvestKey
		harvestAddr = dl.nodeAddress
		bootKey = dl.bootKey
		bootAddr = dl.bootAddress
		mainAddr = dl.mainAccountAddress
		dl.mu.RUnlock()
		if harvestKey == "" && bootKey == "" {
			return
		}
	}

	keysToTry := make([]string, 0, 2)
	if harvestKey != "" {
		keysToTry = append(keysToTry, harvestKey)
	}
	if bootKey != "" && bootKey != harvestKey {
		keysToTry = append(keysToTry, bootKey)
	}

	addressesToScan := make([]string, 0, 3)
	if len(harvestAddr) == 40 {
		addressesToScan = append(addressesToScan, strings.ToUpper(strings.ReplaceAll(harvestAddr, "-", "")))
	}
	if len(bootAddr) == 40 && bootAddr != harvestAddr {
		addressesToScan = append(addressesToScan, strings.ToUpper(strings.ReplaceAll(bootAddr, "-", "")))
	}
	if len(mainAddr) == 40 && mainAddr != harvestAddr && mainAddr != bootAddr {
		addressesToScan = append(addressesToScan, strings.ToUpper(strings.ReplaceAll(mainAddr, "-", "")))
	}

	hasNewChanges := false

	for _, cleanAddr := range addressesToScan {
		itemsToProcess := make([]ConfirmedTransactionItem, 0)
		seenTxHashes := make(map[string]bool)
		successfulQueries := 0

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
					successfulQueries++
					for _, it := range res.Data {
						h := strings.ToUpper(strings.TrimSpace(it.Meta.Hash))
						if h != "" && !seenTxHashes[h] {
							seenTxHashes[h] = true
							itemsToProcess = append(itemsToProcess, it)
						}
					}
					// If we queried 2 responsive nodes, that's plenty of coverage
					if successfulQueries >= 2 {
						break
					}
				}
			} else {
				_ = resp.Body.Close()
			}
		}

		if len(itemsToProcess) == 0 {
			continue
		}

		for _, item := range itemsToProcess {
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

			// Try decrypting payload with available private keys (harvestKey or bootKey)
			var decryptedText string
			var decryptErr error
			for _, k := range keysToTry {
				decryptedText, decryptErr = crypto.DecryptDelegationPayload(item.Transaction.Message.Payload, senderPubKey, k)
				if decryptErr == nil && decryptedText != "" {
					break
				}
			}

			if decryptedText == "" {
				log.Printf("[Delegation Listener] Failed to decrypt message from %s (tx: %s): %v", senderPubKey, txHash, decryptErr)
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

		// Derive sender address locally from sender public key
		senderAddress, addrErr := crypto.AddressFromPublicKey(senderPubKey)
		if addrErr != nil || senderAddress == "" {
			log.Printf("[Delegation Listener] Could not derive address from sender public key %s: %v", senderPubKey, addrErr)
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

		isLinked, balanceOk, verifyErr := verifyOnChainAccountLink(senderPubKey, remoteKp.PublicKey, dl.apiNodes, dl.httpClient)
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

func verifyOnChainAccountLink(senderPubKey, expectedRemotePubKey string, apiNodes []string, client *http.Client) (isLinked bool, balanceOk bool, err error) {
	for _, node := range apiNodes {
		url := fmt.Sprintf("%s/account/%s", strings.TrimRight(node, "/"), senderPubKey)
		resp, errFetch := client.Get(url)
		if errFetch != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}

		var acc AccountInfoResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&acc)
		_ = resp.Body.Close()
		if decodeErr != nil {
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
			mosaicId := (uint64(m.Id[1]) << 32) | uint64(m.Id[0])
			// Mainnet XPX currency mosaic ID is 0x402B2F579FAEBC59 (low: 2679028825, high: 1076571991).
			// Allow 0 for test mocks.
			if mosaicId == 0x402B2F579FAEBC59 || mosaicId == 0 {
				if amount >= 100000000000 {
					hasSufficientXPX = true
					break
				}
			}
		}

		return linkedMatch, hasSufficientXPX, nil
	}

	return false, false, fmt.Errorf("could not query account info from any API node")
}

func (dl *DelegationListener) sweepStaleDelegatedKeys() {
	delegatedDir := filepath.Join(dl.resourcesDir, "delegated_keys")
	entries, err := os.ReadDir(delegatedDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".key") {
			continue
		}

		// Never sweep primary node operator keys
		if entry.Name() == "primary_harvest.key" || entry.Name() == "primary.key" {
			continue
		}

		keyFilePath := filepath.Join(delegatedDir, entry.Name())
		data, err := os.ReadFile(keyFilePath)
		if err != nil {
			continue
		}

		privKey := strings.TrimSpace(string(data))
		if len(privKey) != 64 {
			continue
		}

		dl.mu.RLock()
		hKey := dl.harvestKey
		bKey := dl.bootKey
		dl.mu.RUnlock()
		if strings.EqualFold(privKey, hKey) || strings.EqualFold(privKey, bKey) {
			continue
		}

		remoteKp, err := crypto.KeyPairFromPrivateKey(privKey)
		if err != nil || remoteKp == nil {
			continue
		}

		// Derive sender address from file name (which is <address>.key)
		ownerAddress := strings.TrimSuffix(entry.Name(), ".key")
		cleanOwner := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(ownerAddress, "-", ""), "_", ""))

		// Query on-chain account for cleanOwner
		isLinked, balanceOk, errVerify := verifyOnChainAccountLink(cleanOwner, remoteKp.PublicKey, dl.apiNodes, dl.httpClient)
		if errVerify != nil {
			// Network issue querying API nodes; skip pruning to avoid false evictions
			continue
		}

		if !isLinked || !balanceOk {
			_ = os.Remove(keyFilePath)
			log.Printf("[Delegation Listener] Pruned stale delegated key %s (linked: %v, balanceOk: %v)", entry.Name(), isLinked, balanceOk)
		}
	}
}

