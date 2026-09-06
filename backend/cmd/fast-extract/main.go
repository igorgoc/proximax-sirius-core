package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

func main() {
	archivePath := "./snapshot.tar.zst"
	targetDir := "./chainconfig/data"

	if len(os.Args) > 1 {
		archivePath = os.Args[1]
	}
	if len(os.Args) > 2 {
		targetDir = os.Args[2]
	}

	fmt.Printf("=========================================================\n")
	fmt.Printf(" High-Speed klauspost/compress/zstd Snapshot Extractor\n")
	fmt.Printf(" Source: %s\n", archivePath)
	fmt.Printf(" Target: %s\n", targetDir)
	fmt.Printf(" Concurrency: ALL CPU Cores (Multi-threaded)\n")
	fmt.Printf("=========================================================\n")

	inFile, err := os.Open(archivePath)
	if err != nil {
		log.Fatalf("Failed to open archive: %v", err)
	}
	defer inFile.Close()

	// Initialize klauspost/compress/zstd with full multi-threaded decoding
	zstdReader, err := zstd.NewReader(inFile, zstd.WithDecoderConcurrency(0))
	if err != nil {
		log.Fatalf("Failed to initialize zstd decoder: %v", err)
	}
	defer zstdReader.Close()

	tarReader := tar.NewReader(zstdReader)

	startTime := time.Now()
	lastReport := time.Now()
	var totalFiles int64
	var totalBytes int64
	buf := make([]byte, 1024*1024) // 1MB buffer

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Tar read error: %v", err)
		}

		cleanedPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanedPath, "..") || filepath.IsAbs(cleanedPath) {
			continue
		}

		destPath := filepath.Join(targetDir, cleanedPath)

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(destPath, 0755)
		case tar.TypeReg, tar.TypeRegA:
			_ = os.MkdirAll(filepath.Dir(destPath), 0755)
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				log.Fatalf("Failed creating file %s: %v", destPath, err)
			}

			written, err := io.CopyBuffer(outFile, tarReader, buf)
			_ = outFile.Close()
			if err != nil {
				log.Fatalf("Failed writing file %s: %v", destPath, err)
			}

			totalBytes += written
			totalFiles++
		}

		if time.Since(lastReport) >= 3*time.Second {
			elapsed := time.Since(startTime).Seconds()
			speedMB := float64(totalBytes) / (1024 * 1024 * elapsed)
			fmt.Printf("[Progress] %d files | %.2f GB written | Speed: %.1f MB/s | Elapsed: %s\n",
				totalFiles, float64(totalBytes)/(1024*1024*1024), speedMB, time.Since(startTime).Round(time.Second))
			lastReport = time.Now()
		}
	}

	elapsed := time.Since(startTime)
	speedMB := float64(totalBytes) / (1024 * 1024 * elapsed.Seconds())
	fmt.Printf("\n=========================================================\n")
	fmt.Printf(" ✅ Extraction Complete in %s!\n", elapsed.Round(time.Second))
	fmt.Printf(" Total Files: %d\n", totalFiles)
	fmt.Printf(" Total Size:  %.2f GB\n", float64(totalBytes)/(1024*1024*1024))
	fmt.Printf(" Avg Speed:   %.1f MB/s\n", speedMB)
	fmt.Printf("=========================================================\n")
}
