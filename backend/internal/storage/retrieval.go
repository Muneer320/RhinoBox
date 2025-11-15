package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrInvalidPath is returned when the path is invalid or contains path traversal.
	ErrInvalidPath = errors.New("invalid path")
)

// FileRetrievalResult contains file metadata and a reader for the file content.
type FileRetrievalResult struct {
	Metadata FileMetadata
	Reader   *os.File
	Size     int64
}

// GetFileByHash retrieves a file by its hash identifier.
func (m *Manager) GetFileByHash(hash string) (*FileRetrievalResult, error) {
	if hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrFileNotFound)
	}

	m.mu.Lock()
	metadata := m.index.FindByHash(hash)
	m.mu.Unlock()

	if metadata == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, hash)
	}

	return m.getFileByMetadata(*metadata)
}

// GetFileByPath retrieves a file by its stored path.
func (m *Manager) GetFileByPath(storedPath string) (*FileRetrievalResult, error) {
	if storedPath == "" {
		return nil, fmt.Errorf("%w: path is required", ErrFileNotFound)
	}

	// Security: prevent path traversal
	if err := validatePath(storedPath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}

	// Find metadata by stored path
	m.mu.Lock()
	var metadata *FileMetadata
	for _, meta := range m.index.data {
		if meta.StoredPath == storedPath {
			metaCopy := meta
			metadata = &metaCopy
			break
		}
	}
	m.mu.Unlock()

	if metadata == nil {
		return nil, fmt.Errorf("%w: path %s", ErrFileNotFound, storedPath)
	}

	return m.getFileByMetadata(*metadata)
}

// GetFileMetadata retrieves file metadata without opening the file.
func (m *Manager) GetFileMetadata(hash string) (*FileMetadata, error) {
	if hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrFileNotFound)
	}

	m.mu.Lock()
	metadata := m.index.FindByHash(hash)
	m.mu.Unlock()

	if metadata == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, hash)
	}

	// Verify file still exists on disk
	fullPath := filepath.Join(m.root, metadata.StoredPath)
	if _, err := os.Stat(fullPath); err != nil {
		return nil, fmt.Errorf("%w: file on disk not found", ErrFileNotFound)
	}

	// Return a copy to prevent external modification
	metaCopy := *metadata
	return &metaCopy, nil
}

// getFileByMetadata opens the file and returns a retrieval result.
func (m *Manager) getFileByMetadata(metadata FileMetadata) (*FileRetrievalResult, error) {
	fullPath := filepath.Join(m.root, metadata.StoredPath)

	// Security: ensure the resolved path is within the root directory
	absRoot, err := filepath.Abs(m.root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root path: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if the resolved path is within the root directory
	relPath, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return nil, fmt.Errorf("%w: path outside root directory", ErrInvalidPath)
	}

	// Prevent path traversal (should not contain "..")
	if strings.Contains(relPath, "..") {
		return nil, fmt.Errorf("%w: path traversal detected", ErrInvalidPath)
	}

	// Open the file
	file, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: file not found on disk", ErrFileNotFound)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info for size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &FileRetrievalResult{
		Metadata: metadata,
		Reader:   file,
		Size:     info.Size(),
	}, nil
}

// validatePath checks if a path is safe and doesn't contain path traversal attempts.
func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected")
	}

	// Check for absolute paths (should be relative)
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths not allowed")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null bytes not allowed")
	}

	return nil
}

// DownloadLog captures download events for analytics.
type DownloadLog struct {
	Hash         string    `json:"hash"`
	StoredPath   string    `json:"stored_path"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	DownloadedAt time.Time `json:"downloaded_at"`
	RangeStart   *int64    `json:"range_start,omitempty"`
	RangeEnd     *int64    `json:"range_end,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	IPAddress    string    `json:"ip_address,omitempty"`
}

// LogDownload appends a download event to the audit log.
func (m *Manager) LogDownload(log DownloadLog) error {
	logPath := filepath.Join(m.root, "metadata", "download_log.ndjson")

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
