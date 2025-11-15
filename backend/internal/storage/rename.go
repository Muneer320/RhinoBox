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

// RenameRequest captures parameters for renaming a file.
type RenameRequest struct {
	Hash             string `json:"hash"`
	NewName          string `json:"new_name"`
	UpdateStoredFile bool   `json:"update_stored_file"`
}

// RenameResult surfaces the outcome of a rename operation.
type RenameResult struct {
	OldMetadata FileMetadata `json:"old_metadata"`
	NewMetadata FileMetadata `json:"new_metadata"`
	Renamed     bool         `json:"renamed"`
	Message     string       `json:"message,omitempty"`
}

// RenameLog captures audit trail for rename operations.
type RenameLog struct {
	Hash              string    `json:"hash"`
	OldOriginalName   string    `json:"old_original_name"`
	NewOriginalName   string    `json:"new_original_name"`
	OldStoredPath     string    `json:"old_stored_path"`
	NewStoredPath     string    `json:"new_stored_path"`
	UpdatedStoredFile bool      `json:"updated_stored_file"`
	RenamedAt         time.Time `json:"renamed_at"`
}

var (
	// ErrFileNotFound is returned when the requested file doesn't exist.
	ErrFileNotFound = errors.New("file not found")
	// ErrInvalidFilename is returned when the new filename is invalid.
	ErrInvalidFilename = errors.New("invalid filename")
	// ErrNameConflict is returned when the new name conflicts with an existing file.
	ErrNameConflict = errors.New("filename conflict")
)

// Filename validation constants
const (
	maxFilenameLength = 255
	minFilenameLength = 1
)

