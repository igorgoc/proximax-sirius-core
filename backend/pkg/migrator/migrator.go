package migrator

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MigrationStatus struct {
	Status          string `json:"status"` // idle, running, completed, failed, cancelled
	ProcessedDirs   int    `json:"processedDirs"`
	TotalDirs       int    `json:"totalDirs"`
	ConvertedBlocks int64  `json:"convertedBlocks"`
	DeletedFiles    int64  `json:"deletedFiles"`
	BytesWritten    int64  `json:"bytesWritten"`
	Percent         int    `json:"percent"`
	CurrentDir      string `json:"currentDir"`
	Message         string `json:"message"`
	Error           string `json:"error,omitempty"`
}

type Migrator struct {
	mu        sync.RWMutex
	status    MigrationStatus
	cancel    chan struct{}
	isRunning bool
}

func New() *Migrator {
	return &Migrator{
		status: MigrationStatus{
			Status:  "idle",
			Message: "Ready to migrate legacy storage.",
		},
	}
}

func (m *Migrator) GetStatus() MigrationStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Migrator) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isRunning && m.cancel != nil {
		select {
		case <-m.cancel:
		default:
			close(m.cancel)
		}
		m.status.Status = "cancelled"
		m.status.Message = "Migration cancelled by user."
	}
}

func (m *Migrator) StartMigration(dataPath string) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("migration already in progress")
	}
	m.isRunning = true
	m.cancel = make(chan struct{})
	m.status = MigrationStatus{
		Status:  "running",
		Message: "Scanning blockchain directories...",
		Percent: 0,
	}
	m.mu.Unlock()

	go m.run(dataPath)
	return nil
}

func (m *Migrator) run(dataPath string) {
	defer func() {
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	log.Printf("[Migrator] Starting legacy block storage migration in: %s", dataPath)

	entries, err := os.ReadDir(dataPath)
	if err != nil {
		m.fail(fmt.Sprintf("Failed to read data directory: %v", err))
		return
	}

	var chunkDirs []string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == 5 {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				chunkDirs = append(chunkDirs, entry.Name())
			}
		}
	}
	sort.Strings(chunkDirs)

	if len(chunkDirs) == 0 {
		m.complete("No chunk directories found to migrate.", 0, 0, 0, time.Since(startTime))
		return
	}

	m.mu.Lock()
	m.status.TotalDirs = len(chunkDirs)
	m.status.ProcessedDirs = 0
	m.mu.Unlock()

	var totalConvertedBlocks int64
	var totalDeletedFiles int64
	var totalBytesWritten int64

	for dirIdx, dirName := range chunkDirs {
		select {
		case <-m.cancel:
			log.Printf("[Migrator] Migration aborted by cancel signal")
			return
		default:
		}

		dirPath := filepath.Join(dataPath, dirName)
		m.mu.Lock()
		m.status.CurrentDir = dirName
		m.status.ProcessedDirs = dirIdx
		if len(chunkDirs) > 0 {
			m.status.Percent = int((float64(dirIdx) / float64(len(chunkDirs))) * 100)
		}
		m.status.Message = fmt.Sprintf("Processing chunk folder %s (%d/%d)...", dirName, dirIdx+1, len(chunkDirs))
		m.mu.Unlock()

		converted, deleted, bytesWritten, err := m.processDirectory(dirPath)
		if err != nil {
			m.fail(fmt.Sprintf("Error migrating directory %s: %v", dirName, err))
			return
		}

		totalConvertedBlocks += converted
		totalDeletedFiles += deleted
		totalBytesWritten += bytesWritten

		m.mu.Lock()
		m.status.ConvertedBlocks = totalConvertedBlocks
		m.status.DeletedFiles = totalDeletedFiles
		m.status.BytesWritten = totalBytesWritten
		m.mu.Unlock()
	}

	m.complete(
		fmt.Sprintf("Successfully migrated %d blocks across %d directories (deleted %d legacy files).", totalConvertedBlocks, len(chunkDirs), totalDeletedFiles),
		totalConvertedBlocks,
		totalDeletedFiles,
		totalBytesWritten,
		time.Since(startTime),
	)
}

