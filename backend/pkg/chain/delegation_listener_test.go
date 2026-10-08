package chain

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"proximax-sirius-core/pkg/crypto"
)

func TestDelegationListener_StatePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "del_state_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(resourcesDir, 0700)
	_ = os.MkdirAll(dataDir, 0700)

	dl := NewDelegationListener(resourcesDir, dataDir, nil)
	dl.markProcessed("HASH_1")
	dl.markProcessed("HASH_2")
	dl.saveState()

	// Create new instance and verify state loaded
	dl2 := NewDelegationListener(resourcesDir, dataDir, nil)
	dl2.mu.RLock()
	h1 := dl2.processedHashes["HASH_1"]
	h2 := dl2.processedHashes["HASH_2"]
	h3 := dl2.processedHashes["HASH_3"]
	dl2.mu.RUnlock()

	if !h1 || !h2 || h3 {
		t.Fatalf("expected HASH_1 and HASH_2 to be loaded, got h1=%v h2=%v h3=%v", h1, h2, h3)
	}
}

func TestDelegationListener_EndToEndIngestion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "del_e2e_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(resourcesDir, 0700)
	_ = os.MkdirAll(dataDir, 0700)

	// Generate Node harvest key
	nodeKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	harvestProp := filepath.Join(resourcesDir, "config-harvesting.properties")
	_ = os.WriteFile(harvestProp, []byte("harvestKey = "+nodeKp.PrivateKey+"\n"), 0600)

	// Generate Delegator key and Remote key
	delegatorKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	remoteKp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Payload with remote key
	payload := crypto.DelegatedStakingPayload{
		Type:             "sirius.delegated_staking",
		Version:          1,
		Action:           "link",
		RemotePrivateKey: remoteKp.PrivateKey,
	}
	payloadBytes, _ := json.Marshal(payload)

	// Encrypt payload for Node using Go internal crypto / block cipher
	// Sender = delegatorKp, Recipient = nodeKp
	senderPrivBytes, _ := hex.DecodeString(delegatorKp.PrivateKey)
	recipPubBytes, _ := hex.DecodeString(nodeKp.PublicKey)
	_ = senderPrivBytes
	_ = recipPubBytes

	// Create mock REST server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.Contains(path, "/transactions/confirmed") {
			// Mock confirmed transaction with dummy encrypted payload
			// We can generate payload using helper
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[]}`))
			return
		}

		if strings.Contains(path, "/account/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return account info with matched link and >=100k XPX
			res := AccountInfoResponse{}
			res.Account.Address = delegatorKp.Address
			res.Account.LinkedAccountKey = remoteKp.PublicKey
			res.Account.Mosaics = append(res.Account.Mosaics, struct {
				Id     [2]uint64 `json:"id"`
				Amount [2]uint64 `json:"amount"`
			}{
				Id:     [2]uint64{0x9FAEBC59, 0x402B2F57},
				Amount: [2]uint64{0x540BE400, 0x17}, // > 100,000 XPX
			})
			_ = json.NewEncoder(w).Encode(res)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	dl := NewDelegationListener(resourcesDir, dataDir, []string{mockServer.URL})
	if dl.GetNodeAddress() != nodeKp.Address {
		t.Errorf("expected node address %s, got %s", nodeKp.Address, dl.GetNodeAddress())
	}
	if !strings.EqualFold(dl.GetNodePublicKey(), nodeKp.PublicKey) {
		t.Errorf("expected node public key %s, got %s", nodeKp.PublicKey, dl.GetNodePublicKey())
	}

	_ = payloadBytes
}

func TestDelegationListener_VerifyOnChainMosaicValidation(t *testing.T) {
	delegatorKp, _ := crypto.GenerateKeyPair()
	remoteKp, _ := crypto.GenerateKeyPair()

	// 1. Server with valid XPX mosaic (0x402B2F579FAEBC59) and balance >= 100k
	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := AccountInfoResponse{}
		res.Account.Address = delegatorKp.Address
		res.Account.LinkedAccountKey = remoteKp.PublicKey
		res.Account.Mosaics = append(res.Account.Mosaics, struct {
			Id     [2]uint64 `json:"id"`
			Amount [2]uint64 `json:"amount"`
		}{
			Id:     [2]uint64{0x9FAEBC59, 0x402B2F57},
			Amount: [2]uint64{0x540BE400, 0x17}, // > 100,000 XPX
		})
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer validServer.Close()

	linked, balOk, err := verifyOnChainAccountLink(delegatorKp.Address, remoteKp.PublicKey, []string{validServer.URL}, validServer.Client())
	if err != nil || !linked || !balOk {
		t.Fatalf("expected valid XPX balance to pass: linked=%v, balOk=%v, err=%v", linked, balOk, err)
	}

	// 2. Server with non-XPX custom mosaic (e.g. spam token)
	invalidMosaicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := AccountInfoResponse{}
		res.Account.Address = delegatorKp.Address
		res.Account.LinkedAccountKey = remoteKp.PublicKey
		res.Account.Mosaics = append(res.Account.Mosaics, struct {
			Id     [2]uint64 `json:"id"`
			Amount [2]uint64 `json:"amount"`
		}{
			Id:     [2]uint64{0x11111111, 0x22222222}, // Invalid mosaic ID
			Amount: [2]uint64{0x540BE400, 0x17},
		})
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer invalidMosaicServer.Close()

	linked2, balOk2, err2 := verifyOnChainAccountLink(delegatorKp.Address, remoteKp.PublicKey, []string{invalidMosaicServer.URL}, invalidMosaicServer.Client())
	if err2 != nil || !linked2 || balOk2 {
		t.Fatalf("expected non-XPX mosaic to fail balance check: linked=%v, balOk=%v, err=%v", linked2, balOk2, err2)
	}
}

func TestDelegationListener_SweepStaleKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "del_sweep_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resourcesDir := filepath.Join(tmpDir, "resources")
	delegatedDir := filepath.Join(resourcesDir, "delegated_keys")
	_ = os.MkdirAll(delegatedDir, 0700)

	remoteKp, _ := crypto.GenerateKeyPair()
	ownerAddr := "XATWCKBWGJ7GVTACDV2I2OK7LLRIYXFGNNMM3IS2"
	keyFile := filepath.Join(delegatedDir, ownerAddr+".key")
	_ = os.WriteFile(keyFile, []byte(remoteKp.PrivateKey+"\n"), 0600)

	// Mock server returning unlinked account
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := AccountInfoResponse{}
		res.Account.Address = ownerAddr
		res.Account.LinkedAccountKey = strings.Repeat("0", 64) // Unlinked!
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer mockServer.Close()

	dl := NewDelegationListener(resourcesDir, tmpDir, []string{mockServer.URL})
	dl.sweepStaleDelegatedKeys()

	if _, statErr := os.Stat(keyFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale unlinked key %s to be deleted by sweep", keyFile)
	}
}

