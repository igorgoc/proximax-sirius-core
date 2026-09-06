package main

import (
	"archive/tar"
	"bufio"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type BlockChunkIndexEntry struct {
	BlockOffset uint32
	BlockSize   uint32
	StmtOffset  uint32
	StmtSize    uint32
}

type DirectoryChunkWriter struct {
	targetDir       string
	dirName         string
	blocksFile      *os.File
	blocksBuf       *bufio.Writer
	blocksOffset    uint32
	stmtFile        *os.File
	stmtBuf         *bufio.Writer
	stmtOffset      uint32
	entries         [65536]BlockChunkIndexEntry
	maxIndexWritten int
	active          bool
}

func (dw *DirectoryChunkWriter) Open(targetDir, dirName string) error {
	dw.targetDir = targetDir
	dw.dirName = dirName
	dirPath := filepath.Join(targetDir, dirName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	bf, err := os.OpenFile(filepath.Join(dirPath, "blocks.dat"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	dw.blocksFile = bf
	dw.blocksBuf = bufio.NewWriterSize(bf, 2*1024*1024)
	dw.blocksOffset = 0

	sf, err := os.OpenFile(filepath.Join(dirPath, "statements.dat"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		bf.Close()
		return err
	}
	dw.stmtFile = sf
	dw.stmtBuf = bufio.NewWriterSize(sf, 1*1024*1024)
	dw.stmtOffset = 0

	dw.entries = [65536]BlockChunkIndexEntry{}
	dw.maxIndexWritten = -1
	dw.active = true
	return nil
}

func (dw *DirectoryChunkWriter) WriteBlock(height int64, payload []byte) error {
	index := int(height % 65536)
	dw.entries[index].BlockOffset = dw.blocksOffset
	dw.entries[index].BlockSize = uint32(len(payload))

	if _, err := dw.blocksBuf.Write(payload); err != nil {
		return err
	}
	dw.blocksOffset += uint32(len(payload))
	if index > dw.maxIndexWritten {
		dw.maxIndexWritten = index
	}
	return nil
}

func (dw *DirectoryChunkWriter) WriteStatement(height int64, payload []byte) error {
	index := int(height % 65536)
	dw.entries[index].StmtOffset = dw.stmtOffset
	dw.entries[index].StmtSize = uint32(len(payload))

	if _, err := dw.stmtBuf.Write(payload); err != nil {
		return err
	}
	dw.stmtOffset += uint32(len(payload))
	if index > dw.maxIndexWritten {
		dw.maxIndexWritten = index
	}
	return nil
}

func (dw *DirectoryChunkWriter) Close() error {
	if !dw.active {
		return nil
	}
	dw.active = false

	if err := dw.blocksBuf.Flush(); err != nil {
		return err
	}
	if err := dw.blocksFile.Close(); err != nil {
		return err
	}

	if err := dw.stmtBuf.Flush(); err != nil {
		return err
	}
	if err := dw.stmtFile.Close(); err != nil {
		return err
	}

	// Write index file
	dirPath := filepath.Join(dw.targetDir, dw.dirName)
	idxPath := filepath.Join(dirPath, "blocks.idx")
	idxFile, err := os.OpenFile(idxPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	idxBuf := bufio.NewWriterSize(idxFile, 1*1024*1024)

	numEntriesToWrite := dw.maxIndexWritten + 1
	if numEntriesToWrite < 0 {
		numEntriesToWrite = 0
	}

	entryBuf := make([]byte, 16)
	for i := 0; i < numEntriesToWrite; i++ {
		e := dw.entries[i]
		binary.LittleEndian.PutUint32(entryBuf[0:4], e.BlockOffset)
		binary.LittleEndian.PutUint32(entryBuf[4:8], e.BlockSize)
		binary.LittleEndian.PutUint32(entryBuf[8:12], e.StmtOffset)
		binary.LittleEndian.PutUint32(entryBuf[12:16], e.StmtSize)
		if _, err := idxBuf.Write(entryBuf); err != nil {
			idxFile.Close()
			return err
		}
	}

	if err := idxBuf.Flush(); err != nil {
		idxFile.Close()
		return err
	}
	return idxFile.Close()
}

func main() {
	archivePath := flag.String("archive", "", "Path to .tar.zst archive")
	targetDir := flag.String("target", "/Volumes/SSD/Sirius_data", "Destination blockchain data directory")
	flag.Parse()

	if *archivePath == "" {
		log.Fatalf("Error: --archive path is required")
	}

	startTime := time.Now()
	log.Printf("==================================================================")
	log.Printf("🚀 Starting Direct On-The-Fly Streaming Chunk Extraction")
	log.Printf("📦 Source Archive : %s", *archivePath)
	log.Printf("💾 Destination SSD: %s", *targetDir)
	log.Printf("==================================================================")

	if err := os.MkdirAll(*targetDir, 0755); err != nil {
		log.Fatalf("Failed to create target directory: %v", err)
	}

	// Launch zstd decompressor process streaming directly into stdout
	zstdCmd := exec.Command("zstd", "-dc", *archivePath)
	stdout, err := zstdCmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to open zstd stdout pipe: %v", err)
	}

	if err := zstdCmd.Start(); err != nil {
		log.Fatalf("Failed to start zstd process: %v", err)
	}

	tarReader := tar.NewReader(bufio.NewReaderSize(stdout, 4*1024*1024))

	var currentWriter DirectoryChunkWriter
	currentDirName := ""

	var processedBlocks int64
	var convertedFiles int64
	var directFiles int64
	var totalBytes int64
	lastReportTime := time.Now()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Tar stream error: %v", err)
		}

		cleanPath := filepath.Clean(header.Name)
		if cleanPath == "." || cleanPath == "/" || strings.HasPrefix(cleanPath, ".DS_Store") {
			continue
		}

		// Handle directory creation
		if header.Typeflag == tar.TypeDir {
			targetSubDir := filepath.Join(*targetDir, cleanPath)
			_ = os.MkdirAll(targetSubDir, 0755)
			continue
		}

		parts := strings.Split(cleanPath, string(filepath.Separator))

		// Check if this is a loose block/statement file inside a 5-digit numeric chunk directory (e.g. 00000/00001.dat)
		_, isNumericDir := strconv.Atoi(parts[0])
		if len(parts) == 2 && len(parts[0]) == 5 && isNumericDir == nil && (strings.HasSuffix(parts[1], ".dat") || strings.HasSuffix(parts[1], ".stmt")) && parts[1] != "blocks.dat" && parts[1] != "hashes.dat" && parts[1] != "index.dat" && parts[1] != "statements.dat" {
			dirName := parts[0]
			fileName := parts[1]

			if dirName != currentDirName {
				if currentWriter.active {
					if err := currentWriter.Close(); err != nil {
						log.Fatalf("Failed to close chunk writer for %s: %v", currentDirName, err)
					}
				}
				currentDirName = dirName
				if err := currentWriter.Open(*targetDir, dirName); err != nil {
					log.Fatalf("Failed to open chunk writer for %s: %v", dirName, err)
				}
			}

			payload := make([]byte, header.Size)
			if _, err := io.ReadFull(tarReader, payload); err != nil {
				log.Fatalf("Failed reading tar payload for %s: %v", cleanPath, err)
			}
			totalBytes += int64(len(payload))
			convertedFiles++

			if strings.HasSuffix(fileName, ".dat") {
				hStr := strings.TrimSuffix(fileName, ".dat")
				if h, err := strconv.ParseInt(hStr, 10, 64); err == nil {
					if err := currentWriter.WriteBlock(h, payload); err != nil {
						log.Fatalf("Failed writing block %d: %v", h, err)
					}
					processedBlocks++
				}
			} else if strings.HasSuffix(fileName, ".stmt") {
				hStr := strings.TrimSuffix(fileName, ".stmt")
				if h, err := strconv.ParseInt(hStr, 10, 64); err == nil {
					if err := currentWriter.WriteStatement(h, payload); err != nil {
						log.Fatalf("Failed writing statement %d: %v", h, err)
					}
				}
			}
		} else {
			// Direct file (e.g., index.dat, hashes.dat, statedb/*, state/*)
			destPath := filepath.Join(*targetDir, cleanPath)
			_ = os.MkdirAll(filepath.Dir(destPath), 0755)

			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				log.Fatalf("Failed creating file %s: %v", destPath, err)
			}
			written, err := io.Copy(f, tarReader)
			f.Close()
			if err != nil {
				log.Fatalf("Failed copying file %s: %v", destPath, err)
			}
			totalBytes += written
			directFiles++
		}

		if time.Since(lastReportTime) >= 3*time.Second {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(processedBlocks) / elapsed
			log.Printf("[Stream-Extract] Blocks: %d | Files converted: %d | Data: %.1f GB | Speed: %.0f blk/s (Dir: %s)",
				processedBlocks, convertedFiles, float64(totalBytes)/(1024*1024*1024), speed, currentDirName)
			lastReportTime = time.Now()
		}
	}

	if currentWriter.active {
		if err := currentWriter.Close(); err != nil {
			log.Fatalf("Failed closing final chunk writer: %v", err)
		}
	}

	_ = zstdCmd.Wait()

	dur := time.Since(startTime)
	log.Printf("==================================================================")
	log.Printf("✅ Streaming Chunk Extraction Complete in %v!", dur.Round(time.Millisecond))
	log.Printf("📊 Total Blocks Processed: %d", processedBlocks)
	log.Printf("🧹 Loose Files Converted : %d (0 loose files on disk!)", convertedFiles)
	log.Printf("📁 Direct State Files   : %d", directFiles)
	log.Printf("💾 Total Extracted Size  : %.2f GB", float64(totalBytes)/(1024*1024*1024))
	log.Printf("==================================================================")
}
