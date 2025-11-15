package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Duplicate-related errors
var (
	ErrDuplicateNotFound     = errors.New("duplicate group not found")
	ErrInvalidMergeRequest   = errors.New("invalid merge request")
	ErrScanInProgress        = errors.New("scan already in progress")
	ErrVerificationFailed    = errors.New("verification failed")
)

// DuplicateScanRequest contains parameters for scanning duplicates
type DuplicateScanRequest struct {
	DeepScan       bool `json:"deep_scan"`        // Re-compute hashes for all files
	IncludeMetadata bool `json:"include_metadata"` // Include full metadata in results
}

// DuplicateScanResult contains the results of a duplicate scan
type DuplicateScanResult struct {
	ScanID          string    `json:"scan_id"`
	TotalFiles      int       `json:"total_files"`
	DuplicatesFound int       `json:"duplicates_found"`
	StorageWasted   int64     `json:"storage_wasted"` // bytes
	Status          string    `json:"status"`          // "completed", "in_progress", "failed"
	StartedAt       time.Time `json:"started_at,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	Error           string    `json:"error,omitempty"`
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	Hash         string                 `json:"hash"`
	Count        int                    `json:"count"`
	Size         int64                  `json:"size"`         // size of one file
	TotalWasted  int64                  `json:"total_wasted"` // (count - 1) * size
	Files        []DuplicateFileInfo    `json:"files"`
}

// DuplicateFileInfo contains information about a duplicate file
type DuplicateFileInfo struct {
	StoredPath   string    `json:"stored_path"`
	OriginalName string    `json:"original_name"`
	UploadedAt   time.Time `json:"uploaded_at"`
	Category     string    `json:"category,omitempty"`
	MimeType     string    `json:"mime_type,omitempty"`
	Size         int64     `json:"size,omitempty"`
}

// VerificationResult contains the results of system verification
type VerificationResult struct {
	MetadataIndexCount int64              `json:"metadata_index_count"`
	PhysicalFilesCount int64              `json:"physical_files_count"`
	HashMismatches     int                `json:"hash_mismatches"`
	OrphanedFiles      int                `json:"orphaned_files"`
	MissingFiles       int                `json:"missing_files"`
	Issues             []VerificationIssue `json:"issues"`
	CompletedAt        time.Time          `json:"completed_at"`
}

// VerificationIssue represents a problem found during verification
type VerificationIssue struct {
	Type    string `json:"type"`    // "orphaned_file", "missing_file", "hash_mismatch"
	Path    string `json:"path"`
	Message string `json:"message"`
	Hash    string `json:"hash,omitempty"`
}

// MergeRequest contains parameters for merging duplicates
type MergeRequest struct {
	Hash         string `json:"hash"`
	Keep         string `json:"keep"`         // stored_path to keep
	RemoveOthers bool   `json:"remove_others"` // whether to remove other duplicates
}

// MergeResult contains the results of a merge operation
type MergeResult struct {
	Hash         string   `json:"hash"`
	Kept         string   `json:"kept"`
	Removed      []string `json:"removed"`
	SpaceReclaimed int64  `json:"space_reclaimed"`
	MergedAt     time.Time `json:"merged_at"`
}

// scanState tracks the state of an ongoing scan
type scanState struct {
	mu              sync.Mutex
	inProgress      bool
	lastResult      *DuplicateScanResult
	duplicateGroups map[string]*DuplicateGroup
}

// ScanForDuplicates scans the storage for duplicate files
func (m *Manager) ScanForDuplicates(req DuplicateScanRequest) (*DuplicateScanResult, error) {
	// Check if scan is already in progress
	m.scanState.mu.Lock()
	if m.scanState.inProgress {
		m.scanState.mu.Unlock()
		return nil, ErrScanInProgress
	}
	m.scanState.inProgress = true
	m.scanState.mu.Unlock()

	defer func() {
		m.scanState.mu.Lock()
		m.scanState.inProgress = false
		m.scanState.mu.Unlock()
	}()

	scanID := fmt.Sprintf("scan-%d", time.Now().UnixNano())
	startedAt := time.Now()

	result := &DuplicateScanResult{
		ScanID:     scanID,
		Status:     "in_progress",
		StartedAt:  startedAt,
	}

	// Group files by hash
	hashGroups := make(map[string][]FileMetadata)
	
	m.mu.Lock()
	// Collect all metadata entries
	for _, meta := range m.index.data {
		hashGroups[meta.Hash] = append(hashGroups[meta.Hash], meta)
	}
	totalFiles := len(m.index.data)
	m.mu.Unlock()

	// If deep scan is requested, verify hashes on disk
	if req.DeepScan {
		if err := m.verifyHashesOnDisk(hashGroups); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.CompletedAt = time.Now()
			return result, err
		}
	}

	// Find duplicates (groups with more than one file)
	duplicateGroups := make(map[string]*DuplicateGroup)
	totalWasted := int64(0)
	duplicateCount := 0

	for hash, files := range hashGroups {
		if len(files) > 1 {
			// This is a duplicate group
			group := &DuplicateGroup{
				Hash:        hash,
				Count:       len(files),
				Size:        files[0].Size,
				TotalWasted: int64(len(files)-1) * files[0].Size,
			}

			// Add file information
			group.Files = make([]DuplicateFileInfo, len(files))
			for i, file := range files {
				group.Files[i] = DuplicateFileInfo{
					StoredPath:   file.StoredPath,
					OriginalName: file.OriginalName,
					UploadedAt:   file.UploadedAt,
					Category:     file.Category,
					MimeType:     file.MimeType,
					Size:         file.Size,
				}
			}

			duplicateGroups[hash] = group
			totalWasted += group.TotalWasted
			duplicateCount++
		}
	}

	// Store results
	m.scanState.mu.Lock()
	if m.scanState.duplicateGroups == nil {
		m.scanState.duplicateGroups = make(map[string]*DuplicateGroup)
	}
	for hash, group := range duplicateGroups {
		m.scanState.duplicateGroups[hash] = group
	}
	m.scanState.lastResult = result
	m.scanState.mu.Unlock()

	result.TotalFiles = totalFiles
	result.DuplicatesFound = duplicateCount
	result.StorageWasted = totalWasted
	result.Status = "completed"
	result.CompletedAt = time.Now()

	return result, nil
}

// verifyHashesOnDisk re-computes hashes for files on disk and compares with metadata
func (m *Manager) verifyHashesOnDisk(hashGroups map[string][]FileMetadata) error {
	for hash, files := range hashGroups {
		for _, file := range files {
			fullPath := filepath.Join(m.root, file.StoredPath)
			
			// Open file and compute hash
			f, err := os.Open(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					// File missing - will be caught by verification
					continue
				}
				return fmt.Errorf("failed to open file %s: %w", fullPath, err)
			}

			hasher := sha256.New()
			if _, err := io.Copy(hasher, f); err != nil {
				f.Close()
				return fmt.Errorf("failed to read file %s: %w", fullPath, err)
			}
			f.Close()

			computedHash := hex.EncodeToString(hasher.Sum(nil))
			if computedHash != hash {
				return fmt.Errorf("hash mismatch for file %s: expected %s, got %s", fullPath, hash, computedHash)
			}
		}
	}
	return nil
}

// GetDuplicateReport returns the current duplicate report
func (m *Manager) GetDuplicateReport() ([]DuplicateGroup, error) {
	m.scanState.mu.Lock()
	defer m.scanState.mu.Unlock()

	if m.scanState.duplicateGroups == nil {
		// No scan has been run yet, return empty
		return []DuplicateGroup{}, nil
	}

	groups := make([]DuplicateGroup, 0, len(m.scanState.duplicateGroups))
	for _, group := range m.scanState.duplicateGroups {
		groups = append(groups, *group)
	}

	return groups, nil
}

// VerifyDeduplicationSystem verifies the integrity of the deduplication system
func (m *Manager) VerifyDeduplicationSystem() (*VerificationResult, error) {
	result := &VerificationResult{
		Issues: make([]VerificationIssue, 0),
	}

	// Count files in metadata index
	m.mu.Lock()
	metadataCount := int64(len(m.index.data))
	metadataMap := make(map[string]FileMetadata)
	for hash, meta := range m.index.data {
		metadataMap[hash] = meta
	}
	m.mu.Unlock()

	result.MetadataIndexCount = metadataCount

	// Walk storage directory and compare with metadata index
	physicalFiles := make(map[string]string) // path -> hash
	orphanedFiles := make([]string, 0)
	hashMismatches := 0

	err := filepath.Walk(m.storageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and temp files
		if info.IsDir() || filepath.Base(path) == ".tmp" {
			return nil
		}

		// Skip temp directory
		if filepath.Dir(path) == filepath.Join(m.storageRoot, ".tmp") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(m.root, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// Compute hash of file
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		hasher := sha256.New()
		if _, err := io.Copy(hasher, f); err != nil {
			return err
		}

		computedHash := hex.EncodeToString(hasher.Sum(nil))
		physicalFiles[relPath] = computedHash
		result.PhysicalFilesCount++

		// Check if file is in metadata index
		foundInIndex := false
		for hash, meta := range metadataMap {
			if meta.StoredPath == relPath {
				foundInIndex = true
				// Verify hash matches
				if hash != computedHash {
					hashMismatches++
					result.Issues = append(result.Issues, VerificationIssue{
						Type:    "hash_mismatch",
						Path:    relPath,
						Message: fmt.Sprintf("Hash mismatch: stored=%s, computed=%s", hash, computedHash),
						Hash:    computedHash,
					})
				}
				break
			}
		}

		if !foundInIndex {
			orphanedFiles = append(orphanedFiles, relPath)
			result.Issues = append(result.Issues, VerificationIssue{
				Type:    "orphaned_file",
				Path:    relPath,
				Message: "File exists on disk but not in metadata index",
				Hash:    computedHash,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk storage: %w", err)
	}

	result.OrphanedFiles = len(orphanedFiles)

	// Check for missing files (in index but not on disk)
	missingFiles := 0
	for hash, meta := range metadataMap {
		// Check if the file exists in physical files map
		fileExists := false
		for path, fileHash := range physicalFiles {
			if path == meta.StoredPath && fileHash == hash {
				fileExists = true
				break
			}
		}
		if !fileExists {
			missingFiles++
			result.Issues = append(result.Issues, VerificationIssue{
				Type:    "missing_file",
				Path:    meta.StoredPath,
				Message: "File in metadata index but not found on disk",
				Hash:    hash,
			})
		}
	}

	result.MissingFiles = missingFiles
	result.HashMismatches = hashMismatches
	result.CompletedAt = time.Now()

	return result, nil
}

// MergeDuplicates merges duplicate files, keeping one and optionally removing others
func (m *Manager) MergeDuplicates(req MergeRequest) (*MergeResult, error) {
	if req.Hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrInvalidMergeRequest)
	}

	if req.Keep == "" {
		return nil, fmt.Errorf("%w: keep path is required", ErrInvalidMergeRequest)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find all files with this hash
	var keepMeta *FileMetadata
	var removeMetas []FileMetadata

	for _, meta := range m.index.data {
		if meta.Hash == req.Hash {
			if meta.StoredPath == req.Keep {
				keepMeta = &meta
			} else {
				removeMetas = append(removeMetas, meta)
			}
		}
	}

	if keepMeta == nil {
		return nil, fmt.Errorf("%w: keep path not found for hash", ErrInvalidMergeRequest)
	}

	if len(removeMetas) == 0 {
		return nil, fmt.Errorf("%w: no duplicates found to merge", ErrDuplicateNotFound)
	}

	// If remove_others is true, delete the duplicate files
	removed := make([]string, 0)
	spaceReclaimed := int64(0)

	if req.RemoveOthers {
		for _, meta := range removeMetas {
			// Delete physical file
			fullPath := filepath.Join(m.root, meta.StoredPath)
			if err := os.Remove(fullPath); err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("failed to remove file %s: %w", meta.StoredPath, err)
				}
			} else {
				removed = append(removed, meta.StoredPath)
				spaceReclaimed += meta.Size
			}

			// Delete metadata entry
			if err := m.index.Delete(meta.Hash); err != nil {
				// Log but continue - file is already deleted
				fmt.Fprintf(os.Stderr, "warning: failed to delete metadata for %s: %v\n", meta.StoredPath, err)
			}
		}
	}

	return &MergeResult{
		Hash:           req.Hash,
		Kept:           req.Keep,
		Removed:        removed,
		SpaceReclaimed: spaceReclaimed,
		MergedAt:       time.Now(),
	}, nil
}

// GetDuplicateStatistics returns statistics about duplicates
func (m *Manager) GetDuplicateStatistics() (map[string]interface{}, error) {
	groups, err := m.GetDuplicateReport()
	if err != nil {
		return nil, err
	}

	totalDuplicates := 0
	totalWasted := int64(0)
	totalGroups := len(groups)

	for _, group := range groups {
		totalDuplicates += group.Count
		totalWasted += group.TotalWasted
	}

	return map[string]interface{}{
		"duplicate_groups":  totalGroups,
		"total_duplicates":   totalDuplicates,
		"storage_wasted":     totalWasted,
		"deduplication_savings": totalWasted,
		"last_scan":         m.scanState.lastResult,
	}, nil
}

