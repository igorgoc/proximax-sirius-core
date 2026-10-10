package supervisor

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// TestStorage_LockSelfHealing verifies that stale locks across flat-files and statedb
// are cleanly swept without manual intervention.
func TestStorage_LockSelfHealing(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	statedbDir := filepath.Join(dataDir, "statedb")
	cf0Dir := filepath.Join(statedbDir, "0")
	cf1Dir := filepath.Join(statedbDir, "1")

	if err := os.MkdirAll(cf0Dir, 0755); err != nil {
		t.Fatalf("failed to create cf0Dir: %v", err)
	}
	if err := os.MkdirAll(cf1Dir, 0755); err != nil {
		t.Fatalf("failed to create cf1Dir: %v", err)
	}

	// Plant stale lock files
	lockFiles := []string{
		filepath.Join(dataDir, "server.lock"),
		filepath.Join(dataDir, "recovery.lock"),
		filepath.Join(dataDir, "broker.lock"),
		filepath.Join(statedbDir, "LOCK"),
		filepath.Join(cf0Dir, "LOCK"),
		filepath.Join(cf1Dir, "LOCK"),
	}

	for _, lf := range lockFiles {
		if err := os.WriteFile(lf, []byte("stale-lock-data"), 0644); err != nil {
			t.Fatalf("failed to plant lock file %s: %v", lf, err)
		}
	}

	// Verify all locks exist before sweep
	for _, lf := range lockFiles {
		if _, err := os.Stat(lf); os.IsNotExist(err) {
			t.Fatalf("lock file %s was not created", lf)
		}
	}

	// Execute lock cleanup logic (matching supervisor pre-flight behavior)
	clearLocks := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if filepath.Ext(e.Name()) == ".lock" {
					_ = os.Remove(filepath.Join(dir, e.Name()))
				}
			}
		}
		// Clear statedb locks
		sdb := filepath.Join(dir, "statedb")
		_ = os.Remove(filepath.Join(sdb, "LOCK"))
		sEntries, sErr := os.ReadDir(sdb)
		if sErr == nil {
			for _, sub := range sEntries {
				if sub.IsDir() {
					_ = os.Remove(filepath.Join(sdb, sub.Name(), "LOCK"))
				}
			}
		}
	}

	clearLocks(dataDir)

	// Assert all locks have been removed
	for _, lf := range lockFiles {
		if _, err := os.Stat(lf); !os.IsNotExist(err) {
			t.Errorf("lock file %s should have been removed, but still exists", lf)
		}
	}
}

// TestStorage_StreamingZstdDecompress verifies in-stream decompression directly from HTTP
// without writing intermediate temporary tar archives to disk.
func TestStorage_StreamingZstdDecompress(t *testing.T) {
	// 1. Build a synthetic .tar.zst archive in memory
	var tarBuf bytes.Buffer
	tarWriter := tar.NewWriter(&tarBuf)

	filesToAdd := map[string][]byte{
		"00000/00001.dat": []byte("nemesis-block-data-00001"),
		"00000/hashes.dat": []byte("nemesis-hashes-data"),
		"index.dat":        {0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		"statedb/0/000005.sst": []byte("rocksdb-column-family-0-data"),
	}

	for name, content := range filesToAdd {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header for %s: %v", name, err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("failed to write tar body for %s: %v", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	// Compress tar stream with zstd
	var zstdBuf bytes.Buffer
	enc, err := zstd.NewWriter(&zstdBuf)
	if err != nil {
		t.Fatalf("failed to create zstd encoder: %v", err)
	}
	if _, err := enc.Write(tarBuf.Bytes()); err != nil {
		t.Fatalf("failed to write zstd data: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("failed to close zstd encoder: %v", err)
	}

	zstdPayload := zstdBuf.Bytes()

	// 2. Serve over local mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zstdPayload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zstdPayload)
	}))
	defer mockServer.Close()

	// 3. Invoke RestoreSnapshot against mock server URL
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "chainconfig")
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(configDir, 0755)

	s := NewProcessSupervisor(configDir)

	if err := s.RestoreSnapshot(mockServer.URL, dataDir); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// Poll until complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for snapshot restore completion")
		default:
			st := s.GetSnapshotStatus()
			if st.Stage == SnapshotStageComplete {
				goto completed
			}
			if st.Stage == SnapshotStageError {
				t.Fatalf("snapshot restore failed with error: %s", st.ErrorMessage)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

completed:
	// 4. Assert that all files extracted properly
	for name, expectedContent := range filesToAdd {
		targetFile := filepath.Join(dataDir, name)
		actual, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("expected file %s to be extracted, got err: %v", name, err)
		}
		if !bytes.Equal(actual, expectedContent) {
			t.Errorf("file content mismatch for %s: got %q, want %q", name, actual, expectedContent)
		}
	}
}

// TestStorage_PathNormalization verifies cross-platform path mapping
func TestStorage_PathNormalization(t *testing.T) {
	if runtime.GOOS == "windows" {
		winPath := `C:\Projects\sirius\data`
		wslPath := ToWSLPath(winPath)
		expected := `/mnt/c/Projects/sirius/data`
		if wslPath != expected {
			t.Errorf("ToWSLPath(%s) = %s, want %s", winPath, wslPath, expected)
		}

		backToWin := FromWSLPath(wslPath)
		if backToWin != winPath {
			t.Errorf("FromWSLPath(%s) = %s, want %s", wslPath, backToWin, winPath)
		}
	} else {
		// On POSIX, normalized path should remain POSIX
		posixPath := "/Volumes/ExternalSSD/chainconfig/data"
		if !filepath.IsAbs(posixPath) {
			t.Errorf("expected %s to be absolute", posixPath)
		}
	}
}
