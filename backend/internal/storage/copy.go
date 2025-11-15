package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	// ErrCopyConflict is returned when a copy operation would create a naming conflict.
	ErrCopyConflict = errors.New("copy conflict")
)

// ReferenceIndex tracks hard links - files that share the same physical file.
// Maps physical file path -> set of metadata hashes that reference it.
type ReferenceIndex struct {
	path string
	mu   sync.RWMutex
	data map[string]map[string]bool // physicalPath -> set of hashes
}

// NewReferenceIndex creates a new reference index.
func NewReferenceIndex(path string) (*ReferenceIndex, error) {
	idx := &ReferenceIndex{
		path: path,
		data: make(map[string]map[string]bool),
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (idx *ReferenceIndex) load() error {
	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	raw, err := os.ReadFile(idx.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if len(raw) == 0 {
		return nil
	}

	var items []struct {
		PhysicalPath string   `json:"physical_path"`
		Hashes       []string `json:"hashes"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}

	for _, item := range items {
		hashSet := make(map[string]bool)
		for _, hash := range item.Hashes {
			hashSet[hash] = true
		}
		idx.data[item.PhysicalPath] = hashSet
	}
	return nil
}

func (idx *ReferenceIndex) persistLocked() error {
	items := make([]struct {
		PhysicalPath string   `json:"physical_path"`
		Hashes       []string `json:"hashes"`
	}, 0, len(idx.data))

	for path, hashSet := range idx.data {
		hashes := make([]string, 0, len(hashSet))
		for hash := range hashSet {
			hashes = append(hashes, hash)
		}
		items = append(items, struct {
			PhysicalPath string   `json:"physical_path"`
			Hashes       []string `json:"hashes"`
		}{PhysicalPath: path, Hashes: hashes})
	}

	tmp := idx.path + ".tmp"
	buf, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, idx.path)
}

// AddReference adds a hash reference to a physical file path.
func (idx *ReferenceIndex) AddReference(physicalPath, hash string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.data[physicalPath] == nil {
		idx.data[physicalPath] = make(map[string]bool)
	}
	idx.data[physicalPath][hash] = true
	return idx.persistLocked()
}

// RemoveReference removes a hash reference from a physical file path.
func (idx *ReferenceIndex) RemoveReference(physicalPath, hash string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if hashSet, exists := idx.data[physicalPath]; exists {
		delete(hashSet, hash)
		if len(hashSet) == 0 {
			delete(idx.data, physicalPath)
		}
		return idx.persistLocked()
	}
	return nil
}

// GetReferenceCount returns the number of references to a physical file.
func (idx *ReferenceIndex) GetReferenceCount(physicalPath string) int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.data[physicalPath])
}

// GetReferences returns all hashes that reference a physical file.
func (idx *ReferenceIndex) GetReferences(physicalPath string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	hashSet := idx.data[physicalPath]
	if hashSet == nil {
		return nil
	}
	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}
	return hashes
}

// CopyRequest captures parameters for copying a file.
type CopyRequest struct {
	Hash         string            `json:"hash"`
	NewName      string            `json:"new_name,omitempty"`
	NewCategory  string            `json:"new_category,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	HardLink     bool              `json:"hard_link,omitempty"`
}

// CopyResult surfaces the outcome of a copy operation.
type CopyResult struct {
	OriginalHash string            `json:"original_hash"`
	NewHash      string            `json:"new_hash"`
	OriginalMeta FileMetadata      `json:"original_metadata"`
	NewMeta      FileMetadata      `json:"new_metadata"`
	HardLink     bool              `json:"hard_link"`
	Message      string            `json:"message,omitempty"`
}

// CopyLog captures audit trail for copy operations.
type CopyLog struct {
	OriginalHash string            `json:"original_hash"`
	NewHash      string            `json:"new_hash"`
	OriginalName string            `json:"original_name"`
	NewName      string            `json:"new_name"`
	OriginalPath string            `json:"original_path"`
	NewPath      string            `json:"new_path"`
	HardLink     bool              `json:"hard_link"`
	CopiedAt     time.Time         `json:"copied_at"`
}

// CopyFile creates a copy of an existing file with new metadata.
func (m *Manager) CopyFile(req CopyRequest) (*CopyResult, error) {
	if req.Hash == "" {
		return nil, fmt.Errorf("%w: hash is required", ErrFileNotFound)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the original file
	original := m.index.FindByHash(req.Hash)
	if original == nil {
		return nil, fmt.Errorf("%w: hash %s", ErrFileNotFound, req.Hash)
	}

	// Determine new name
	newName := req.NewName
	if newName == "" {
		// Generate a default name based on original
		ext := filepath.Ext(original.OriginalName)
		base := strings.TrimSuffix(original.OriginalName, ext)
		if base == "" {
			base = "file"
		}
		newName = fmt.Sprintf("%s_copy%s", base, ext)
	}

	// Validate new filename
	if err := ValidateFilename(newName); err != nil {
		return nil, err
	}

	// Determine new category
	newCategory := req.NewCategory
	if newCategory == "" {
		newCategory = original.Category
	}

	// Generate new hash for the copy (different from original)
	// For hard links, we'll use a different hash but track the reference
	newHash := generateCopyHash(original.Hash, newName, time.Now())

	// Check for name conflicts in the new category
	if m.checkNameConflictInCategory(newHash, newName, newCategory) {
		return nil, fmt.Errorf("%w: file with name %s already exists in category %s", ErrCopyConflict, newName, newCategory)
	}

	originalPath := filepath.Join(m.root, original.StoredPath)

	// Verify original file exists
	if _, err := os.Stat(originalPath); err != nil {
		return nil, fmt.Errorf("original file not found: %w", err)
	}

	var newPath string
	var hardLink bool

	if req.HardLink {
		// Hard link mode: create a new metadata entry pointing to the same file
		// Use the same stored path
		newPath = original.StoredPath
		hardLink = true

		// Initialize reference index if not already done
		if m.referenceIndex == nil {
			refIdx, err := NewReferenceIndex(filepath.Join(m.root, "metadata", "references.json"))
			if err != nil {
				return nil, fmt.Errorf("failed to initialize reference index: %w", err)
			}
			m.referenceIndex = refIdx
		}

		// Add both original and new hash to the reference index
		// This tracks all metadata entries that point to the same physical file
		if err := m.referenceIndex.AddReference(originalPath, original.Hash); err != nil {
			return nil, fmt.Errorf("failed to add original reference: %w", err)
		}
		if err := m.referenceIndex.AddReference(originalPath, newHash); err != nil {
			// Rollback original reference
			_ = m.referenceIndex.RemoveReference(originalPath, original.Hash)
			return nil, fmt.Errorf("failed to add new reference: %w", err)
		}
	} else {
		// Full copy mode: duplicate the physical file
		components := m.classifier.Classify(original.MimeType, newName, newCategory)
		fullDir := filepath.Join(append([]string{m.storageRoot}, components...)...)
		if err := os.MkdirAll(fullDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		base := strings.TrimSuffix(newName, filepath.Ext(newName))
		if base == "" {
			base = "file"
		}
		base = sanitize(base)
		ext := strings.ToLower(filepath.Ext(newName))
		if ext == "" {
			ext = ""
		}

		filename := fmt.Sprintf("%s_%s%s", newHash[:12], base, ext)
		newPath = filepath.Join(fullDir, filename)

		// Copy the file
		if err := copyFile(originalPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}

		// Get relative path
		rel, err := filepath.Rel(m.root, newPath)
		if err != nil {
			_ = os.Remove(newPath)
			return nil, fmt.Errorf("failed to compute relative path: %w", err)
		}
		newPath = filepath.ToSlash(rel)
	}

	// Merge metadata
	newMetadata := make(map[string]string)
	if original.Metadata != nil {
		for k, v := range original.Metadata {
			newMetadata[k] = v
		}
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			newMetadata[k] = v
		}
	}

	// Create new metadata entry
	newMeta := FileMetadata{
		Hash:         newHash,
		OriginalName: newName,
		StoredPath:   newPath,
		Category:     newCategory,
		MimeType:     original.MimeType,
		Size:         original.Size,
		UploadedAt:   time.Now().UTC(),
		Metadata:     newMetadata,
	}

	// Add to index
	if err := m.index.Add(newMeta); err != nil {
		// Rollback: remove references or delete copied file
		if req.HardLink {
			_ = m.referenceIndex.RemoveReference(originalPath, original.Hash)
			_ = m.referenceIndex.RemoveReference(originalPath, newHash)
		} else {
			_ = os.Remove(filepath.Join(m.root, newPath))
		}
		return nil, fmt.Errorf("failed to add metadata: %w", err)
	}

	// Log the copy operation
	logEntry := CopyLog{
		OriginalHash: original.Hash,
		NewHash:      newHash,
		OriginalName: original.OriginalName,
		NewName:      newName,
		OriginalPath: original.StoredPath,
		NewPath:      newPath,
		HardLink:     hardLink,
		CopiedAt:     time.Now().UTC(),
	}
	_ = m.logCopy(logEntry) // Best effort logging

	return &CopyResult{
		OriginalHash: original.Hash,
		NewHash:      newHash,
		OriginalMeta: *original,
		NewMeta:      newMeta,
		HardLink:     hardLink,
		Message:      fmt.Sprintf("copied %s to %s", original.OriginalName, newName),
	}, nil
}

// checkNameConflictInCategory checks if a name conflicts in a specific category.
func (m *Manager) checkNameConflictInCategory(excludeHash, name, category string) bool {
	for hash, meta := range m.index.data {
		if hash == excludeHash {
			continue
		}
		if meta.Category == category && strings.EqualFold(meta.OriginalName, name) {
			return true
		}
	}
	return false
}

// generateCopyHash generates a unique hash for a copy based on original hash, name, and timestamp.
func generateCopyHash(originalHash, newName string, timestamp time.Time) string {
	// Use a combination of original hash, new name, and timestamp to generate unique hash
	// This ensures each copy gets a unique identifier
	input := fmt.Sprintf("%s:%s:%d", originalHash, newName, timestamp.UnixNano())
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

// copyFile copies a file from source to destination.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		_ = os.Remove(dst)
		return err
	}

	return destFile.Sync()
}

// logCopy appends a copy operation to the audit log.
func (m *Manager) logCopy(log CopyLog) error {
	logPath := filepath.Join(m.root, "metadata", "copy_log.ndjson")

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

