package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

type NodeController interface {
	StopNode() error
	StartNode(dataPath string) error
	IsRunning() bool
}

type SnapshotManager struct {
	mu             sync.RWMutex
	client         *http.Client
	controller     NodeController
	status         SnapshotStatus
	cancelFunc     context.CancelFunc
	auditLogger    func(action, details string)
	defaultPubKey  string
}

func NewSnapshotManager(controller NodeController, defaultPubKey string, auditLogger func(action, details string)) *SnapshotManager {
	if auditLogger == nil {
		auditLogger = func(action, details string) {
			// default silent or stdout
		}
	}
	return &SnapshotManager{
		client: &http.Client{
			Timeout: 0, // Streaming downloads require no total timeout, controlled by context
		},
		controller:    controller,
		defaultPubKey: strings.TrimSpace(defaultPubKey),
		auditLogger:   auditLogger,
		status: SnapshotStatus{
			Stage:   StageIdle,
			Message: "Ready",
		},
	}
}

func (sm *SnapshotManager) GetStatus() SnapshotStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.status
}

func (sm *SnapshotManager) Cancel() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.cancelFunc != nil {
		sm.cancelFunc()
		sm.status.Stage = StageCancelled
		sm.status.Message = "Operation cancelled by operator"
		sm.status.CompletedAt = time.Now()
		return nil
	}
	return nil
}

