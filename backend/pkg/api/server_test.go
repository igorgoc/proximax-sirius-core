package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"proximax-sirius-core/pkg/updater"
)

func TestEngineResetMethodEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	_ = os.MkdirAll(binDir, 0755)

	manifestPath := filepath.Join(tempDir, "engine.compat.json")
	_ = os.WriteFile(manifestPath, []byte(`{"recommendedVersion":"v1.9.7"}`), 0644)

	eu := updater.NewEngineUpdater(binDir, manifestPath, nil, nil)
	s := &Server{
		engineUpdater: eu,
		apiToken:      "test_token_123",
	}

	// 1. GET /api/engine/reset MUST return 405 Method Not Allowed
	getReq := httptest.NewRequest("GET", "/api/engine/reset", nil)
	getRec := httptest.NewRecorder()
	s.handleEngineReset(getRec, getReq)

	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Expected GET /api/engine/reset to return HTTP 405, got: %d", getRec.Code)
	}

	// 2. POST /api/engine/reset should succeed (200 OK)
	postReq := httptest.NewRequest("POST", "/api/engine/reset", nil)
	postRec := httptest.NewRecorder()
	s.handleEngineReset(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("Expected POST /api/engine/reset to return HTTP 200, got: %d", postRec.Code)
	}
}
