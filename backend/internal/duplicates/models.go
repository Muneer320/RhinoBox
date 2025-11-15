package duplicates

import (
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// ScanResult represents the outcome of a duplicate scan operation.
type ScanResult struct {
	ScanID         string           `json:"scan_id"`
	TotalFiles     int              `json:"total_files"`
	DuplicatesFound int             `json:"duplicates_found"`
	StorageWasted  int64            `json:"storage_wasted"`
	Status         string           `json:"status"`
	StartedAt      time.Time        `json:"started_at"`
	CompletedAt    time.Time        `json:"completed_at"`
	Groups         []DuplicateGroup `json:"groups,omitempty"`
}

// DuplicateGroup represents a set of files with the same hash.
type DuplicateGroup struct {
	Hash         string                 `json:"hash"`
	Count        int                    `json:"count"`
	Size         int64                  `json:"size"`
	TotalWasted  int64                  `json:"total_wasted"`
	Files        []storage.FileMetadata `json:"files"`
}

// VerifyResult represents the outcome of a verification scan.
type VerifyResult struct {
	MetadataIndexCount int              `json:"metadata_index_count"`
	PhysicalFilesCount int              `json:"physical_files_count"`
	HashMismatches     int              `json:"hash_mismatches"`
	OrphanedFiles      int              `json:"orphaned_files"`
	MissingFiles       int              `json:"missing_files"`
	Issues             []VerifyIssue    `json:"issues"`
}

// VerifyIssue represents a specific verification problem.
type VerifyIssue struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Hash    string `json:"hash,omitempty"`
	Message string `json:"message"`
}

// MergeRequest represents a request to merge duplicate files.
type MergeRequest struct {
	Hash         string `json:"hash"`
	Keep         string `json:"keep"`
	RemoveOthers bool   `json:"remove_others"`
}

// MergeResult represents the outcome of a merge operation.
type MergeResult struct {
	Hash          string   `json:"hash"`
	KeptFile      string   `json:"kept_file"`
	RemovedFiles  []string `json:"removed_files"`
	SpaceReclaimed int64   `json:"space_reclaimed"`
}

// ScanOptions configures the duplicate scan behavior.
type ScanOptions struct {
	DeepScan        bool `json:"deep_scan"`
	IncludeMetadata bool `json:"include_metadata"`
}