// CreateLocalSnapshot creates a compressed archive of srcDataPath into targetDestPath.
// It computes the SHA-256 incrementally in a single pass and produces an accompanying manifest.json.
func (sm *SnapshotManager) CreateLocalSnapshot(srcDataPath, targetDestPath, format string, chainHeight int64) (*SnapshotManifest, error) {
	sm.mu.Lock()
	if sm.status.Stage == StageArchiving || sm.status.Stage == StageDownloading || sm.status.Stage == StageExtracting {
		sm.mu.Unlock()
		return nil, ErrSnapshotBusy
	}

	srcDir := filepath.Clean(srcDataPath)
	srcInfo, err := os.Stat(srcDir)
	if err != nil || !srcInfo.IsDir() {
		sm.mu.Unlock()
		return nil, fmt.Errorf("source data directory does not exist or is not a directory: %s", srcDir)
	}

	destDir := targetDestPath
	if destDir == "" {
		destDir = filepath.Dir(srcDir)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		sm.mu.Unlock()
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	cleanFmt := strings.ToLower(strings.TrimSpace(format))
	isZstd := cleanFmt == "" || cleanFmt == "zst" || cleanFmt == "tar.zst" || cleanFmt == "zstandard"
	ext := ".tar.zst"
	fmtName := "tar.zst"
	if !isZstd {
		ext = ".tar.gz"
		fmtName = "tar.gz"
	}

	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("sirius-snapshot-h%d-%s%s", chainHeight, timestamp, ext)
	targetArchiveFile := filepath.Join(destDir, archiveName)

	ctx, cancel := context.WithCancel(context.Background())
	sm.cancelFunc = cancel
	sm.status = SnapshotStatus{
		Stage:      StageArchiving,
		Operation:  "create",
		Message:    "Scanning blockchain data directory...",
		TargetFile: targetArchiveFile,
		StartedAt:  time.Now(),
		Progress:   ProgressInfo{Percentage: 0},
	}
	sm.mu.Unlock()

	sm.auditLogger("SNAPSHOT_CREATE_START", fmt.Sprintf("Source: %s, Target: %s, Format: %s", srcDir, targetArchiveFile, fmtName))

	go func() {
		defer cancel()

		var totalUncompressedBytes int64
		var fileList []string

		_ = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			name := info.Name()
			if name == "server.lock" || strings.HasSuffix(name, ".tmp") || name == ".DS_Store" {
				return nil
			}
			if !info.IsDir() {
				totalUncompressedBytes += info.Size()
			}
			fileList = append(fileList, path)
			return nil
		})

		sm.mu.Lock()
		sm.status.Progress.TotalBytes = totalUncompressedBytes
		sm.status.Message = fmt.Sprintf("Archiving %d items (%s uncompressed)...", len(fileList), formatBytes(totalUncompressedBytes))
		sm.mu.Unlock()

		outFile, err := os.Create(targetArchiveFile)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to create archive file: %v", err))
			return
		}
		defer outFile.Close()

		hasher := sha256.New()
		// Tee output to both file and SHA-256 hasher simultaneously
		multiOut := io.MultiWriter(outFile, hasher)

		var compWriter io.WriteCloser
		if isZstd {
			zw, errZ := zstd.NewWriter(multiOut, zstd.WithEncoderConcurrency(0), zstd.WithEncoderLevel(zstd.SpeedDefault))
			if errZ != nil {
				sm.setError(fmt.Sprintf("Failed to initialize Zstandard encoder: %v", errZ))
				return
			}
			compWriter = zw
		} else {
			compWriter = gzip.NewWriter(multiOut)
		}

		tarWriter := tar.NewWriter(compWriter)
		var writtenRawBytes int64
		startTime := time.Now()
		lastUpdate := time.Now()

		for _, filePath := range fileList {
			select {
			case <-ctx.Done():
				_ = tarWriter.Close()
				_ = compWriter.Close()
				_ = outFile.Close()
				_ = os.Remove(targetArchiveFile)
				sm.mu.Lock()
				sm.status.Stage = StageCancelled
				sm.status.Message = "Snapshot creation cancelled"
				sm.mu.Unlock()
				return
			default:
			}

			relPath, err := filepath.Rel(srcDir, filePath)
			if err != nil {
				continue
			}

			info, err := os.Lstat(filePath)
			if err != nil {
				continue
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				continue
			}
			header.Name = relPath

			if err := tarWriter.WriteHeader(header); err != nil {
				sm.setError(fmt.Sprintf("Tar write header error: %v", err))
				return
			}

			if info.Mode().IsRegular() {
				file, err := os.Open(filePath)
				if err != nil {
					continue
				}
				n, err := io.Copy(tarWriter, file)
				file.Close()
				if err != nil {
					sm.setError(fmt.Sprintf("Tar write data error: %v", err))
					return
				}
				writtenRawBytes += n
			}

			if time.Since(lastUpdate) >= 500*time.Millisecond {
				lastUpdate = time.Now()
				elapsed := time.Since(startTime).Seconds()
				speed := float64(writtenRawBytes) / (1024 * 1024 * elapsed)
				var percentage float64
				var eta int64
				if totalUncompressedBytes > 0 {
					percentage = (float64(writtenRawBytes) / float64(totalUncompressedBytes)) * 100
					rem := totalUncompressedBytes - writtenRawBytes
					if speed > 0 {
						eta = int64(float64(rem) / (speed * 1024 * 1024))
					}
				}

				sm.mu.Lock()
				sm.status.Progress.ProcessedBytes = writtenRawBytes
				sm.status.Progress.Percentage = percentage
				sm.status.Progress.SpeedMBs = speed
				sm.status.Progress.ETASeconds = eta
				sm.status.Progress.CurrentItem = relPath
				sm.status.Message = fmt.Sprintf("Archiving: %.1f%% (%s) at %.1f MB/s", percentage, formatBytes(writtenRawBytes), speed)
				sm.mu.Unlock()
			}
		}

		_ = tarWriter.Close()
		_ = compWriter.Close()
		_ = outFile.Close()

		compressedInfo, err := os.Stat(targetArchiveFile)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to read archive stats: %v", err))
			return
		}

		sha256Hex := hex.EncodeToString(hasher.Sum(nil))
		uncompressedGB := float64(totalUncompressedBytes) / (1024 * 1024 * 1024)

		manifest := &SnapshotManifest{
			Version:         "1.0",
			ChainHeight:     chainHeight,
			Network:         "mainnet",
			ArchiveName:     archiveName,
			DownloadUrl:     "", // Pluggable: will be set upon remote storage publish
			Sha256:          sha256Hex,
			Format:          fmtName,
			UncompressedGB:  uncompressedGB,
			CompressedBytes: compressedInfo.Size(),
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		}

		// Save companion manifest.json next to archive
		manifestPath := filepath.Join(destDir, fmt.Sprintf("manifest-h%d.json", chainHeight))
		manifestData, _ := json.MarshalIndent(manifest, "", "  ")
		_ = os.WriteFile(manifestPath, manifestData, 0644)

		sm.mu.Lock()
		sm.status.Stage = StageCompleted
		sm.status.Progress.Percentage = 100
		sm.status.Manifest = manifest
		sm.status.Message = fmt.Sprintf("Snapshot created successfully (%s, SHA-256: %s...)", formatBytes(compressedInfo.Size()), sha256Hex[:12])
		sm.status.CompletedAt = time.Now()
		sm.mu.Unlock()

		sm.auditLogger("SNAPSHOT_CREATE_SUCCESS", fmt.Sprintf("Archive: %s (%s, sha256: %s)", targetArchiveFile, formatBytes(compressedInfo.Size()), sha256Hex))
	}()

	return nil, nil
}

