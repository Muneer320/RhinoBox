package storage

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

// FileMetadata captures stored file information for dedup and auditing.
type FileMetadata struct {
    Hash         string            `json:"hash"`
    OriginalName string            `json:"original_name"`
    StoredPath   string            `json:"stored_path"`
    Category     string            `json:"category"`
    MimeType     string            `json:"mime_type"`
    Size         int64             `json:"size"`
    UploadedAt   time.Time         `json:"uploaded_at"`
    Metadata     map[string]string `json:"metadata"`
}

// MetadataIndex persists file metadata to disk and enables duplicate detection.
type MetadataIndex struct {
    path string
    mu   sync.RWMutex
    data map[string]FileMetadata
}

func NewMetadataIndex(path string) (*MetadataIndex, error) {
    idx := &MetadataIndex{path: path, data: map[string]FileMetadata{}}
    if err := idx.load(); err != nil {
        return nil, err
    }
    return idx, nil
}

func (idx *MetadataIndex) load() error {
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

    var items []FileMetadata
    if len(raw) == 0 {
        return nil
    }
    if err := json.Unmarshal(raw, &items); err != nil {
        return err
    }

    for _, item := range items {
        idx.data[item.Hash] = item
    }
    return nil
}

func (idx *MetadataIndex) persistLocked() error {
    items := make([]FileMetadata, 0, len(idx.data))
    for _, meta := range idx.data {
        items = append(items, meta)
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

func (idx *MetadataIndex) FindByHash(hash string) *FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    if meta, ok := idx.data[hash]; ok {
        clone := meta
        return &clone
    }
    return nil
}

func (idx *MetadataIndex) Add(meta FileMetadata) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    idx.data[meta.Hash] = meta
    return idx.persistLocked()
}

// Delete removes a metadata entry by hash.
func (idx *MetadataIndex) Delete(hash string) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    if _, exists := idx.data[hash]; !exists {
        return errors.New("metadata entry not found")
    }
    delete(idx.data, hash)
    return idx.persistLocked()
}

// FindByType returns all files matching the given collection type.
// The type should match the first component of the Category path (e.g., "images", "videos").
func (idx *MetadataIndex) FindByType(collectionType string) []FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    
    results := make([]FileMetadata, 0)
    typeLower := strings.ToLower(collectionType)
    
    for _, meta := range idx.data {
        // Category format is "type/subcategory/..." or just "type"
        categoryParts := strings.Split(meta.Category, "/")
        if len(categoryParts) > 0 && strings.ToLower(categoryParts[0]) == typeLower {
            results = append(results, meta)
        }
    }
    
    return results
}

// FindByCategoryPrefix returns all files whose category starts with the given prefix.
// This is useful for finding all files in a collection type (e.g., "images" matches "images/jpg", "images/png", etc.).
func (idx *MetadataIndex) FindByCategoryPrefix(prefix string) []FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    
    results := make([]FileMetadata, 0)
    for _, meta := range idx.data {
        // Check if category starts with prefix (exact match or prefix with "/")
        if meta.Category == prefix || 
           (len(meta.Category) > len(prefix) && meta.Category[:len(prefix)+1] == prefix+"/") {
            results = append(results, meta)
        }
    }
    return results
}

// GetAllMetadata returns a copy of all metadata entries.
func (idx *MetadataIndex) GetAllMetadata() []FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    result := make([]FileMetadata, 0, len(idx.data))
    for _, meta := range idx.data {
        result = append(result, meta)
    }
    return result
}
