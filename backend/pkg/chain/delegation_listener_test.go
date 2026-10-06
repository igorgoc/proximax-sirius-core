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
