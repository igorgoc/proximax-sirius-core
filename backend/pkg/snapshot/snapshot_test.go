package snapshot

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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

type mockLifecycleController struct {
	isRunning bool
	stopCalls int
}

func (m *mockLifecycleController) StopNode() error {
	m.isRunning = false
	m.stopCalls++
	return nil
}

func (m *mockLifecycleController) StartNode(dataPath string) error {
	m.isRunning = true
	return nil
}

func (m *mockLifecycleController) IsRunning() bool {
	return m.isRunning
}

// 1. Unit Test: Ed25519 signature verification & checksum parsing
func TestSignatureAndChecksumVerification(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test keypair: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	checksumsContent := `
# Sirius Mainnet Checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  dummy.txt
4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945  sirius-snapshot-h13885000.tar.zst
`
	checksumsBytes := []byte(strings.TrimSpace(checksumsContent))
	sigBytes := ed25519.Sign(priv, checksumsBytes)

	// Valid signature
	if err := VerifyEd25519Signature(pubHex, checksumsBytes, sigBytes); err != nil {
		t.Fatalf("Valid signature rejected: %v", err)
	}

	// Tampered data
	tamperedBytes := append(checksumsBytes, []byte("\ncorrupted")...)
	if err := VerifyEd25519Signature(pubHex, tamperedBytes, sigBytes); err != ErrInvalidSignature {
		t.Fatalf("Expected ErrInvalidSignature on tampered data, got: %v", err)
	}

	// Wrong public key
	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := VerifyEd25519Signature(hex.EncodeToString(wrongPub), checksumsBytes, sigBytes); err != ErrInvalidSignature {
		t.Fatalf("Expected ErrInvalidSignature with wrong pubkey, got: %v", err)
	}

	// Parse valid checksum
	hash, err := ParseSha256Sums(string(checksumsBytes), "sirius-snapshot-h13885000.tar.zst")
	if err != nil {
		t.Fatalf("Failed to parse checksum: %v", err)
	}
	if hash != "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945" {
		t.Fatalf("Parsed incorrect hash: %s", hash)
	}

	// Missing entry
	if _, err := ParseSha256Sums(string(checksumsBytes), "nonexistent.tar.zst"); err == nil {
		t.Fatalf("Expected error for missing checksum entry")
	}
}

