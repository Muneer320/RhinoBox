package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MoveRequest captures parameters for moving/recategorizing a file.
type MoveRequest struct {
	Hash        string `json:"hash"`
	NewCategory string `json:"new_category"`
	Reason      string `json:"reason,omitempty"`
}

// MoveResult surfaces the outcome of a move operation.
type MoveResult struct {
	Hash        string      `json:"hash"`
	OldCategory string      `json:"old_category"`
	NewCategory string      `json:"new_category"`
	OldPath     string      `json:"old_path"`
	NewPath     string      `json:"new_path"`
	Moved       bool        `json:"moved"`
	Message     string      `json:"message,omitempty"`
	Metadata    FileMetadata `json:"metadata"`
}

// BatchMoveRequest captures parameters for batch move operations.
type BatchMoveRequest struct {
	Files []MoveRequest `json:"files"`
}

// BatchMoveResult contains results for batch move operations.
type BatchMoveResult struct {
	Results       []MoveResult `json:"results"`
	Total         int          `json:"total"`
	SuccessCount  int          `json:"success_count"`
	FailureCount  int          `json:"failure_count"`
}

// MoveLog captures audit trail for move operations.
type MoveLog struct {
	Hash        string    `json:"hash"`
	OldCategory string    `json:"old_category"`
	NewCategory string    `json:"new_category"`
	OldPath     string    `json:"old_path"`
	NewPath     string    `json:"new_path"`
	Reason      string    `json:"reason,omitempty"`
	MovedAt     time.Time `json:"moved_at"`
	DurationMs  int64     `json:"duration_ms,omitempty"`
}

// MoveMetrics tracks performance metrics for move operations.
type MoveMetrics struct {
	TotalMoves      int64   `json:"total_moves"`
	SuccessfulMoves int64   `json:"successful_moves"`
	FailedMoves     int64   `json:"failed_moves"`
	AverageDuration float64 `json:"average_duration_ms"`
	TotalDuration   int64   `json:"total_duration_ms"`
}

var (
	// ErrInvalidCategory is returned when the category is invalid.
	ErrInvalidCategory = errors.New("invalid category")
	// ErrCategoryConflict is returned when a file with the same name exists in the target category.
	ErrCategoryConflict = errors.New("category conflict: file with same name exists")
	// ErrMoveFailed is returned when the move operation fails.
	ErrMoveFailed = errors.New("move operation failed")
)

// Category validation constants
const (
	maxCategoryDepth    = 10
	maxCategorySegLength = 100
)

// Invalid category patterns
var (
	// Path traversal attempts
	categoryPathTraversalPattern = regexp.MustCompile(`\.\.`)
	// Control characters (0x00-0x1F)
	categoryControlCharsPattern = regexp.MustCompile(`[\x00-\x1F]`)
	// Invalid characters in category paths
	invalidCategoryChars = regexp.MustCompile(`[<>:"|?*\x00]`)
)

// ValidateCategory checks if a category path is safe and valid.
func ValidateCategory(category string) error {
	if category == "" {
		return fmt.Errorf("%w: category cannot be empty", ErrInvalidCategory)
	}

	// Check for path traversal
	if categoryPathTraversalPattern.MatchString(category) {
		return fmt.Errorf("%w: path traversal detected", ErrInvalidCategory)
	}

	// Check for control characters
	if categoryControlCharsPattern.MatchString(category) {
		return fmt.Errorf("%w: control characters not allowed", ErrInvalidCategory)
	}

	// Check for invalid characters
	if invalidCategoryChars.MatchString(category) {
		return fmt.Errorf("%w: contains invalid characters", ErrInvalidCategory)
	}

	// Split and validate each segment
	segments := strings.Split(category, "/")
	if len(segments) > maxCategoryDepth {
		return fmt.Errorf("%w: category depth exceeds %d levels", ErrInvalidCategory, maxCategoryDepth)
	}

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return fmt.Errorf("%w: empty segment in category path", ErrInvalidCategory)
		}
		if len(seg) > maxCategorySegLength {
			return fmt.Errorf("%w: segment length exceeds %d characters", ErrInvalidCategory, maxCategorySegLength)
		}
		// Check for leading/trailing dots or spaces
		if strings.HasPrefix(seg, ".") || strings.HasSuffix(seg, ".") {
			return fmt.Errorf("%w: leading/trailing dots not allowed in segments", ErrInvalidCategory)
		}
	}

	return nil
}

