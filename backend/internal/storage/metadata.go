package storage

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sort"
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

// FileQuery defines filtering and sorting options for querying files.
type FileQuery struct {
    Category      string
    MimeType      string
    MinSize       int64
    MaxSize       int64
    UploadedAfter time.Time
    UploadedBefore time.Time
    SearchTerm    string
    SortBy        string // "name", "date", "size", "category"
    SortOrder     string // "asc", "desc"
    Limit         int
    Offset        int
}

// Query returns files matching the given criteria.
func (idx *MetadataIndex) Query(q FileQuery) []FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    results := make([]FileMetadata, 0)
    for _, meta := range idx.data {
        if !matchesQuery(meta, q) {
            continue
        }
        results = append(results, meta)
    }

    sortFiles(results, q.SortBy, q.SortOrder)

    // Apply pagination
    start := q.Offset
    if start > len(results) {
        return []FileMetadata{}
    }
    end := start + q.Limit
    if q.Limit == 0 || end > len(results) {
        end = len(results)
    }

    return results[start:end]
}

// Count returns the total number of files matching the query.
func (idx *MetadataIndex) Count(q FileQuery) int {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    count := 0
    for _, meta := range idx.data {
        if matchesQuery(meta, q) {
            count++
        }
    }
    return count
}

// ListAll returns all files in the index.
func (idx *MetadataIndex) ListAll() []FileMetadata {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    results := make([]FileMetadata, 0, len(idx.data))
    for _, meta := range idx.data {
        results = append(results, meta)
    }
    return results
}

// GetCategories returns all unique categories with file counts and total sizes.
func (idx *MetadataIndex) GetCategories() map[string]CategoryStats {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    stats := make(map[string]CategoryStats)
    for _, meta := range idx.data {
        s := stats[meta.Category]
        s.Count++
        s.Size += meta.Size
        stats[meta.Category] = s
    }
    return stats
}

// GetStats returns overall storage statistics.
func (idx *MetadataIndex) GetStats() StorageStats {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    stats := StorageStats{
        Categories: make(map[string]CategoryStats),
        FileTypes:  make(map[string]int),
    }

    now := time.Now().UTC()
    day24h := now.Add(-24 * time.Hour)
    day7 := now.Add(-7 * 24 * time.Hour)
    day30 := now.Add(-30 * 24 * time.Hour)

    for _, meta := range idx.data {
        stats.TotalFiles++
        stats.TotalSize += meta.Size

        // Category stats
        cs := stats.Categories[meta.Category]
        cs.Count++
        cs.Size += meta.Size
        stats.Categories[meta.Category] = cs

        // File type stats
        stats.FileTypes[meta.MimeType]++

        // Recent uploads
        if meta.UploadedAt.After(day24h) {
            stats.Recent24h++
        }
        if meta.UploadedAt.After(day7) {
            stats.Recent7d++
        }
        if meta.UploadedAt.After(day30) {
            stats.Recent30d++
        }
    }

    return stats
}

// CategoryStats holds statistics for a category.
type CategoryStats struct {
    Count int   `json:"count"`
    Size  int64 `json:"size"`
}

// StorageStats holds overall storage statistics.
type StorageStats struct {
    TotalFiles int                       `json:"total_files"`
    TotalSize  int64                     `json:"total_size"`
    Categories map[string]CategoryStats  `json:"categories"`
    FileTypes  map[string]int            `json:"file_types"`
    Recent24h  int                       `json:"recent_24h"`
    Recent7d   int                       `json:"recent_7d"`
    Recent30d  int                       `json:"recent_30d"`
}

func matchesQuery(meta FileMetadata, q FileQuery) bool {
    if q.Category != "" && meta.Category != q.Category {
        return false
    }
    if q.MimeType != "" && meta.MimeType != q.MimeType {
        return false
    }
    if q.MinSize > 0 && meta.Size < q.MinSize {
        return false
    }
    if q.MaxSize > 0 && meta.Size > q.MaxSize {
        return false
    }
    if !q.UploadedAfter.IsZero() && meta.UploadedAt.Before(q.UploadedAfter) {
        return false
    }
    if !q.UploadedBefore.IsZero() && meta.UploadedAt.After(q.UploadedBefore) {
        return false
    }
    if q.SearchTerm != "" {
        searchLower := strings.ToLower(q.SearchTerm)
        if !strings.Contains(strings.ToLower(meta.OriginalName), searchLower) &&
            !strings.Contains(strings.ToLower(meta.Category), searchLower) {
            return false
        }
    }
    return true
}

func sortFiles(files []FileMetadata, sortBy, order string) {
    if order == "" {
        order = "asc"
    }

    less := func(i, j int) bool {
        var result bool
        switch sortBy {
        case "name":
            result = files[i].OriginalName < files[j].OriginalName
        case "size":
            result = files[i].Size < files[j].Size
        case "category":
            result = files[i].Category < files[j].Category
        case "date":
            fallthrough
        default:
            result = files[i].UploadedAt.Before(files[j].UploadedAt)
        }
        if order == "desc" {
            return !result
        }
        return result
    }

    sort.Slice(files, less)
}