// RestoreLocalSnapshot restores an on-disk archive (.tar.zst, .tar.gz, .tar.xz) into targetDataPath.
func (sm *SnapshotManager) RestoreLocalSnapshot(archivePath, targetDataPath string) error {
	sm.mu.Lock()
	if sm.status.Stage == StageArchiving || sm.status.Stage == StageDownloading || sm.status.Stage == StageExtracting {
		sm.mu.Unlock()
		return ErrSnapshotBusy
	}

	cleanArchive := filepath.Clean(archivePath)
	fileInfo, err := os.Stat(cleanArchive)
	if err != nil || fileInfo.IsDir() {
		sm.mu.Unlock()
		return fmt.Errorf("snapshot archive not found: %s", cleanArchive)
	}

	targetDir := filepath.Clean(targetDataPath)
	if targetDir == "" {
		sm.mu.Unlock()
		return fmt.Errorf("target data path cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sm.cancelFunc = cancel
	sm.status = SnapshotStatus{
		Stage:      StageExtracting,
		Operation:  "restore_local",
		Message:    fmt.Sprintf("Preparing extraction of %s...", filepath.Base(cleanArchive)),
		TargetFile: cleanArchive,
		StartedAt:  time.Now(),
		Progress: ProgressInfo{
			TotalBytes: fileInfo.Size(),
		},
	}
	sm.mu.Unlock()

	sm.auditLogger("SNAPSHOT_RESTORE_LOCAL_START", fmt.Sprintf("Archive: %s -> Destination: %s", cleanArchive, targetDir))

	go func() {
		defer cancel()

		if sm.controller != nil && sm.controller.IsRunning() {
			_ = sm.controller.StopNode()
		}

		_ = os.MkdirAll(targetDir, 0755)
		_ = os.Remove(filepath.Join(targetDir, "server.lock"))

		file, err := os.Open(cleanArchive)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to open archive: %v", err))
			return
		}
		defer file.Close()

		if err := sm.extractArchiveStream(ctx, file, fileInfo.Size(), cleanArchive, targetDir); err != nil {
			if ctx.Err() == context.Canceled {
				sm.mu.Lock()
				sm.status.Stage = StageCancelled
				sm.status.Message = "Restore operation cancelled"
				sm.mu.Unlock()
				return
			}
			sm.setError(fmt.Sprintf("Extraction failed: %v", err))
			return
		}

		_ = os.Remove(filepath.Join(targetDir, "server.lock"))

		sm.mu.Lock()
		sm.status.Stage = StageCompleted
		sm.status.Progress.Percentage = 100
		sm.status.Message = fmt.Sprintf("Snapshot %s restored successfully into %s", filepath.Base(cleanArchive), targetDir)
		sm.status.CompletedAt = time.Now()
		sm.mu.Unlock()

		sm.auditLogger("SNAPSHOT_RESTORE_LOCAL_SUCCESS", fmt.Sprintf("Extracted %s into %s", cleanArchive, targetDir))
	}()

	return nil
}

// RestoreRemoteSnapshot coordinates the generic, backend-agnostic remote snapshot flow:
// 1. Fetches manifest.json from manifestUrl.
// 2. Fetches SHA256SUMS and SHA256SUMS.sig (sibling files of manifest or release tag).
// 3. Verifies Ed25519 signature over SHA256SUMS using releasePubKeyHex.
// 4. Verifies manifest.Sha256 matches SHA256SUMS.
// 5. Streams download directly from manifest.DownloadUrl (R2, B2, S3, or any HTTPS server).
// 6. Simultaneously verifies SHA-256 incrementally and decompresses on the fly into targetDataPath.
func (sm *SnapshotManager) RestoreRemoteSnapshot(manifestUrl, releasePubKeyHex, targetDataPath string) error {
	sm.mu.Lock()
	if sm.status.Stage == StageArchiving || sm.status.Stage == StageDownloading || sm.status.Stage == StageExtracting {
		sm.mu.Unlock()
		return ErrSnapshotBusy
	}

	pubKey := strings.TrimSpace(releasePubKeyHex)
	if pubKey == "" {
		pubKey = sm.defaultPubKey
	}
	if pubKey == "" {
		sm.mu.Unlock()
		return fmt.Errorf("release public key is required for cryptographic signature verification")
	}

	targetDir := filepath.Clean(targetDataPath)
	if targetDir == "" {
		sm.mu.Unlock()
		return fmt.Errorf("target data path cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sm.cancelFunc = cancel
	sm.status = SnapshotStatus{
		Stage:     StageFetchingManifest,
		Operation: "restore_remote",
		Message:   "Fetching verified snapshot manifest...",
		StartedAt: time.Now(),
	}
	sm.mu.Unlock()

	sm.auditLogger("SNAPSHOT_RESTORE_REMOTE_START", fmt.Sprintf("Manifest: %s, Target: %s", manifestUrl, targetDir))

	go func() {
		defer cancel()

		// 1. Fetch manifest.json
		manifest, err := sm.fetchManifest(ctx, manifestUrl)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to fetch snapshot manifest: %v", err))
			return
		}

		sm.mu.Lock()
		sm.status.Manifest = manifest
		sm.status.Stage = StageVerifyingSig
		sm.status.Message = "Verifying release signature (Ed25519)..."
		sm.mu.Unlock()

		// 2. Fetch SHA256SUMS and SHA256SUMS.sig from manifest's base URL
		baseUrl := manifestUrl[:strings.LastIndex(manifestUrl, "/")+1]
		checksumsUrl := baseUrl + "SHA256SUMS"
		sigUrl := baseUrl + "SHA256SUMS.sig"

		checksumsData, err := sm.fetchBytes(ctx, checksumsUrl)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to fetch SHA256SUMS: %v", err))
			return
		}

		sigData, err := sm.fetchBytes(ctx, sigUrl)
		if err != nil {
			sm.setError(fmt.Sprintf("Failed to fetch SHA256SUMS.sig: %v", err))
			return
		}

		// 3. Cryptographically verify Ed25519 signature
		if err := VerifyEd25519Signature(pubKey, checksumsData, sigData); err != nil {
			sm.setError(fmt.Sprintf("Cryptographic signature check failed: %v", err))
			return
		}

		// 4. Verify archive checksum entry in SHA256SUMS matches manifest
		expectedHash, err := ParseSha256Sums(string(checksumsData), manifest.ArchiveName)
		if err != nil {
			sm.setError(fmt.Sprintf("Checksums file missing entry for %s: %v", manifest.ArchiveName, err))
			return
		}
		if !strings.EqualFold(expectedHash, manifest.Sha256) {
			sm.setError(fmt.Sprintf("Manifest checksum %s does not match signed checksum %s", manifest.Sha256, expectedHash))
			return
		}

		// 5. Stop node cleanly and prepare destination
		if sm.controller != nil && sm.controller.IsRunning() {
			_ = sm.controller.StopNode()
		}
		_ = os.MkdirAll(targetDir, 0755)
		_ = os.Remove(filepath.Join(targetDir, "server.lock"))

		// 6. Connect to pluggable DownloadUrl
		sm.mu.Lock()
		sm.status.Stage = StageDownloading
		sm.status.Message = fmt.Sprintf("Connecting to snapshot storage (%s)...", manifest.ArchiveName)
		sm.mu.Unlock()

		req, err := http.NewRequestWithContext(ctx, "GET", manifest.DownloadUrl, nil)
		if err != nil {
			sm.setError(fmt.Sprintf("Invalid download URL: %v", err))
			return
		}

		resp, err := sm.client.Do(req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				sm.mu.Lock()
				sm.status.Stage = StageCancelled
				sm.status.Message = "Download cancelled"
				sm.mu.Unlock()
				return
			}
			sm.setError(fmt.Sprintf("Failed to connect to storage: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			sm.setError(fmt.Sprintf("Storage server returned HTTP %d: %s", resp.StatusCode, resp.Status))
			return
		}

		contentLength := resp.ContentLength
		if contentLength <= 0 {
			contentLength = manifest.CompressedBytes
		}

		sm.mu.Lock()
		sm.status.Progress.TotalBytes = contentLength
		sm.status.Message = fmt.Sprintf("Streaming & extracting %s (%.1f GB uncompressed)...", manifest.ArchiveName, manifest.UncompressedGB)
		sm.mu.Unlock()

		// 7. Tee download body to hasher for on-the-fly checksum verification
		hasher := sha256.New()
		teeReader := io.TeeReader(resp.Body, hasher)

		// Extract directly from stream
		if err := sm.extractArchiveStream(ctx, teeReader, contentLength, manifest.ArchiveName, targetDir); err != nil {
			if ctx.Err() == context.Canceled {
				sm.mu.Lock()
				sm.status.Stage = StageCancelled
				sm.status.Message = "Snapshot restoration cancelled"
				sm.mu.Unlock()
				return
			}
			sm.setError(fmt.Sprintf("Streaming extraction failed: %v", err))
			return
		}

		// 8. Verify final SHA-256 hash computed over entire download stream
		calculatedSha := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(calculatedSha, manifest.Sha256) {
			sm.setError(fmt.Sprintf("%v: calculated %s, expected %s", ErrChecksumMismatch, calculatedSha, manifest.Sha256))
			return
		}

		_ = os.Remove(filepath.Join(targetDir, "server.lock"))

		sm.mu.Lock()
		sm.status.Stage = StageCompleted
		sm.status.Progress.Percentage = 100
		sm.status.Message = fmt.Sprintf("Verified snapshot restored successfully at block height %d", manifest.ChainHeight)
		sm.status.CompletedAt = time.Now()
		sm.mu.Unlock()

		sm.auditLogger("SNAPSHOT_RESTORE_REMOTE_SUCCESS", fmt.Sprintf("Height: %d, Sha256: %s, Dest: %s", manifest.ChainHeight, calculatedSha, targetDir))
	}()

	return nil
}

func (sm *SnapshotManager) extractArchiveStream(ctx context.Context, reader io.Reader, totalBytes int64, archiveName, targetDir string) error {
	lowerName := strings.ToLower(archiveName)

	if strings.HasSuffix(lowerName, ".tar.xz") || strings.HasSuffix(lowerName, ".xz") {
		// Use system tar command for direct streaming .tar.xz extraction (tar -xJf -)
		tarCmd := exec.CommandContext(ctx, "tar", "-xJf", "-", "-C", targetDir)
		stdinPipe, err := tarCmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := tarCmd.Start(); err != nil {
			return err
		}

		buf := make([]byte, 512*1024)
		var processed int64
		startTime := time.Now()
		lastUpdate := time.Now()

		for {
			select {
			case <-ctx.Done():
				_ = stdinPipe.Close()
				_ = tarCmd.Process.Kill()
				return ctx.Err()
			default:
			}

			n, rErr := reader.Read(buf)
			if n > 0 {
				if _, wErr := stdinPipe.Write(buf[:n]); wErr != nil {
					return wErr
				}
				processed += int64(n)

				if time.Since(lastUpdate) >= 500*time.Millisecond {
					lastUpdate = time.Now()
					sm.updateProgress(processed, totalBytes, startTime)
				}
			}
			if rErr != nil {
				if rErr == io.EOF {
					break
				}
				return rErr
			}
		}

		_ = stdinPipe.Close()
		return tarCmd.Wait()
	}

	// For .tar.zst, .tar.gz, .tar
	var compReader io.Reader
	var closeComp func() error

	if strings.HasSuffix(lowerName, ".tar.zst") || strings.HasSuffix(lowerName, ".zst") {
		zstdReader, err := zstd.NewReader(reader)
		if err != nil {
			return fmt.Errorf("zstd decoder error: %w", err)
		}
		compReader = zstdReader
		closeComp = func() error { zstdReader.Close(); return nil }
	} else if strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("gzip decoder error: %w", err)
		}
		compReader = gzReader
		closeComp = gzReader.Close
	} else {
		compReader = reader
	}

	if closeComp != nil {
		defer closeComp()
	}

	tarReader := tar.NewReader(compReader)
	startTime := time.Now()
	lastUpdate := time.Now()
	var processedBytes int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanedPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanedPath, "..") || filepath.IsAbs(cleanedPath) {
			continue
		}

		targetFile := filepath.Join(targetDir, cleanedPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetFile, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(targetFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			n, err := io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return err
			}
			processedBytes += n

			if time.Since(lastUpdate) >= 500*time.Millisecond {
				lastUpdate = time.Now()
				sm.updateProgress(processedBytes, totalBytes, startTime)
			}
		}
	}

	return nil
}

