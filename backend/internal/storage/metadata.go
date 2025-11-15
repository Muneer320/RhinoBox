package storage

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
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
    RefCount     int               `json:"ref_count,omitempty"`     // Reference count for hard links
    IsHardLink   bool              `json:"is_hard_link,omitempty"`  // True if this is a hard link reference
    LinkedTo     string            `json:"linked_to,omitempty"`     // Hash of the original file if hard link
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

// FindByPath searches for a file by its stored path.
func (idx *MetadataIndex) FindByPath(storedPath string) *FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    for _, meta := range idx.data {
        if meta.StoredPath == storedPath {
            clone := meta
            return &clone
        }
    }
    return nil
}

// Update updates an existing metadata entry.
func (idx *MetadataIndex) Update(meta FileMetadata) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    if _, exists := idx.data[meta.Hash]; !exists {
        return errors.New("metadata not found")
    }
    idx.data[meta.Hash] = meta
    return idx.persistLocked()
}

// IncrementRefCount increases the reference count for a file.
func (idx *MetadataIndex) IncrementRefCount(hash string) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    meta, ok := idx.data[hash]
    if !ok {
        return errors.New("file not found")
    }
    meta.RefCount++
    idx.data[hash] = meta
    return idx.persistLocked()
}

// DecrementRefCount decreases the reference count for a file.
func (idx *MetadataIndex) DecrementRefCount(hash string) (int, error) {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    meta, ok := idx.data[hash]
    if !ok {
        return 0, errors.New("file not found")
    }
    if meta.RefCount > 0 {
        meta.RefCount--
    }
    idx.data[hash] = meta
    if err := idx.persistLocked(); err != nil {
        return meta.RefCount, err
    }
    return meta.RefCount, nil
}