// ensureCategoryDirectory creates the category directory structure if it doesn't exist.
func (m *Manager) ensureCategoryDirectory(category string) (string, error) {
	if err := ValidateCategory(category); err != nil {
		return "", err
	}

	// Split category into components
	components := strings.Split(category, "/")
	
	// Build full directory path
	fullDir := filepath.Join(append([]string{m.storageRoot}, components...)...)
	
	// Create directory structure
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create category directory: %w", err)
	}

	return fullDir, nil
}

// checkFilenameConflict checks if a file with the same name exists in the target category.
// REQUIRES: Caller must hold m.mu lock before calling this function.
func (m *Manager) checkFilenameConflict(hash, targetCategory string, filename string) error {
	for h, meta := range m.index.data {
		// Skip the file being moved
		if h == hash {
			continue
		}
		// Check for same filename in target category
		if meta.Category == targetCategory {
			// Extract filename from stored path
			metaFilename := filepath.Base(meta.StoredPath)
			if metaFilename == filename {
				return fmt.Errorf("%w: file %s already exists in category %s", ErrCategoryConflict, filename, targetCategory)
			}
		}
	}

	return nil
}

// MoveFile moves a file to a new category, maintaining metadata integrity.
// This is an atomic operation with rollback on failure.
func (m *Manager) MoveFile(req MoveRequest) (*MoveResult, error) {
	// Validate request
	if req.Hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrFileNotFound)
	}
	if req.NewCategory == "" {
		return nil, fmt.Errorf("%w: new_category is required", ErrInvalidCategory)
	}

	// Validate category
	if err := ValidateCategory(req.NewCategory); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing file by hash
	existing := m.index.FindByHash(req.Hash)
	if existing == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, req.Hash)
	}

	// Check if already in target category
	if existing.Category == req.NewCategory {
		return &MoveResult{
			Hash:        existing.Hash,
			OldCategory: existing.Category,
			NewCategory: req.NewCategory,
			OldPath:     existing.StoredPath,
			NewPath:     existing.StoredPath,
			Moved:       false,
			Message:     "file already in target category",
			Metadata:    *existing,
		}, nil
	}

	// Get absolute path to current file
	oldPath := filepath.Join(m.root, existing.StoredPath)

	// Check if the file actually exists on disk
	if _, err := os.Stat(oldPath); err != nil {
		return nil, fmt.Errorf("stored file not found: %w", err)
	}

	// Ensure target category directory exists
	targetDir, err := m.ensureCategoryDirectory(req.NewCategory)
	if err != nil {
		return nil, err
	}

	// Extract filename from stored path
	filename := filepath.Base(oldPath)

	// Check for filename conflicts in target category
	if err := m.checkFilenameConflict(req.Hash, req.NewCategory, filename); err != nil {
		return nil, err
	}

	// Build new path
	newPath := filepath.Join(targetDir, filename)

	// If source and destination are the same, no move needed
	if oldPath == newPath {
		// Just update category in metadata
		newMetadata := *existing
		newMetadata.Category = req.NewCategory
		m.index.data[req.Hash] = newMetadata
		if err := m.index.persistLocked(); err != nil {
			return nil, fmt.Errorf("failed to persist metadata: %w", err)
		}

		return &MoveResult{
			Hash:        existing.Hash,
			OldCategory: existing.Category,
			NewCategory: req.NewCategory,
			OldPath:     existing.StoredPath,
			NewPath:     existing.StoredPath,
			Moved:       false,
			Message:     "category updated (file already in correct location)",
			Metadata:    newMetadata,
		}, nil
	}

	// Check if target file already exists (shouldn't happen due to hash-based naming, but check anyway)
	if _, err := os.Stat(newPath); err == nil {
		// File exists - this shouldn't happen with hash-based naming, but handle it
		return nil, fmt.Errorf("%w: target file already exists", ErrCategoryConflict)
	}

	// Perform the actual file move
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Calculate relative path for new location
	rel, err := filepath.Rel(m.root, newPath)
	if err != nil {
		// Rollback: try to move file back
		_ = os.Rename(newPath, oldPath)
		return nil, fmt.Errorf("failed to compute relative path: %w", err)
	}

	// Update metadata
	newMetadata := *existing
	newMetadata.Category = req.NewCategory
	newMetadata.StoredPath = filepath.ToSlash(rel)

	// Track move operation start time for metrics
	moveStart := time.Now()

	// Update metadata in index
	m.index.data[req.Hash] = newMetadata

	// Persist metadata changes
	if err := m.index.persistLocked(); err != nil {
		// Rollback: try to move file back and restore metadata
		_ = os.Rename(newPath, oldPath)
		m.index.data[req.Hash] = *existing
		_ = m.index.persistLocked()
		return nil, fmt.Errorf("failed to persist metadata: %w", err)
	}

	// Log the move operation with metrics
	duration := time.Since(moveStart)
	logEntry := MoveLog{
		Hash:        req.Hash,
		OldCategory: existing.Category,
		NewCategory: req.NewCategory,
		OldPath:     existing.StoredPath,
		NewPath:     newMetadata.StoredPath,
		Reason:      req.Reason,
		MovedAt:     time.Now().UTC(),
		DurationMs:  duration.Milliseconds(),
	}
	_ = m.logMove(logEntry) // Best effort logging

	return &MoveResult{
		Hash:        req.Hash,
		OldCategory: existing.Category,
		NewCategory: req.NewCategory,
		OldPath:     existing.StoredPath,
		NewPath:     newMetadata.StoredPath,
		Moved:       true,
		Message:     fmt.Sprintf("moved from %s to %s", existing.Category, req.NewCategory),
		Metadata:    newMetadata,
	}, nil
}