func (sm *SnapshotManager) updateProgress(current, total int64, startTime time.Time) {
	elapsed := time.Since(startTime).Seconds()
	speed := float64(current) / (1024 * 1024 * elapsed)
	var percentage float64
	var eta int64
	if total > 0 {
		percentage = (float64(current) / float64(total)) * 100
		rem := total - current
		if speed > 0 {
			eta = int64(float64(rem) / (speed * 1024 * 1024))
		}
	}

	sm.mu.Lock()
	sm.status.Progress.ProcessedBytes = current
	sm.status.Progress.Percentage = percentage
	sm.status.Progress.SpeedMBs = speed
	sm.status.Progress.ETASeconds = eta
	sm.status.Message = fmt.Sprintf("Processing: %.1f%% (%s) at %.1f MB/s", percentage, formatBytes(current), speed)
	sm.mu.Unlock()
}

func (sm *SnapshotManager) fetchManifest(ctx context.Context, url string) (*SnapshotManifest, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := sm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var manifest SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err == nil && manifest.ArchiveName != "" {
		return &manifest, nil
	}

	var envelope struct {
		Manifest SnapshotManifest `json:"manifest"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Manifest.ArchiveName != "" {
		return &envelope.Manifest, nil
	}

	return nil, fmt.Errorf("invalid manifest format: could not find valid snapshot manifest in response")
}

func (sm *SnapshotManager) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := sm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (sm *SnapshotManager) setError(msg string) {
	sm.mu.Lock()
	sm.status.Stage = StageError
	sm.status.ErrorMessage = msg
	sm.status.Message = msg
	sm.status.CompletedAt = time.Now()
	sm.mu.Unlock()
	sm.auditLogger("SNAPSHOT_ERROR", msg)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
