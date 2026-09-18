package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChainMonitor_GetLocalHeight(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius_height_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	indexPath := filepath.Join(tempDir, "index.dat")
	// Height 1337 in Little Endian uint64
	data := []byte{0x39, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		t.Fatalf("failed to write index.dat: %v", err)
	}

	cm := NewChainMonitor()
	h, err := cm.GetLocalHeight(tempDir)
	if err != nil {
		t.Fatalf("GetLocalHeight failed: %v", err)
	}
	if h != 1337 {
		t.Errorf("expected height 1337, got %d", h)
	}
}

func TestChainMonitor_CheckHarvesterStatus(t *testing.T) {
	// Create mock server simulating Sirius REST response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/harvesting") {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"harvester": map[string]interface{}{
						"key":                    "F1526889268625DF83CACC8D65DA130ACA4CF1B94CA651E7436E33E1279E4CF7",
						"owner":                  "AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899",
						"address":                "B852C87F0684EE8178A2C6F31CBE44F7AA9F28C48EAC851FAF",
						"disabledHeight":         []uint64{0, 0},
						"lastSigningBlockHeight": []uint64{13000000, 0},
						"effectiveBalance":       []uint64{1500000000000 % (1 << 32), 1500000000000 >> 32},
						"canHarvest":             true,
						"activity":               0.5,
						"greed":                  0.1,
					},
				},
			})
			return
		}

		// Main / Remote account lookup
		resp := map[string]interface{}{
			"account": map[string]interface{}{
				"address":          "B852C87F0684EE8178A2C6F31CBE44F7AA9F28C48EAC851FAF",
				"publicKey":        "F1526889268625DF83CACC8D65DA130ACA4CF1B94CA651E7436E33E1279E4CF7",
				"accountType":      2,
				"linkedAccountKey": "AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899",
				"mosaics": []interface{}{
					map[string]interface{}{
						"id":     []interface{}{float64(2679028825), float64(1076571991)},
						"amount": []interface{}{float64(1500000000000 % (1 << 32)), float64(1500000000000 >> 32)}, // 1,500,000 XPX
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cm := NewChainMonitor()
	status, err := cm.CheckHarvesterStatus("F1526889268625DF83CACC8D65DA130ACA4CF1B94CA651E7436E33E1279E4CF7", mockServer.URL)
	if err != nil {
		t.Fatalf("CheckHarvesterStatus failed: %v", err)
	}

	if !status.IsLinked {
		t.Errorf("expected IsLinked true, got false")
	}
	if status.AccountType != 2 {
		t.Errorf("expected AccountType 2, got %d", status.AccountType)
	}
	if status.LinkedPublicKey != "AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899" {
		t.Errorf("expected linked key 'AABB...', got '%s'", status.LinkedPublicKey)
	}
	if !status.IsEligible {
		t.Errorf("expected IsEligible true for >= 1,000,000 XPX, got false")
	}
	if !status.IsCommitteeHarvester {
		t.Errorf("expected IsCommitteeHarvester true, got false")
	}
	if !status.CanHarvest {
		t.Errorf("expected CanHarvest true, got false")
	}
}
