package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// GetFilesByTypeRequest contains parameters for filtering files by type.
type GetFilesByTypeRequest struct {
	Type     string // Collection type (images, videos, audio, documents, etc.)
	Page     int    // Page number (1-indexed)
	Limit    int    // Number of items per page
	Category string // Optional category filter within the type
}

// GetFilesByTypeResponse contains paginated file results.
type GetFilesByTypeResponse struct {
	Files      []FileMetadata `json:"files"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
}

// GetFilesByType retrieves files filtered by collection type with pagination support.
func (m *Manager) GetFilesByType(req GetFilesByTypeRequest) (*GetFilesByTypeResponse, error) {
	if req.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Default pagination values
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = 50 // Default limit
	}
	if req.Limit > 1000 {
		req.Limit = 1000 // Max limit to prevent excessive memory usage
	}

	// Special-case JSON collections: they are stored as NDJSON batches on disk
	// and are not part of the metadata index. Enumerate JSON batches directly.
	var allFiles []FileMetadata
	if strings.ToLower(req.Type) == "json" {
		var err error
		allFiles, err = m.ListJSONBatches()
		if err != nil {
			return nil, fmt.Errorf("failed to list json batches: %v", err)
		}
	} else {
		m.mu.Lock()
		allFiles = m.index.FindByType(req.Type)
		m.mu.Unlock()
	}

	// Filter by category if specified
	filteredFiles := allFiles
	if req.Category != "" {
		categoryLower := strings.ToLower(req.Category)
		filtered := make([]FileMetadata, 0)
		for _, file := range allFiles {
			// Check if category matches any part of the category path
			categoryParts := strings.Split(file.Category, "/")
			for _, part := range categoryParts {
				if strings.ToLower(part) == categoryLower {
					filtered = append(filtered, file)
					break
				}
			}
		}
		filteredFiles = filtered
	}

	// Sort by upload date (newest first)
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].UploadedAt.After(filteredFiles[j].UploadedAt)
	})

	total := len(filteredFiles)
	totalPages := (total + req.Limit - 1) / req.Limit

	// Calculate pagination
	start := (req.Page - 1) * req.Limit
	end := start + req.Limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// Extract page
	var pageFiles []FileMetadata
	if start < total {
		pageFiles = filteredFiles[start:end]
	} else {
		pageFiles = []FileMetadata{}
	}

	return &GetFilesByTypeResponse{
		Files:      pageFiles,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
	}, nil
}

// ListJSONBatches scans the json directory for NDJSON batch files and returns
// them as FileMetadata entries with namespace/engine information embedded in
// the Metadata map so the API can expose namespace/collection names to the
// frontend.
func (m *Manager) ListJSONBatches() ([]FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	root := filepath.Join(m.root, "json")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileMetadata{}, nil
		}
		return nil, err
	}

	results := make([]FileMetadata, 0)
	for _, engineEntry := range entries {
		if !engineEntry.IsDir() {
			continue
		}
		engine := engineEntry.Name()
		enginePath := filepath.Join(root, engine)
		nsEntries, err := os.ReadDir(enginePath)
		if err != nil {
			continue
		}
		for _, nsEntry := range nsEntries {
			if !nsEntry.IsDir() {
				continue
			}
			namespace := nsEntry.Name()
			nsPath := filepath.Join(enginePath, namespace)
			files, err := os.ReadDir(nsPath)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				name := f.Name()
				if !strings.HasPrefix(name, "batch_") || !strings.HasSuffix(name, ".ndjson") {
					continue
				}
				info, err := f.Info()
				if err != nil {
					continue
				}
				rel := filepath.ToSlash(filepath.Join("json", engine, namespace, name))
				meta := map[string]string{
					"namespace": namespace,
					"engine":    engine,
				}
				fm := FileMetadata{
					Hash:         "",
					OriginalName: name,
					StoredPath:   rel,
					Category:     "json/" + engine,
					MimeType:     "application/x-ndjson",
					Size:         info.Size(),
					UploadedAt:   info.ModTime().UTC(),
					Metadata:     meta,
				}
				results = append(results, fm)
			}
		}
	}

	// Sort by uploaded time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].UploadedAt.After(results[j].UploadedAt)
	})

	return results, nil
}
