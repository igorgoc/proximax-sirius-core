package snapshot

import (
	"errors"
	"time"
)

var (
	ErrInvalidSignature     = errors.New("cryptographic signature verification failed for snapshot manifest")
	ErrChecksumMismatch     = errors.New("sha256 checksum mismatch on downloaded snapshot archive")
	ErrMissingChecksum      = errors.New("checksums manifest does not contain entry for snapshot archive")
	ErrSnapshotBusy         = errors.New("another snapshot operation is currently in progress")
	ErrDecompressionBomb    = errors.New("decompression limit exceeded: archive uncompressed size exceeds maximum safety ceiling")
	ErrDownloadLimitExceeded = errors.New("download limit exceeded: stream volume exceeds maximum safety ceiling")
)

// SnapshotManifest defines the portable, backend-agnostic signed snapshot pointer.
// The downloadUrl field is completely generic — it works identically whether pointing
// to Cloudflare R2, Backblaze B2, AWS S3, or any generic HTTPS web server.
type SnapshotManifest struct {
	Version         string  `json:"version"`         // Manifest schema version e.g. "1.0"
	ChainHeight     int64   `json:"chainHeight"`     // Sirius block height at time of snapshot
	Network         string  `json:"network"`         // "mainnet" or "testnet"
	ArchiveName     string  `json:"archiveName"`     // e.g. "sirius-snapshot-13885400.tar.zst"
	DownloadUrl     string  `json:"downloadUrl"`     // Pluggable HTTPS URL (R2, B2, S3, custom host)
	Sha256          string  `json:"sha256"`          // Hex-encoded SHA-256 hash of the archive
	Format          string  `json:"format"`          // "tar.zst", "tar.gz", "tar.xz"
	UncompressedGB  float64 `json:"uncompressedGb"`  // Estimated uncompressed footprint in GB
	CompressedBytes int64   `json:"compressedBytes"` // Exact compressed archive size in bytes
	CreatedAt       string  `json:"createdAt"`       // RFC3339 timestamp
	ReleaseTag      string  `json:"releaseTag,omitempty"` // Associated GitHub release tag if hosted on GH
}

type OperationStage string

const (
	StageIdle              OperationStage = "idle"
	StageFetchingManifest  OperationStage = "fetching_manifest"
	StageVerifyingSig      OperationStage = "verifying_signature"
	StageDownloading       OperationStage = "downloading"
	StageVerifyingChecksum OperationStage = "verifying_checksum"
	StageExtracting        OperationStage = "extracting"
	StageArchiving         OperationStage = "archiving"
	StageCompleted         OperationStage = "completed"
	StageError             OperationStage = "error"
	StageCancelled         OperationStage = "cancelled"
)

type ProgressInfo struct {
	Percentage      float64 `json:"percentage"`
	ProcessedBytes  int64   `json:"processedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	SpeedMBs        float64 `json:"speedMbs"`
	ETASeconds      int64   `json:"etaSeconds"`
	CurrentItem     string  `json:"currentItem,omitempty"`
}

type SnapshotStatus struct {
	Stage        OperationStage `json:"stage"`
	Operation    string         `json:"operation,omitempty"` // "create", "restore_local", "restore_remote"
	Message      string         `json:"message"`
	Progress     ProgressInfo   `json:"progress"`
	Manifest     *SnapshotManifest `json:"manifest,omitempty"`
	TargetFile   string         `json:"targetFile,omitempty"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	StartedAt    time.Time      `json:"startedAt,omitempty"`
	CompletedAt  time.Time      `json:"completedAt,omitempty"`
}