// 2. Integration Test: Part 1 Local Snapshot Create and Restore
func TestPart1_LocalSnapshotCreateAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	srcDataDir := filepath.Join(tempDir, "source_data")
	destBackupDir := filepath.Join(tempDir, "backups")
	restoreDestDir := filepath.Join(tempDir, "restored_data")

	_ = os.MkdirAll(filepath.Join(srcDataDir, "00000"), 0755)
	_ = os.MkdirAll(filepath.Join(srcDataDir, "statedb"), 0755)

	// Write mock blockchain data files
	_ = os.WriteFile(filepath.Join(srcDataDir, "index.dat"), []byte("INDEX_BLOCK_DATA_13885000"), 0644)
	_ = os.WriteFile(filepath.Join(srcDataDir, "commit_step.dat"), []byte("STEP_02"), 0644)
	_ = os.WriteFile(filepath.Join(srcDataDir, "00000", "hashes.dat"), []byte("HASHES_DATA_CHUNK_00000"), 0644)
	_ = os.WriteFile(filepath.Join(srcDataDir, "statedb", "accounts.sst"), []byte("ROCKSDB_SSTABLE_DATA_BLOCKS"), 0644)

	// Invariant test: server.lock MUST be excluded from snapshot archives
	_ = os.WriteFile(filepath.Join(srcDataDir, "server.lock"), []byte("LOCK_PID_1234"), 0644)

	ctrl := &mockLifecycleController{isRunning: true}
	mgr := NewSnapshotManager(ctrl, "", nil)

	// 1. Create local snapshot
	_, err := mgr.CreateLocalSnapshot(srcDataDir, destBackupDir, "tar.zst", 13885000)
	if err != nil {
		t.Fatalf("CreateLocalSnapshot failed to initiate: %v", err)
	}

	// Wait for background completion
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := mgr.GetStatus()
		if st.Stage == StageCompleted {
			break
		}
		if st.Stage == StageError {
			t.Fatalf("CreateLocalSnapshot failed: %s", st.ErrorMessage)
		}
		if time.Now().After(deadline) {
			t.Fatalf("CreateLocalSnapshot timed out")
		}
		time.Sleep(50 * time.Millisecond)
	}

	status := mgr.GetStatus()
	archivePath := status.TargetFile
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("Snapshot archive not found at %s: %v", archivePath, err)
	}

	if status.Manifest == nil {
		t.Fatalf("Manifest was not populated in status")
	}
	if status.Manifest.Sha256 == "" {
		t.Fatalf("Manifest SHA-256 is empty")
	}

	// Verify SHA-256 match
	actualSha, err := ComputeFileSha256(archivePath)
	if err != nil {
		t.Fatalf("Failed to compute SHA256 of created archive: %v", err)
	}
	if !strings.EqualFold(actualSha, status.Manifest.Sha256) {
		t.Fatalf("SHA256 mismatch on created archive! computed %s, manifest %s", actualSha, status.Manifest.Sha256)
	}

	// 2. Restore local snapshot into fresh directory
	if err := mgr.RestoreLocalSnapshot(archivePath, restoreDestDir); err != nil {
		t.Fatalf("RestoreLocalSnapshot failed to initiate: %v", err)
	}

	deadline = time.Now().Add(5 * time.Second)
	for {
		st := mgr.GetStatus()
		if st.Stage == StageCompleted {
			break
		}
		if st.Stage == StageError {
			t.Fatalf("RestoreLocalSnapshot failed: %s", st.ErrorMessage)
		}
		if time.Now().After(deadline) {
			t.Fatalf("RestoreLocalSnapshot timed out")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify restored file contents match originals
	indexRestored, err := os.ReadFile(filepath.Join(restoreDestDir, "index.dat"))
	if err != nil || string(indexRestored) != "INDEX_BLOCK_DATA_13885000" {
		t.Fatalf("Restored index.dat mismatch: %v (%s)", err, string(indexRestored))
	}

	hashesRestored, err := os.ReadFile(filepath.Join(restoreDestDir, "00000", "hashes.dat"))
	if err != nil || string(hashesRestored) != "HASHES_DATA_CHUNK_00000" {
		t.Fatalf("Restored hashes.dat mismatch: %v (%s)", err, string(hashesRestored))
	}

	// Verify server.lock is absent
	if _, err := os.Stat(filepath.Join(restoreDestDir, "server.lock")); err == nil {
		t.Fatalf("Invariant violated: server.lock was found in restored snapshot directory!")
	}
}

// 3. Integration Test: Part 2 Pluggable Remote Snapshot Streaming against Mock HTTP Server
func TestPart2_RemoteSnapshotStreamingMockServer(t *testing.T) {
	tempDir := t.TempDir()
	restoreTargetDir := filepath.Join(tempDir, "remote_restored")

	// Generate signing keys
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	// Create test .tar.zst archive in memory
	var archiveBuf strings.Builder
	hasher := sha256.New()

	archivePath := filepath.Join(tempDir, "sirius-snapshot-remote.tar.zst")
	outFile, _ := os.Create(archivePath)
	multiWriter := io.MultiWriter(outFile, hasher)

	zw, _ := zstd.NewWriter(multiWriter)
	tw := tar.NewWriter(zw)

	fileData := []byte("STREAMED_REMOTE_BLOCK_DATA_ROCKSDB")
	hdr := &tar.Header{
		Name: "index.dat",
		Mode: 0644,
		Size: int64(len(fileData)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(fileData)
	_ = tw.Close()
	_ = zw.Close()
	_ = outFile.Close()

	archiveBytes, _ := os.ReadFile(archivePath)
	archiveSha256 := hex.EncodeToString(hasher.Sum(nil))

	archiveName := "sirius-snapshot-remote.tar.zst"
	checksumsText := fmt.Sprintf("%s  %s\n", archiveSha256, archiveName)
	sigBytes := ed25519.Sign(priv, []byte(checksumsText))

	// Setup mock HTTP server standing in for Cloudflare R2 / S3 / B2 storage backend
	var mockServer *httptest.Server
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			manifest := SnapshotManifest{
				Version:         "1.0",
				ChainHeight:     13885390,
				Network:         "mainnet",
				ArchiveName:     archiveName,
				DownloadUrl:     mockServer.URL + "/storage/" + archiveName,
				Sha256:          archiveSha256,
				Format:          "tar.zst",
				UncompressedGB:  0.05,
				CompressedBytes: int64(len(archiveBytes)),
				CreatedAt:       time.Now().UTC().Format(time.RFC3339),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)

		case "/SHA256SUMS":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(checksumsText))

		case "/SHA256SUMS.sig":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(sigBytes)

		case "/storage/" + archiveName:
			w.Header().Set("Content-Type", "application/zstd")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveBytes)))
			_, _ = w.Write(archiveBytes)

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	ctrl := &mockLifecycleController{isRunning: true}
	mgr := NewSnapshotManager(ctrl, pubHex, nil)

	// Run Remote Restore against Mock Server
	manifestUrl := mockServer.URL + "/manifest.json"
	err = mgr.RestoreRemoteSnapshot(manifestUrl, pubHex, restoreTargetDir)
	if err != nil {
		t.Fatalf("RestoreRemoteSnapshot failed to initiate: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		st := mgr.GetStatus()
		if st.Stage == StageCompleted {
			break
		}
		if st.Stage == StageError {
			t.Fatalf("RestoreRemoteSnapshot failed: %s", st.ErrorMessage)
		}
		if time.Now().After(deadline) {
			t.Fatalf("RestoreRemoteSnapshot timed out")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify restored file
	restoredIndex, err := os.ReadFile(filepath.Join(restoreTargetDir, "index.dat"))
	if err != nil || string(restoredIndex) != "STREAMED_REMOTE_BLOCK_DATA_ROCKSDB" {
		t.Fatalf("Remote streamed index.dat verification failed: %v (%s)", err, string(restoredIndex))
	}

	// 4. Negative Test: Corrupted Download Stream fails with ErrChecksumMismatch
	corruptTargetDir := filepath.Join(tempDir, "corrupted_restored")
	corruptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			manifest := SnapshotManifest{
				Version:         "1.0",
				ChainHeight:     13885390,
				Network:         "mainnet",
				ArchiveName:     archiveName,
				DownloadUrl:     mockServer.URL + "/storage/corrupt-" + archiveName,
				Sha256:          archiveSha256,
				Format:          "tar.zst",
				CompressedBytes: int64(len(archiveBytes)),
			}
			_ = json.NewEncoder(w).Encode(manifest)
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(checksumsText))
		case "/SHA256SUMS.sig":
			_, _ = w.Write(sigBytes)
		case "/storage/corrupt-" + archiveName:
			// Send corrupted byte stream
			corruptBytes := make([]byte, len(archiveBytes))
			copy(corruptBytes, archiveBytes)
			if len(corruptBytes) > 20 {
				corruptBytes[20] ^= 0xFF // bit-flip
			}
			_, _ = w.Write(corruptBytes)
		}
	}))
	defer corruptServer.Close()

	mgrCorrupt := NewSnapshotManager(ctrl, pubHex, nil)
	_ = mgrCorrupt.RestoreRemoteSnapshot(corruptServer.URL+"/manifest.json", pubHex, corruptTargetDir)

	deadline = time.Now().Add(5 * time.Second)
	var finalStage OperationStage
	for {
		st := mgrCorrupt.GetStatus()
		if st.Stage == StageError || st.Stage == StageCompleted {
			finalStage = st.Stage
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if finalStage != StageError {
		t.Fatalf("Expected StageError on corrupted remote download stream, got: %s", finalStage)
	}
	_ = archiveBuf
}
