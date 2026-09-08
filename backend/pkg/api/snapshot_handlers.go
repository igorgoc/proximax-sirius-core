package api

import (
	"archive/tar"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"proximax-sirius-core/pkg/snapshot"
)

func (s *Server) handleSnapshotStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.snapshotMgr.GetStatus())
}

func (s *Server) handleSnapshotCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = s.snapshotMgr.Cancel()
	jsonResponse(w, map[string]string{"status": "ok", "message": "Snapshot operation cancelled"})
}

func (s *Server) handleSnapshotCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SourcePath string `json:"sourcePath"`
		TargetPath string `json:"targetPath"`
		Format     string `json:"format"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	srcPath := strings.TrimSpace(req.SourcePath)
	if srcPath == "" {
		srcPath = s.configMgr.GetDataPath()
	}
	destPath := strings.TrimSpace(req.TargetPath)
	format := strings.TrimSpace(req.Format)

	height, _ := s.chainMon.GetLocalHeight(srcPath)
	if logH := s.supervisor.GetLatestLogHeight(); logH > height {
		height = logH
	}

	_, err := s.snapshotMgr.CreateLocalSnapshot(srcPath, destPath, format, height)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "in_progress",
		"message": "Local snapshot creation initiated.",
	})
}

func (s *Server) handleSnapshotRestoreLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ArchivePath    string `json:"archivePath"`
		TargetDataPath string `json:"targetDataPath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	archivePath := strings.TrimSpace(req.ArchivePath)
	if archivePath == "" {
		jsonError(w, "Archive path cannot be empty", http.StatusBadRequest)
		return
	}

	targetDir := strings.TrimSpace(req.TargetDataPath)
	if targetDir == "" {
		targetDir = s.configMgr.GetDataPath()
	}

	if err := s.snapshotMgr.RestoreLocalSnapshot(archivePath, targetDir); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "in_progress",
		"message": "Local snapshot restoration initiated.",
	})
}

func (s *Server) handleSnapshotRestoreRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ManifestUrl      string `json:"manifestUrl"`
		ReleasePubKeyHex string `json:"releasePublicKeyHex"`
		TargetDataPath   string `json:"targetDataPath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	manifestUrl := strings.TrimSpace(req.ManifestUrl)
	if manifestUrl == "" {
		jsonError(w, "Manifest URL cannot be empty", http.StatusBadRequest)
		return
	}

	targetDir := strings.TrimSpace(req.TargetDataPath)
	if targetDir == "" {
		targetDir = s.configMgr.GetDataPath()
	}

	if err := s.snapshotMgr.RestoreRemoteSnapshot(manifestUrl, req.ReleasePubKeyHex, targetDir); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "in_progress",
		"message": "Remote snapshot download and verified extraction initiated.",
	})
}

// Mock snapshot state for UI flow verification
var (
	mockMu           sync.Mutex
	mockPubKey       ed25519.PublicKey
	mockPrivKey      ed25519.PrivateKey
	mockArchiveBytes []byte
	mockSha256       string
	mockArchiveName  = "sirius-snapshot-mock-h13885400.tar.zst"
)

func initMockSnapshot() {
	mockMu.Lock()
	defer mockMu.Unlock()
	if len(mockPubKey) > 0 {
		return
	}

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mockPubKey = pub
	mockPrivKey = priv

	// Build mock .tar.zst archive
	var buf strings.Builder
	hasher := sha256.New()
	mw := io.MultiWriter(&buf, hasher)
	zw, _ := zstd.NewWriter(mw)
	tw := tar.NewWriter(zw)

	content := []byte("ROCKSDB_CHUNKED_MOCK_RESTORED_CHAIN_DATA")
	hdr := &tar.Header{
		Name: "index.dat",
		Mode: 0644,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = zw.Close()

	mockArchiveBytes = []byte(buf.String())
	mockSha256 = hex.EncodeToString(hasher.Sum(nil))
}

// handleSnapshotMock serves mock files for verified end-to-end UI testing:
// - /api/snapshot/mock/manifest.json
// - /api/snapshot/mock/SHA256SUMS
// - /api/snapshot/mock/SHA256SUMS.sig
// - /api/snapshot/mock/sirius-snapshot-mock-h13885400.tar.zst
func (s *Server) handleSnapshotMock(w http.ResponseWriter, r *http.Request) {
	initMockSnapshot()

	path := strings.TrimPrefix(r.URL.Path, "/api/snapshot/mock/")
	host := r.Host
	if host == "" {
		host = "127.0.0.1:3080"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	switch path {
	case "manifest.json":
		manifest := snapshot.SnapshotManifest{
			Version:         "1.0",
			ChainHeight:     13885400,
			Network:         "mainnet",
			ArchiveName:     mockArchiveName,
			DownloadUrl:     fmt.Sprintf("%s://%s/api/snapshot/mock/%s", scheme, host, mockArchiveName),
			Sha256:          mockSha256,
			Format:          "tar.zst",
			UncompressedGB:  0.1,
			CompressedBytes: int64(len(mockArchiveBytes)),
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		}
		jsonResponse(w, manifest)

	case "info":
		jsonResponse(w, map[string]interface{}{
			"manifestUrl":      fmt.Sprintf("%s://%s/api/snapshot/mock/manifest.json", scheme, host),
			"mockPublicKeyHex": hex.EncodeToString(mockPubKey),
		})

	case "SHA256SUMS":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		line := fmt.Sprintf("%s  %s\n", mockSha256, mockArchiveName)
		_, _ = w.Write([]byte(line))

	case "SHA256SUMS.sig":
		w.Header().Set("Content-Type", "application/octet-stream")
		line := fmt.Sprintf("%s  %s\n", mockSha256, mockArchiveName)
		sig := ed25519.Sign(mockPrivKey, []byte(line))
		_, _ = w.Write(sig)

	case mockArchiveName:
		w.Header().Set("Content-Type", "application/zstd")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mockArchiveBytes)))
		_, _ = w.Write(mockArchiveBytes)

	default:
		http.NotFound(w, r)
	}
}