func (m *Migrator) processDirectory(dirPath string) (int64, int64, int64, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, 0, 0, err
	}

	// 1. Identify loose .dat block files (ignoring blocks.dat, hashes.dat, index.dat)
	var looseHeights []int64
	looseFilesMap := make(map[int64]string)
	stmtFilesMap := make(map[int64]string)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if name == "blocks.dat" || name == "blocks.idx" || name == "statements.dat" || name == "hashes.dat" || name == "index.dat" {
			continue
		}

		if strings.HasSuffix(name, ".dat") {
			base := strings.TrimSuffix(name, ".dat")
			if h, err := strconv.ParseInt(base, 10, 64); err == nil {
				looseHeights = append(looseHeights, h)
				looseFilesMap[h] = filepath.Join(dirPath, name)
			}
		} else if strings.HasSuffix(name, ".stmt") {
			base := strings.TrimSuffix(name, ".stmt")
			if h, err := strconv.ParseInt(base, 10, 64); err == nil {
				stmtFilesMap[h] = filepath.Join(dirPath, name)
			}
		}
	}

	if len(looseHeights) == 0 {
		return 0, 0, 0, nil // Directory is already migrated or empty
	}

	sort.Slice(looseHeights, func(i, j int) bool {
		return looseHeights[i] < looseHeights[j]
	})

	blocksDatPath := filepath.Join(dirPath, "blocks.dat")
	stmtDatPath := filepath.Join(dirPath, "statements.dat")
	blocksIdxPath := filepath.Join(dirPath, "blocks.idx")

	blocksFile, err := os.OpenFile(blocksDatPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, err
	}
	defer blocksFile.Close()

	stmtFile, err := os.OpenFile(stmtDatPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, err
	}
	defer stmtFile.Close()

	idxFile, err := os.OpenFile(blocksIdxPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, err
	}
	defer idxFile.Close()

	var convertedBlocks int64
	var deletedFiles int64
	var bytesWritten int64

	for _, height := range looseHeights {
		select {
		case <-m.cancel:
			return convertedBlocks, deletedFiles, bytesWritten, fmt.Errorf("migration cancelled")
		default:
		}

		datPath := looseFilesMap[height]
		blockPayload, err := os.ReadFile(datPath)
		if err != nil {
			continue
		}

		// Read statements payload if available
		var stmtPayload []byte
		if sPath, ok := stmtFilesMap[height]; ok {
			stmtPayload, _ = os.ReadFile(sPath)
		}

		// Write block payload
		bInfo, _ := blocksFile.Stat()
		blockOffset := uint32(bInfo.Size())
		if _, err := blocksFile.Seek(int64(blockOffset), io.SeekStart); err != nil {
			return convertedBlocks, deletedFiles, bytesWritten, err
		}
		if _, err := blocksFile.Write(blockPayload); err != nil {
			return convertedBlocks, deletedFiles, bytesWritten, err
		}
		blockSize := uint32(len(blockPayload))
		bytesWritten += int64(blockSize)

		// Write statements payload
		var stmtOffset, stmtSize uint32
		if len(stmtPayload) > 0 {
			sInfo, _ := stmtFile.Stat()
			stmtOffset = uint32(sInfo.Size())
			if _, err := stmtFile.Seek(int64(stmtOffset), io.SeekStart); err != nil {
				return convertedBlocks, deletedFiles, bytesWritten, err
			}
			if _, err := stmtFile.Write(stmtPayload); err != nil {
				return convertedBlocks, deletedFiles, bytesWritten, err
			}
			stmtSize = uint32(len(stmtPayload))
			bytesWritten += int64(stmtSize)
		}

		// Write 16-byte index entry into blocks.idx
		index := uint32(height % 65536)
		targetIdxOffset := int64(index * 16)

		idxInfo, _ := idxFile.Stat()
		if idxInfo.Size() < targetIdxOffset {
			padZeros := make([]byte, targetIdxOffset-idxInfo.Size())
			if _, err := idxFile.Seek(idxInfo.Size(), io.SeekStart); err == nil {
				_, _ = idxFile.Write(padZeros)
			}
		}

		entryBuf := make([]byte, 16)
		binary.LittleEndian.PutUint32(entryBuf[0:4], blockOffset)
		binary.LittleEndian.PutUint32(entryBuf[4:8], blockSize)
		binary.LittleEndian.PutUint32(entryBuf[8:12], stmtOffset)
		binary.LittleEndian.PutUint32(entryBuf[12:16], stmtSize)

		if _, err := idxFile.Seek(targetIdxOffset, io.SeekStart); err == nil {
			if _, err := idxFile.Write(entryBuf); err != nil {
				return convertedBlocks, deletedFiles, bytesWritten, err
			}
		}

		// Clean up loose files
		_ = os.Remove(datPath)
		deletedFiles++
		if sPath, ok := stmtFilesMap[height]; ok {
			_ = os.Remove(sPath)
			deletedFiles++
		}

		convertedBlocks++
	}

	return convertedBlocks, deletedFiles, bytesWritten, nil
}

func (m *Migrator) fail(errMsg string) {
	log.Printf("[Migrator] Error: %s", errMsg)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Status = "failed"
	m.status.Error = errMsg
	m.status.Message = errMsg
}

func (m *Migrator) complete(msg string, blocks, deleted, bytes int64, dur time.Duration) {
	log.Printf("[Migrator] Completed in %v: %s", dur.Round(time.Millisecond), msg)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Status = "completed"
	m.status.Percent = 100
	m.status.Message = fmt.Sprintf("%s (Completed in %v)", msg, dur.Round(time.Millisecond))
	m.status.ConvertedBlocks = blocks
	m.status.DeletedFiles = deleted
	m.status.BytesWritten = bytes
}
