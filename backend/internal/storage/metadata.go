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

// Update modifies an existing metadata entry.
func (idx *MetadataIndex) Update(meta FileMetadata) error {
    idx.mu.Lock()
    defer idx.mu.Unlock()
    if _, exists := idx.data[meta.Hash]; !exists {
        return errors.New("metadata not found")
    }
    idx.data[meta.Hash] = meta
    return idx.persistLocked()
}

// FindByPath searches for a file by its stored path.
func (idx *MetadataIndex) FindByPath(path string) *FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    for _, meta := range idx.data {
        if meta.StoredPath == path {
            clone := meta
            return &clone
        }
    }
    return nil
}