// Invalid filename patterns
var (
	// Path traversal attempts
	pathTraversalPattern = regexp.MustCompile(`\.\.`)
	// Control characters (0x00-0x1F)
	controlCharsPattern = regexp.MustCompile(`[\x00-\x1F]`)
	// Reserved Windows names
	reservedWindowsNames = map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true,
		"COM9": true, "LPT1": true, "LPT2": true, "LPT3": true,
		"LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true,
		"LPT8": true, "LPT9": true,
	}
	// Invalid characters in filenames
	invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*]`)
)

// ValidateFilename checks if a filename is safe and valid.
func ValidateFilename(filename string) error {
	if filename == "" || len(filename) < minFilenameLength {
		return fmt.Errorf("%w: filename too short", ErrInvalidFilename)
	}
	if len(filename) > maxFilenameLength {
		return fmt.Errorf("%w: filename exceeds %d characters", ErrInvalidFilename, maxFilenameLength)
	}

	// Check for path traversal
	if pathTraversalPattern.MatchString(filename) {
		return fmt.Errorf("%w: path traversal detected", ErrInvalidFilename)
	}

	// Check for directory separators
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("%w: directory separators not allowed", ErrInvalidFilename)
	}

	// Check for control characters
	if controlCharsPattern.MatchString(filename) {
		return fmt.Errorf("%w: control characters not allowed", ErrInvalidFilename)
	}

	// Check for invalid characters
	if invalidFilenameChars.MatchString(filename) {
		return fmt.Errorf("%w: contains invalid characters", ErrInvalidFilename)
	}

	// Check for reserved Windows names
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if reservedWindowsNames[strings.ToUpper(base)] {
		return fmt.Errorf("%w: reserved system name", ErrInvalidFilename)
	}

	// Check for leading/trailing dots or spaces
	trimmed := strings.TrimSpace(filename)
	if trimmed != filename {
		return fmt.Errorf("%w: leading/trailing spaces not allowed", ErrInvalidFilename)
	}
	if strings.HasPrefix(filename, ".") || strings.HasSuffix(filename, ".") {
		return fmt.Errorf("%w: leading/trailing dots not allowed", ErrInvalidFilename)
	}

	return nil
}

// RenameFile renames a file identified by hash, with options for metadata-only or full rename.
func (m *Manager) RenameFile(req RenameRequest) (*RenameResult, error) {
	// Validate new filename
	if err := ValidateFilename(req.NewName); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing file by hash
	existing := m.index.FindByHash(req.Hash)
	if existing == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, req.Hash)
	}

	// Create a copy for the result
	oldMetadata := *existing
	newMetadata := *existing

	// Update the original name in metadata
	newMetadata.OriginalName = req.NewName

	// If updating the stored file, perform the rename
	if req.UpdateStoredFile {
		oldPath := filepath.Join(m.root, existing.StoredPath)

		// Check if the file actually exists on disk
		if _, err := os.Stat(oldPath); err != nil {
			return nil, fmt.Errorf("stored file not found: %w", err)
		}

		// Extract the directory and generate new stored filename
		dir := filepath.Dir(oldPath)
		ext := strings.ToLower(filepath.Ext(req.NewName))
		base := strings.TrimSuffix(req.NewName, filepath.Ext(req.NewName))
		if base == "" {
			base = "file"
		}
		base = sanitize(base)

		// Use hash prefix for uniqueness (same pattern as StoreFile)
		newFilename := fmt.Sprintf("%s_%s%s", existing.Hash[:12], base, ext)
		newPath := filepath.Join(dir, newFilename)

		// Check for filename conflicts
		if oldPath != newPath {
			if _, err := os.Stat(newPath); err == nil {
				return nil, fmt.Errorf("%w: file %s already exists", ErrNameConflict, newFilename)
			}
		}

		// Perform the actual file rename
		if err := os.Rename(oldPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to rename stored file: %w", err)
		}

		// Update the stored path in metadata
		rel, err := filepath.Rel(m.root, newPath)
		if err != nil {
			// Rollback the file rename
			_ = os.Rename(newPath, oldPath)
			return nil, fmt.Errorf("failed to compute relative path: %w", err)
		}
		newMetadata.StoredPath = filepath.ToSlash(rel)
	}

	// Update metadata in the index
	m.index.data[req.Hash] = newMetadata
	if err := m.index.persistLocked(); err != nil {
		// If persistence fails and we renamed the file, try to rollback
		if req.UpdateStoredFile {
			oldPath := filepath.Join(m.root, oldMetadata.StoredPath)
			newPath := filepath.Join(m.root, newMetadata.StoredPath)
			_ = os.Rename(newPath, oldPath)
		}
		return nil, fmt.Errorf("failed to persist metadata: %w", err)
	}

	// Log the rename operation
	logEntry := RenameLog{
		Hash:              req.Hash,
		OldOriginalName:   oldMetadata.OriginalName,
		NewOriginalName:   newMetadata.OriginalName,
		OldStoredPath:     oldMetadata.StoredPath,
		NewStoredPath:     newMetadata.StoredPath,
		UpdatedStoredFile: req.UpdateStoredFile,
		RenamedAt:         time.Now().UTC(),
	}
	_ = m.logRename(logEntry) // Best effort logging

	return &RenameResult{
		OldMetadata: oldMetadata,
		NewMetadata: newMetadata,
		Renamed:     true,
		Message:     fmt.Sprintf("renamed %s to %s", oldMetadata.OriginalName, newMetadata.OriginalName),
	}, nil
}

// logRename appends a rename operation to the audit log.
func (m *Manager) logRename(log RenameLog) error {
	logPath := filepath.Join(m.root, "metadata", "rename_log.ndjson")

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

// FindByOriginalName searches for files by their original name (case-insensitive partial match).
func (m *Manager) FindByOriginalName(name string) []FileMetadata {
	m.mu.Lock()
	defer m.mu.Unlock()

	searchLower := strings.ToLower(name)
	results := make([]FileMetadata, 0)

	for _, meta := range m.index.data {
		if strings.Contains(strings.ToLower(meta.OriginalName), searchLower) {
			results = append(results, meta)
		}
	}

	return results
}

// CheckNameConflict checks if a filename would conflict with existing files in the same category.
func (m *Manager) CheckNameConflict(hash, newName, category string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for h, meta := range m.index.data {
		// Skip the file being renamed
		if h == hash {
			continue
		}
		// Check for same name in same category
		if meta.Category == category && strings.EqualFold(meta.OriginalName, newName) {
			return true
		}
	}

	return false
}
