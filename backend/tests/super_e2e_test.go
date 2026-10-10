package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"proximax-sirius-core/pkg/chain"
	"proximax-sirius-core/pkg/crypto"
)

// getFreePort asks the kernel for a free open TCP port.
func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startMockSiriusAPIServer(t *testing.T, port int) (*exec.Cmd, string) {
	possiblePaths := []string{
		"../../scripts/ci/mock_sirius_api.py",
		"../scripts/ci/mock_sirius_api.py",
		"scripts/ci/mock_sirius_api.py",
	}

	var scriptPath string
	for _, p := range possiblePaths {
		if abs, err := filepath.Abs(p); err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				scriptPath = abs
				break
			}
		}
	}

	if scriptPath == "" {
		t.Fatalf("could not locate mock_sirius_api.py in candidate paths")
	}

	cmd := exec.Command("python3", scriptPath, fmt.Sprintf("%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mock_sirius_api.py: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait for server health check
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		_ = cmd.Process.Kill()
		t.Fatalf("mock_sirius_api.py failed to respond healthy on port %d within 5s", port)
	}

	return cmd, url
}

func TestSuperE2E_FullStakingLifecycle(t *testing.T) {
	port := getFreePort(t)
	mockCmd, mockURL := startMockSiriusAPIServer(t, port)
	defer func() {
		if mockCmd != nil && mockCmd.Process != nil {
			_ = mockCmd.Process.Kill()
			_ = mockCmd.Wait()
		}
	}()

	// 1. Create sandbox directories
	tmpDir, err := os.MkdirTemp("", "super_e2e_sandbox_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	dataDir := filepath.Join(tmpDir, "data")
	delegatedDir := filepath.Join(resourcesDir, "delegated_keys")
	_ = os.MkdirAll(resourcesDir, 0700)
	_ = os.MkdirAll(dataDir, 0700)
	_ = os.MkdirAll(delegatedDir, 0700)

	// 2. Setup Node Harvester Keypair with 10 slots available
	nodeKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate node keypair: %v", err)
	}
	harvestProp := filepath.Join(resourcesDir, "config-harvesting.properties")
	harvestConfig := "harvestKey = " + nodeKp.PrivateKey + "\nmaxUnlockedAccounts = 10\n"
	if err := os.WriteFile(harvestProp, []byte(harvestConfig), 0600); err != nil {
		t.Fatal(err)
	}

	// 3. Setup Alice (Legitimate delegator: 200,000 XPX)
	aliceKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Alice keypair: %v", err)
	}
	remoteKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Remote keypair: %v", err)
	}

	// Register Alice on mock API with active account link & 200,000 XPX
	aliceMockAcc := map[string]interface{}{
		"account":          aliceKp.PublicKey,
		"address":          aliceKp.Address,
		"accountType":      1,
		"linkedAccountKey": remoteKp.PublicKey,
		"balance":          200000000000, // 200,000 XPX
	}
	accPayload, _ := json.Marshal(aliceMockAcc)
	resp, err := http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(accPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to register Alice on mock API: %v", err)
	}
	_ = resp.Body.Close()

	// 4. Encrypt Alice's delegated staking payload
	alicePayloadObj := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: remoteKp.PrivateKey,
	}
	alicePayloadJson, _ := json.Marshal(alicePayloadObj)

	encHex, err := crypto.EncryptDelegationPayload(string(alicePayloadJson), aliceKp.PrivateKey, nodeKp.PublicKey)
	if err != nil {
		t.Fatalf("failed to encrypt Alice delegation payload: %v", err)
	}

	// 5. Inject confirmed transaction into Mock Sirius API
	linkTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "A1B2C3D4E5F67890123456789012345678901234567890123456789012345678",
		},
		"transaction": map[string]interface{}{
			"type":      16724, // TransferTransaction
			"signer":    aliceKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1, // EncryptedMessage
				"payload": encHex,
			},
		},
	}
	txBytes, _ := json.Marshal(linkTx)
	injResp, err := http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(txBytes))
	if err != nil || injResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to inject transaction to mock API: %v", err)
	}
	_ = injResp.Body.Close()

	// 6. Start autonomous Delegation Listener daemon
	dl := chain.NewDelegationListener(resourcesDir, dataDir, []string{mockURL})

	// Verify initial slot capacity: 10 max slots, 0 active (all 10 slots free)
	if dl.GetMaxSlots() != 10 {
		t.Fatalf("expected maxSlots = 10, got %d", dl.GetMaxSlots())
	}
	if dl.GetActiveSlots() != 0 {
		t.Fatalf("expected activeSlots = 0 initially, got %d", dl.GetActiveSlots())
	}

	dl.Start()
	defer dl.Stop()

	// 7. Assert that Alice's .key file is created with mode 0600
	aliceKeyFile := filepath.Join(delegatedDir, aliceKp.Address+".key")
	var keyFound bool
	for i := 0; i < 40; i++ {
		info, statErr := os.Stat(aliceKeyFile)
		if statErr == nil {
			keyFound = true
			perm := info.Mode().Perm()
			if perm != 0600 {
				t.Fatalf("expected key file permissions 0600, got %o", perm)
			}
			content, _ := os.ReadFile(aliceKeyFile)
			trimmed := strings.TrimSpace(string(content))
			if !strings.EqualFold(trimmed, remoteKp.PrivateKey) {
				t.Fatalf("key file content mismatch: expected %s, got %s", remoteKp.PrivateKey, trimmed)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !keyFound {
		t.Fatalf("expected delegated key file %s was not created by listener within 4s", aliceKeyFile)
	}

	// Verify active slot count increased to 1 (9 free slots remain)
	if dl.GetActiveSlots() != 1 {
		t.Fatalf("expected activeSlots = 1 after Alice connected, got %d", dl.GetActiveSlots())
	}

	// 8. Adversarial Test 1: Underfunded account (Bob with only 5,000 XPX)
	bobKp, _ := crypto.GenerateKeyPair()
	bobRemoteKp, _ := crypto.GenerateKeyPair()
	bobMockAcc := map[string]interface{}{
		"account":          bobKp.PublicKey,
		"address":          bobKp.Address,
		"accountType":      1,
		"linkedAccountKey": bobRemoteKp.PublicKey,
		"balance":          5000000000, // 5,000 XPX (< 100,000 XPX threshold)
	}
	bobAccPayload, _ := json.Marshal(bobMockAcc)
	resp, _ = http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(bobAccPayload))
	_ = resp.Body.Close()

	bobPayloadObj := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: bobRemoteKp.PrivateKey,
	}
	bobJson, _ := json.Marshal(bobPayloadObj)
	bobEncHex, err := crypto.EncryptDelegationPayload(string(bobJson), bobKp.PrivateKey, nodeKp.PublicKey)
	if err != nil {
		t.Fatalf("failed to encrypt Bob payload: %v", err)
	}

	bobTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    bobKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": bobEncHex,
			},
		},
	}
	bobTxBytes, _ := json.Marshal(bobTx)
	injResp, _ = http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(bobTxBytes))
	_ = injResp.Body.Close()

	dl.TriggerCheck()
	time.Sleep(200 * time.Millisecond)
	bobKeyFile := filepath.Join(delegatedDir, bobKp.Address+".key")
	if _, statErr := os.Stat(bobKeyFile); !os.IsNotExist(statErr) {
		t.Fatalf("security violation: underfunded account key %s was written to disk!", bobKeyFile)
	}

	// 9. Adversarial Test 2: User gives WRONG private key (Mismatched on-chain AccountLink)
	// Charlie has 200k XPX and is linked to RemoteKeyA on-chain, but maliciously or accidentally
	// sends RemoteKeyB's private key in the message payload.
	charlieKp, _ := crypto.GenerateKeyPair()
	charlieRealRemoteKp, _ := crypto.GenerateKeyPair()
	charlieWrongRemoteKp, _ := crypto.GenerateKeyPair()

	charlieMockAcc := map[string]interface{}{
		"account":          charlieKp.PublicKey,
		"address":          charlieKp.Address,
		"accountType":      1,
		"linkedAccountKey": charlieRealRemoteKp.PublicKey, // Linked to Real Remote key
		"balance":          200000000000,
	}
	charlieAccPayload, _ := json.Marshal(charlieMockAcc)
	resp, _ = http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(charlieAccPayload))
	_ = resp.Body.Close()

	// But Charlie sends charlieWrongRemoteKp in payload!
	charlieMismatchedPayload := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: charlieWrongRemoteKp.PrivateKey, // Wrong/Mismatched key!
	}
	charlieJson, _ := json.Marshal(charlieMismatchedPayload)
	charlieEncHex, err := crypto.EncryptDelegationPayload(string(charlieJson), charlieKp.PrivateKey, nodeKp.PublicKey)
	if err != nil {
		t.Fatalf("failed to encrypt Charlie payload: %v", err)
	}

	charlieTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    charlieKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": charlieEncHex,
			},
		},
	}
	charlieTxBytes, _ := json.Marshal(charlieTx)
	injResp, _ = http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(charlieTxBytes))
	_ = injResp.Body.Close()

	dl.TriggerCheck()
	time.Sleep(200 * time.Millisecond)
	charlieKeyFile := filepath.Join(delegatedDir, charlieKp.Address+".key")
	if _, statErr := os.Stat(charlieKeyFile); !os.IsNotExist(statErr) {
		t.Fatalf("security violation: mismatched remote key %s was written to disk!", charlieKeyFile)
	}

	// 10. Adversarial Test 3: User gives CORRUPTED/INVALID hex string as private key
	daveKp, _ := crypto.GenerateKeyPair()
	daveMockAcc := map[string]interface{}{
		"account":          daveKp.PublicKey,
		"address":          daveKp.Address,
		"accountType":      1,
		"linkedAccountKey": "1111111111111111111111111111111111111111111111111111111111111111",
		"balance":          200000000000,
	}
	daveAccPayload, _ := json.Marshal(daveMockAcc)
	resp, _ = http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(daveAccPayload))
	_ = resp.Body.Close()

	daveCorruptedPayload := map[string]interface{}{
		"type":             "sirius.delegated_staking",
		"version":          1,
		"action":           "link",
		"remotePrivateKey": "NOT_A_VALID_HEX_KEY_1234567890", // Corrupted input
	}
	daveJson, _ := json.Marshal(daveCorruptedPayload)
	daveEncHex, _ := crypto.EncryptDelegationPayload(string(daveJson), daveKp.PrivateKey, nodeKp.PublicKey)

	daveTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0D0",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    daveKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": daveEncHex,
			},
		},
	}
	daveTxBytes, _ := json.Marshal(daveTx)
	injResp, _ = http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(daveTxBytes))
	_ = injResp.Body.Close()

	dl.TriggerCheck()
	time.Sleep(200 * time.Millisecond)
	daveKeyFile := filepath.Join(delegatedDir, daveKp.Address+".key")
	if _, statErr := os.Stat(daveKeyFile); !os.IsNotExist(statErr) {
		t.Fatalf("security violation: corrupted key %s was written to disk!", daveKeyFile)
	}

	// 11. Unlink Test: Alice broadcasts unlink action
	unlinkPayloadObj := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "unlink",
		RemotePrivateKey: remoteKp.PrivateKey,
	}
	unlinkJson, _ := json.Marshal(unlinkPayloadObj)
	unlinkEncHex, err := crypto.EncryptDelegationPayload(string(unlinkJson), aliceKp.PrivateKey, nodeKp.PublicKey)
	if err != nil {
		t.Fatalf("failed to encrypt unlink payload: %v", err)
	}

	unlinkTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "F9E8D7C6B5A43210F9E8D7C6B5A43210F9E8D7C6B5A43210F9E8D7C6B5A43210",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    aliceKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": unlinkEncHex,
			},
		},
	}
	unlinkTxBytes, _ := json.Marshal(unlinkTx)
	injResp, _ = http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(unlinkTxBytes))
	_ = injResp.Body.Close()

	// Trigger listener check for unlink transaction
	dl.TriggerCheck()

	// Assert Alice's key file is removed and slot is freed
	var unlinkedOk bool
	for i := 0; i < 40; i++ {
		if _, statErr := os.Stat(aliceKeyFile); os.IsNotExist(statErr) {
			unlinkedOk = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !unlinkedOk {
		t.Fatalf("expected key file %s to be deleted after unlink transaction", aliceKeyFile)
	}

	if dl.GetActiveSlots() != 0 {
		t.Fatalf("expected activeSlots = 0 after unlink, got %d", dl.GetActiveSlots())
	}
}

func TestSuperE2E_PoolCapacitySaturation(t *testing.T) {
	port := getFreePort(t)
	mockCmd, mockURL := startMockSiriusAPIServer(t, port)
	defer func() {
		if mockCmd != nil && mockCmd.Process != nil {
			_ = mockCmd.Process.Kill()
			_ = mockCmd.Wait()
		}
	}()

	tmpDir, err := os.MkdirTemp("", "super_e2e_pool_sat_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	dataDir := filepath.Join(tmpDir, "data")
	delegatedDir := filepath.Join(resourcesDir, "delegated_keys")
	_ = os.MkdirAll(resourcesDir, 0700)
	_ = os.MkdirAll(dataDir, 0700)
	_ = os.MkdirAll(delegatedDir, 0700)

	// Configure node with exactly 1 MAX SLOT
	nodeKp, _ := crypto.GenerateKeyPair()
	harvestProp := filepath.Join(resourcesDir, "config-harvesting.properties")
	_ = os.WriteFile(harvestProp, []byte("harvestKey = "+nodeKp.PrivateKey+"\nmaxUnlockedAccounts = 1\n"), 0600)

	dl := chain.NewDelegationListener(resourcesDir, dataDir, []string{mockURL})
	if dl.GetMaxSlots() != 1 {
		t.Fatalf("expected maxSlots = 1, got %d", dl.GetMaxSlots())
	}
	dl.Start()
	defer dl.Stop()

	// 1. User 1 (Alice) connects and takes the ONLY available slot
	aliceKp, _ := crypto.GenerateKeyPair()
	aliceRemoteKp, _ := crypto.GenerateKeyPair()
	aliceMockAcc := map[string]interface{}{
		"account":          aliceKp.PublicKey,
		"address":          aliceKp.Address,
		"accountType":      1,
		"linkedAccountKey": aliceRemoteKp.PublicKey,
		"balance":          200000000000,
	}
	aBytes, _ := json.Marshal(aliceMockAcc)
	resp, _ := http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(aBytes))
	_ = resp.Body.Close()

	alicePayload := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: aliceRemoteKp.PrivateKey,
	}
	aJson, _ := json.Marshal(alicePayload)
	aEnc, _ := crypto.EncryptDelegationPayload(string(aJson), aliceKp.PrivateKey, nodeKp.PublicKey)
	aTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "1111111111111111111111111111111111111111111111111111111111111111",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    aliceKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": aEnc,
			},
		},
	}
	aTxBytes, _ := json.Marshal(aTx)
	injResp, _ := http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(aTxBytes))
	_ = injResp.Body.Close()

	dl.TriggerCheck()
	time.Sleep(300 * time.Millisecond)

	aliceKeyFile := filepath.Join(delegatedDir, aliceKp.Address+".key")
	if _, statErr := os.Stat(aliceKeyFile); os.IsNotExist(statErr) {
		t.Fatalf("expected Alice's key to be written into the 1 available slot")
	}
	if dl.GetActiveSlots() != 1 {
		t.Fatalf("expected activeSlots = 1, got %d", dl.GetActiveSlots())
	}

	// 2. User 2 (Bob) is legitimate (200k XPX, valid link), but POOL IS FULL (1/1 slots active)!
	bobKp, _ := crypto.GenerateKeyPair()
	bobRemoteKp, _ := crypto.GenerateKeyPair()
	bobMockAcc := map[string]interface{}{
		"account":          bobKp.PublicKey,
		"address":          bobKp.Address,
		"accountType":      1,
		"linkedAccountKey": bobRemoteKp.PublicKey,
		"balance":          200000000000,
	}
	bBytes, _ := json.Marshal(bobMockAcc)
	resp, _ = http.Post(mockURL+"/mock/set_account", "application/json", bytes.NewReader(bBytes))
	_ = resp.Body.Close()

	bobPayload := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: bobRemoteKp.PrivateKey,
	}
	bJson, _ := json.Marshal(bobPayload)
	bEnc, _ := crypto.EncryptDelegationPayload(string(bJson), bobKp.PrivateKey, nodeKp.PublicKey)
	bTx := map[string]interface{}{
		"meta": map[string]interface{}{
			"hash": "2222222222222222222222222222222222222222222222222222222222222222",
		},
		"transaction": map[string]interface{}{
			"type":      16724,
			"signer":    bobKp.PublicKey,
			"recipient": nodeKp.Address,
			"message": map[string]interface{}{
				"type":    1,
				"payload": bEnc,
			},
		},
	}
	bTxBytes, _ := json.Marshal(bTx)
	injResp, _ = http.Post(mockURL+"/mock/inject_tx", "application/json", bytes.NewReader(bTxBytes))
	_ = injResp.Body.Close()

	dl.TriggerCheck()
	time.Sleep(300 * time.Millisecond)

	bobKeyFile := filepath.Join(delegatedDir, bobKp.Address+".key")
	if _, statErr := os.Stat(bobKeyFile); !os.IsNotExist(statErr) {
		t.Fatalf("capacity violation: Bob's key was accepted when slot capacity was already full (1/1)!")
	}
}
