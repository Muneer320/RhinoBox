package duplicates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
	"github.com/google/uuid"
)

// Scanner handles duplicate detection operations.
type Scanner struct {
	storage *storage.Manager
}

// NewScanner creates a new duplicate scanner.
func NewScanner(storage *storage.Manager) *Scanner {
	return &Scanner{storage: storage}
}

// Scan performs a duplicate scan across all files in the metadata index.
func (s *Scanner) Scan(opts ScanOptions) (*ScanResult, error) {
	startTime := time.Now()
	scanID := fmt.Sprintf("scan-%s", uuid.New().String()[:8])

	index := s.storage.GetIndex()
	allFiles := index.GetAll()

	// Group files by hash
	hashGroups := make(map[string][]storage.FileMetadata)
	for _, file := range allFiles {
		hashGroups[file.Hash] = append(hashGroups[file.Hash], file)
	}

	// Find duplicates (groups with more than 1 file)
	var groups []DuplicateGroup
	duplicateCount := 0
	var storageWasted int64

	for hash, files := range hashGroups {
		if len(files) > 1 {
			// Calculate wasted storage (all copies except one)
			fileSize := files[0].Size
			wastedSpace := fileSize * int64(len(files)-1)

			group := DuplicateGroup{
				Hash:        hash,
				Count:       len(files),
				Size:        fileSize,
				TotalWasted: wastedSpace,
				Files:       files,
			}
			groups = append(groups, group)
			duplicateCount += len(files) - 1
			storageWasted += wastedSpace
		}
	}

	result := &ScanResult{
		ScanID:          scanID,
		TotalFiles:      len(allFiles),
		DuplicatesFound: duplicateCount,
		StorageWasted:   storageWasted,
		Status:          "completed",
		StartedAt:       startTime,
		CompletedAt:     time.Now(),
	}

	if opts.IncludeMetadata {
		result.Groups = groups
	}

	return result, nil
}

// GetDuplicateGroups returns all groups of duplicate files.
func (s *Scanner) GetDuplicateGroups() ([]DuplicateGroup, error) {
	index := s.storage.GetIndex()
	allFiles := index.GetAll()

	// Group files by hash
	hashGroups := make(map[string][]storage.FileMetadata)
	for _, file := range allFiles {
		hashGroups[file.Hash] = append(hashGroups[file.Hash], file)
	}

	// Find duplicates
	var groups []DuplicateGroup
	for hash, files := range hashGroups {
		if len(files) > 1 {
			fileSize := files[0].Size
			wastedSpace := fileSize * int64(len(files)-1)

			group := DuplicateGroup{
				Hash:        hash,
				Count:       len(files),
				Size:        fileSize,
				TotalWasted: wastedSpace,
				Files:       files,
			}
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// Verify performs integrity checks on the storage system.
func (s *Scanner) Verify() (*VerifyResult, error) {
	result := &VerifyResult{
		Issues: []VerifyIssue{},
	}

	index := s.storage.GetIndex()
	allMetadata := index.GetAll()
	result.MetadataIndexCount = len(allMetadata)

	root := s.storage.Root()
	storageRoot := s.storage.GetStorageRoot()

	// Track files we've seen in the index
	indexedPaths := make(map[string]storage.FileMetadata)
	for _, meta := range allMetadata {
		indexedPaths[meta.StoredPath] = meta
	}

	// Walk the storage directory and find all physical files
	physicalFiles := make(map[string]string) // path -> actual hash
	err := filepath.Walk(storageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Skip temporary files
		if strings.Contains(path, ".tmp") {
			return nil
		}

		// Get relative path from root
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// Count physical file
		result.PhysicalFilesCount++
		physicalFiles[relPath] = ""

		// Check if file is in index
		meta, inIndex := indexedPaths[relPath]
		if !inIndex {
			result.OrphanedFiles++
			result.Issues = append(result.Issues, VerifyIssue{
				Type:    "orphaned_file",
				Path:    relPath,
				Message: "File exists on disk but not in metadata index",
			})
			return nil
		}

		// Verify hash if deep scan enabled
		actualHash, err := computeFileHash(path)
		if err != nil {
			result.Issues = append(result.Issues, VerifyIssue{
				Type:    "hash_error",
				Path:    relPath,
				Hash:    meta.Hash,
				Message: fmt.Sprintf("Failed to compute hash: %v", err),
			})
			return nil
		}
		physicalFiles[relPath] = actualHash

		if actualHash != meta.Hash {
			result.HashMismatches++
			result.Issues = append(result.Issues, VerifyIssue{
				Type:    "hash_mismatch",
				Path:    relPath,
				Hash:    meta.Hash,
				Message: fmt.Sprintf("Stored hash %s does not match actual hash %s", meta.Hash[:12], actualHash[:12]),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk storage: %w", err)
	}

	// Find missing files (in index but not on disk)
	for path := range indexedPaths {
		if _, exists := physicalFiles[path]; !exists {
			result.MissingFiles++
			meta := indexedPaths[path]
			result.Issues = append(result.Issues, VerifyIssue{
				Type:    "missing_file",
				Path:    path,
				Hash:    meta.Hash,
				Message: "File in metadata index but not found on disk",
			})
		}
	}

	return result, nil
}

// Merge removes duplicate files keeping only one copy.
func (s *Scanner) Merge(req MergeRequest) (*MergeResult, error) {
	index := s.storage.GetIndex()
	allFiles := index.GetAll()

	// Find all files with the target hash
	var matchingFiles []storage.FileMetadata
	for _, file := range allFiles {
		if file.Hash == req.Hash {
			matchingFiles = append(matchingFiles, file)
		}
	}

	if len(matchingFiles) == 0 {
		return nil, fmt.Errorf("no files found with hash %s", req.Hash)
	}

	if len(matchingFiles) == 1 {
		return &MergeResult{
			Hash:          req.Hash,
			KeptFile:      matchingFiles[0].StoredPath,
			RemovedFiles:  []string{},
			SpaceReclaimed: 0,
		}, nil
	}

	// Verify the file to keep exists
	var keepFile *storage.FileMetadata
	for i := range matchingFiles {
		if matchingFiles[i].StoredPath == req.Keep {
			keepFile = &matchingFiles[i]
			break
		}
	}
	if keepFile == nil {
		return nil, fmt.Errorf("file to keep %s not found in duplicate group", req.Keep)
	}

	result := &MergeResult{
		Hash:         req.Hash,
		KeptFile:     req.Keep,
		RemovedFiles: []string{},
	}

	if !req.RemoveOthers {
		// Just return what would be removed
		for _, file := range matchingFiles {
			if file.StoredPath != req.Keep {
				result.RemovedFiles = append(result.RemovedFiles, file.StoredPath)
				result.SpaceReclaimed += file.Size
			}
		}
		return result, nil
	}

	// Actually remove the duplicates
	root := s.storage.Root()
	for _, file := range matchingFiles {
		if file.StoredPath == req.Keep {
			continue
		}

		// Remove physical file
		fullPath := filepath.Join(root, filepath.FromSlash(file.StoredPath))
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove file %s: %w", file.StoredPath, err)
		}

		result.RemovedFiles = append(result.RemovedFiles, file.StoredPath)
		result.SpaceReclaimed += file.Size
	}

	// Note: We keep the metadata index entry for the kept file
	// The duplicates are removed from disk but the index still points to the kept file

	return result, nil
}

// computeFileHash calculates SHA256 hash of a file.
func computeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
