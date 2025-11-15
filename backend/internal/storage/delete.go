package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrInvalidInput is returned when the input validation fails (e.g., empty hash).
var ErrInvalidInput = errors.New("invalid input")

// DeleteRequest captures parameters for deleting a file.
type DeleteRequest struct {
	Hash string `json:"hash"`
}

// DeleteResult surfaces the outcome of a deletion operation.
type DeleteResult struct {
	Hash         string    `json:"hash"`
	OriginalName string    `json:"original_name"`
	StoredPath   string    `json:"stored_path"`
	Deleted      bool      `json:"deleted"`
	DeletedAt    time.Time `json:"deleted_at"`
	Message      string    `json:"message,omitempty"`
}

// DeleteLog captures audit trail for deletion operations.
type DeleteLog struct {
	Hash         string    `json:"hash"`
	OriginalName string    `json:"original_name"`
	StoredPath   string    `json:"stored_path"`
	Category     string    `json:"category"`
	MimeType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	DeletedAt    time.Time `json:"deleted_at"`
}

// DeleteFile deletes a file identified by hash, removing both the physical file and metadata.
func (m *Manager) DeleteFile(req DeleteRequest) (*DeleteResult, error) {
	if req.Hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrInvalidInput)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing file by hash
	existing := m.index.FindByHash(req.Hash)
	if existing == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, req.Hash)
	}

	// Capture timestamp once for consistency
	deletedAt := time.Now().UTC()

	// Check if this is a hard link (has references)
	filePath := filepath.Join(m.root, existing.StoredPath)
	shouldDeletePhysicalFile := true
	
	if m.referenceIndex != nil {
		refCount := m.referenceIndex.GetReferenceCount(filePath)
		if refCount > 1 {
			// This is a hard link with other references - only delete metadata
			shouldDeletePhysicalFile = false
		}
	}

	// Delete metadata first to maintain consistency
	// If metadata deletion fails, we haven't touched the physical file yet
	if err := m.index.Delete(req.Hash); err != nil {
		return nil, fmt.Errorf("failed to delete metadata: %w", err)
	}

	// Remove reference if it exists and check if we should delete physical file
	if m.referenceIndex != nil {
		_ = m.referenceIndex.RemoveReference(filePath, req.Hash)
		// After removal, check if there are any remaining references
		remainingRefs := m.referenceIndex.GetReferenceCount(filePath)
		if remainingRefs > 0 {
			shouldDeletePhysicalFile = false
		}
	}

	// Delete the physical file only if it's not a hard link or it's the last reference
	if shouldDeletePhysicalFile {
		if err := os.Remove(filePath); err != nil {
			// If file doesn't exist on disk, that's okay - metadata is already deleted
			// If it's a different error, attempt to rollback metadata deletion
			if !errors.Is(err, os.ErrNotExist) {
				// Attempt to restore metadata entry
				// Note: This is best-effort; if it fails, we log the inconsistency
				if restoreErr := m.index.Add(*existing); restoreErr != nil {
					// Log both errors but return the original file deletion error
					// The metadata is now inconsistent (deleted but file still exists)
					return nil, fmt.Errorf("failed to delete file (metadata rollback also failed): %w (rollback: %v)", err, restoreErr)
				}
				return nil, fmt.Errorf("failed to delete file: %w", err)
			}
		}
	}

	// Log the deletion operation
	logEntry := DeleteLog{
		Hash:         existing.Hash,
		OriginalName: existing.OriginalName,
		StoredPath:   existing.StoredPath,
		Category:     existing.Category,
		MimeType:     existing.MimeType,
		Size:         existing.Size,
		DeletedAt:    deletedAt,
	}
	if err := m.logDelete(logEntry); err != nil {
		// Log the audit log failure but don't fail the deletion
		// Try to log to stderr as fallback
		fmt.Fprintf(os.Stderr, "audit log delete failed: %v\n", err)
	}

	return &DeleteResult{
		Hash:         existing.Hash,
		OriginalName: existing.OriginalName,
		StoredPath:   existing.StoredPath,
		Deleted:      true,
		DeletedAt:    deletedAt,
		Message:      fmt.Sprintf("deleted file %s", existing.OriginalName),
	}, nil
}

// logDelete appends a deletion operation to the audit log.
func (m *Manager) logDelete(log DeleteLog) error {
	logPath := filepath.Join(m.root, "metadata", "delete_log.ndjson")

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
