package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
		return nil, fmt.Errorf("%w: hash is required", ErrFileNotFound)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing file by hash
	existing := m.index.FindByHash(req.Hash)
	if existing == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, req.Hash)
	}

	// Delete the physical file
	filePath := filepath.Join(m.root, existing.StoredPath)
	if err := os.Remove(filePath); err != nil {
		// If file doesn't exist on disk, continue with metadata deletion
		// This handles cases where file was manually deleted but metadata remains
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to delete file: %w", err)
		}
	}

	// Remove metadata entry
	if err := m.index.Delete(req.Hash); err != nil {
		return nil, fmt.Errorf("failed to delete metadata: %w", err)
	}

	// Log the deletion operation
	logEntry := DeleteLog{
		Hash:         existing.Hash,
		OriginalName: existing.OriginalName,
		StoredPath:   existing.StoredPath,
		Category:     existing.Category,
		MimeType:     existing.MimeType,
		Size:         existing.Size,
		DeletedAt:    time.Now().UTC(),
	}
	_ = m.logDelete(logEntry) // Best effort logging

	return &DeleteResult{
		Hash:         existing.Hash,
		OriginalName: existing.OriginalName,
		StoredPath:   existing.StoredPath,
		Deleted:      true,
		DeletedAt:    time.Now().UTC(),
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