// BatchMoveFile moves multiple files to new categories.
func (m *Manager) BatchMoveFile(req BatchMoveRequest) (*BatchMoveResult, error) {
	if len(req.Files) == 0 {
		return nil, fmt.Errorf("no files provided for batch move")
	}

	if len(req.Files) > 100 {
		return nil, fmt.Errorf("batch move limited to 100 files")
	}

	results := make([]MoveResult, 0, len(req.Files))
	successCount := 0
	failureCount := 0

	// Process each move request
	for _, moveReq := range req.Files {
		result, err := m.MoveFile(moveReq)
		if err != nil {
			failureCount++
			// Create error result
			results = append(results, MoveResult{
				Hash:        moveReq.Hash,
				OldCategory: "",
				NewCategory: moveReq.NewCategory,
				Moved:       false,
				Message:     fmt.Sprintf("move failed: %v", err),
			})
		} else {
			if result.Moved {
				successCount++
			}
			results = append(results, *result)
		}
	}

	return &BatchMoveResult{
		Results:      results,
		Total:        len(req.Files),
		SuccessCount: successCount,
		FailureCount: failureCount,
	}, nil
}

// logMove appends a move operation to the audit log.
func (m *Manager) logMove(log MoveLog) error {
	logPath := filepath.Join(m.root, "metadata", "move_log.ndjson")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	// Open file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write as newline-delimited JSON
	encoder := json.NewEncoder(file)
	return encoder.Encode(log)
}

