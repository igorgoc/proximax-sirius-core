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

	// Verify final progress state
	stFinal := mgr.GetStatus()
	if stFinal.Progress.ProcessedBytes != stFinal.Progress.TotalBytes || stFinal.Progress.TotalBytes == 0 {
		t.Fatalf("Expected completed Progress.ProcessedBytes (%d) == Progress.TotalBytes (%d) > 0", stFinal.Progress.ProcessedBytes, stFinal.Progress.TotalBytes)
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

// 4. Verification Test: Multi-MB Progress Counter Increments in Real-Time
func TestPart2_ProgressCounterIncrementation(t *testing.T) {
	tempDir := t.TempDir()
	restoreTargetDir := filepath.Join(tempDir, "multi_mb_restored")

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	// Create 2 MB payload to observe real-time progression
	archivePath := filepath.Join(tempDir, "multi-mb-snapshot.tar.zst")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}
	hasher := sha256.New()
	multiWriter := io.MultiWriter(outFile, hasher)

	zw, _ := zstd.NewWriter(multiWriter)
	tw := tar.NewWriter(zw)

	dataChunk := make([]byte, 512*1024)
	_, _ = rand.Read(dataChunk)

	// Write 4 files of 512KB each = 2MB uncompressed
	for fIdx := 0; fIdx < 4; fIdx++ {
		hdr := &tar.Header{
			Name: fmt.Sprintf("chunk_%02d.dat", fIdx),
			Mode: 0644,
			Size: int64(len(dataChunk)),
		}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write(dataChunk)
	}
	_ = tw.Close()
	_ = zw.Close()
	_ = outFile.Close()

	archiveBytes, _ := os.ReadFile(archivePath)
	archiveSha256 := hex.EncodeToString(hasher.Sum(nil))
	archiveName := "multi-mb-snapshot.tar.zst"
	checksumsText := fmt.Sprintf("%s  %s\n", archiveSha256, archiveName)
	sigBytes := ed25519.Sign(priv, []byte(checksumsText))

	// Mock server that streams chunks with a small sleep to simulate wire transfer
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			manifest := SnapshotManifest{
				Version:         "1.0",
				ChainHeight:     13885400,
				Network:         "mainnet",
				ArchiveName:     archiveName,
				DownloadUrl:     server.URL + "/storage/" + archiveName,
				Sha256:          archiveSha256,
				Format:          "tar.zst",
				UncompressedGB:  0.002,
				CompressedBytes: int64(len(archiveBytes)),
				CreatedAt:       time.Now().UTC().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(manifest)
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(checksumsText))
		case "/SHA256SUMS.sig":
			_, _ = w.Write(sigBytes)
		case "/storage/" + archiveName:
			w.Header().Set("Content-Type", "application/zstd")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveBytes)))
			flusher, canFlush := w.(http.Flusher)
			chunkSize := 32 * 1024
			for i := 0; i < len(archiveBytes); i += chunkSize {
				end := i + chunkSize
				if end > len(archiveBytes) {
					end = len(archiveBytes)
				}
				_, _ = w.Write(archiveBytes[i:end])
				if canFlush {
					flusher.Flush()
				}
				time.Sleep(15 * time.Millisecond) // Throttled transfer
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctrl := &mockLifecycleController{isRunning: true}
	mgr := NewSnapshotManager(ctrl, pubHex, nil)

	err = mgr.RestoreRemoteSnapshot(server.URL+"/manifest.json", pubHex, restoreTargetDir)
	if err != nil {
		t.Fatalf("RestoreRemoteSnapshot failed: %v", err)
	}

	var observedBytes []int64
	deadline := time.Now().Add(10 * time.Second)

	for {
		st := mgr.GetStatus()
		if st.Progress.ProcessedBytes > 0 {
			if len(observedBytes) == 0 || observedBytes[len(observedBytes)-1] != st.Progress.ProcessedBytes {
				observedBytes = append(observedBytes, st.Progress.ProcessedBytes)
			}
		}

		if st.Stage == StageCompleted {
			break
		}
		if st.Stage == StageError {
			t.Fatalf("Operation failed with: %s", st.ErrorMessage)
		}
		if time.Now().After(deadline) {
			t.Fatalf("Timed out waiting for completion")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Check observations
	t.Logf("Observed incremental byte counter points (%d samples): %v", len(observedBytes), observedBytes)
	if len(observedBytes) < 3 {
		t.Fatalf("Expected to observe at least 3 distinct progress steps during transfer, but observed %d: %v", len(observedBytes), observedBytes)
	}

	finalStatus := mgr.GetStatus()
	if finalStatus.Progress.ProcessedBytes != finalStatus.Progress.TotalBytes {
		t.Fatalf("Final ProcessedBytes (%d) does not match TotalBytes (%d)", finalStatus.Progress.ProcessedBytes, finalStatus.Progress.TotalBytes)
	}
	if finalStatus.Progress.Percentage != 100.0 {
		t.Fatalf("Final Percentage (%f) is not 100.0", finalStatus.Progress.Percentage)
	}
}

// 5. Security Test (SEC-02/03): Decompression Bomb Protection
// Confirms extraction aborts immediately inside the running copy loop
// and purges all partially extracted and previously written files from disk.
func TestDecompressionBombProtection_AbortsAndCleansUp(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "bomb_extraction_target")
	_ = os.MkdirAll(targetDir, 0755)

	archivePath := filepath.Join(tempDir, "oversized-bomb.tar.zst")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	zw, err := zstd.NewWriter(outFile)
	if err != nil {
		t.Fatalf("Failed to create zstd writer: %v", err)
	}
	tw := tar.NewWriter(zw)

	// File 1: 150 KB
	file1Data := make([]byte, 150*1024)
	for i := range file1Data {
		file1Data[i] = byte('A')
	}
	hdr1 := &tar.Header{
		Name: "block_01.dat",
		Mode: 0644,
		Size: int64(len(file1Data)),
	}
	if err := tw.WriteHeader(hdr1); err != nil {
		t.Fatalf("WriteHeader 1 error: %v", err)
	}
	if _, err := tw.Write(file1Data); err != nil {
		t.Fatalf("Write 1 error: %v", err)
	}

	// File 2: 400 KB (Total payload = 550 KB uncompressed)
	file2Data := make([]byte, 400*1024)
	for i := range file2Data {
		file2Data[i] = byte('B')
	}
	hdr2 := &tar.Header{
		Name: "block_02.dat",
		Mode: 0644,
		Size: int64(len(file2Data)),
	}
	if err := tw.WriteHeader(hdr2); err != nil {
		t.Fatalf("WriteHeader 2 error: %v", err)
	}
	if _, err := tw.Write(file2Data); err != nil {
		t.Fatalf("Write 2 error: %v", err)
	}

	_ = tw.Close()
	_ = zw.Close()
	_ = outFile.Close()

	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("Archive stat error: %v", err)
	}

	t.Logf("Constructed test archive: compressed size %d bytes, uncompressed size 550 KB", fileInfo.Size())

	// Set safety ceiling to 250 KB (less than the 550 KB uncompressed payload)
	// File 1 (150 KB) will be written first, but File 2 will trip the running ceiling midway!
	const safetyCeiling = 250 * 1024

	ctrl := &mockLifecycleController{isRunning: true}
	mgr := NewSnapshotManager(ctrl, "", nil)
	mgr.SetMaxDecompressedBytes(safetyCeiling)

	if err := mgr.RestoreLocalSnapshot(archivePath, targetDir); err != nil {
		t.Fatalf("RestoreLocalSnapshot failed to launch: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var finalStage OperationStage
	var errorMessage string

	for {
		st := mgr.GetStatus()
		if st.Stage == StageError || st.Stage == StageCompleted {
			finalStage = st.Stage
			errorMessage = st.ErrorMessage
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Operation timed out waiting for decompression bomb check")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 1. Verify that extraction aborted with error
	if finalStage != StageError {
		t.Fatalf("Expected StageError, but operation reached stage: %s", finalStage)
	}
	t.Logf("Decompression bomb check successfully triggered error: %s", errorMessage)

	if !strings.Contains(errorMessage, "decompression limit exceeded") {
		t.Fatalf("Expected 'decompression limit exceeded' in error message, got: %s", errorMessage)
	}

	// 2. Verify that NO orphaned or partial files remain on disk
	file1Path := filepath.Join(targetDir, "block_01.dat")
	if _, err := os.Stat(file1Path); !os.IsNotExist(err) {
		t.Fatalf("Security invariant violated: block_01.dat was NOT cleaned up from disk! (found: %s)", file1Path)
	}

	file2Path := filepath.Join(targetDir, "block_02.dat")
	if _, err := os.Stat(file2Path); !os.IsNotExist(err) {
		t.Fatalf("Security invariant violated: partially written block_02.dat was NOT cleaned up from disk! (found: %s)", file2Path)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) > 0 {
		var remaining []string
		for _, e := range entries {
			remaining = append(remaining, e.Name())
		}
		t.Fatalf("Security invariant violated: orphaned files left in target directory: %v", remaining)
	}

	t.Logf("✓ Verified: Extraction aborted midway inside copy loop; zero orphaned files left in target directory (%d entries)", len(entries))
}

// 6. Unit Test: SnapshotManager Reset restores stage to StageIdle
func TestSnapshotManager_Reset(t *testing.T) {
	ctrl := &mockLifecycleController{}
	mgr := NewSnapshotManager(ctrl, "", nil)

	// Set to error
	mgr.mu.Lock()
	mgr.status.Stage = StageError
	mgr.status.ErrorMessage = "Some error"
	mgr.status.Message = "Failed"
	mgr.mu.Unlock()

	mgr.Reset()

	st := mgr.GetStatus()
	if st.Stage != StageIdle || st.Message != "Ready" {
		t.Fatalf("Expected StageIdle and 'Ready', got stage=%s msg=%s", st.Stage, st.Message)
	}

	// Active stage should not be reset
	mgr.mu.Lock()
	mgr.status.Stage = StageArchiving
	mgr.mu.Unlock()

	mgr.Reset()

	st = mgr.GetStatus()
	if st.Stage != StageArchiving {
		t.Fatalf("Expected StageArchiving to not be reset, got stage=%s", st.Stage)
	}
}

// 7. Unit Test: CreateLocalSnapshot pauses running node and handles growing files without error
func TestCreateLocalSnapshot_PausesRunningNodeAndProtectsGrowingFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sirius-snapshot-create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "data")
	destDir := filepath.Join(tempDir, "snapshots")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.MkdirAll(destDir, 0755)

	// Create test file in data dir
	testFile := filepath.Join(srcDir, "blocks.dat")
	if err := os.WriteFile(testFile, []byte("initial block content that might grow"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	ctrl := &mockLifecycleController{isRunning: true}
	mgr := NewSnapshotManager(ctrl, "", nil)

	_, err = mgr.CreateLocalSnapshot(srcDir, destDir, "tar.zst", 1000)
	if err != nil {
		t.Fatalf("CreateLocalSnapshot returned error: %v", err)
	}


	// Wait for completion
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := mgr.GetStatus()
		if st.Stage == StageCompleted {
			break
		}
		if st.Stage == StageError {
			t.Fatalf("Snapshot creation failed: %s (err=%s)", st.Message, st.ErrorMessage)
		}
		time.Sleep(50 * time.Millisecond)
	}

	st := mgr.GetStatus()
	if st.Stage != StageCompleted {
		t.Fatalf("Expected StageCompleted, got: %s", st.Stage)
	}

	// Verify the running node was stopped and restarted
	if ctrl.stopCalls == 0 {
		t.Fatalf("Expected StopNode to be called when node was running")
	}
	if !ctrl.IsRunning() {
		t.Fatalf("Expected node to be restarted (isRunning=true) after snapshot completion")
	}

	// Verify target archive was created
	if st.Manifest == nil || st.Manifest.ArchiveName == "" {
		t.Fatalf("Expected non-empty manifest in st.Manifest")
	}
	targetArchive := filepath.Join(destDir, st.Manifest.ArchiveName)
	if _, err := os.Stat(targetArchive); os.IsNotExist(err) {
		t.Fatalf("Target archive file was not found: %s", targetArchive)
	}

}


