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

// SearchQuery contains parameters for searching files.
type SearchQuery struct {
    Name         string    // Pattern match on original name (case-insensitive substring)
    Extension    string    // File extension (e.g., ".pdf", "pdf")
    Category     string    // Category/type (e.g., "images", "documents")
    MimeType     string    // MIME type (exact or prefix match)
    Hash         string    // Exact hash match
    MinSize      int64     // Minimum file size in bytes
    MaxSize      int64     // Maximum file size in bytes
    DateFrom     time.Time // Uploaded at or after this date
    DateTo       time.Time // Uploaded at or before this date
    SortBy       string    // Field to sort by: "name", "size", "date"
    SortOrder    string    // "asc" or "desc"
    Limit        int       // Max results to return (0 = no limit)
    Offset       int       // Skip this many results
}

// SearchResult contains matching files and pagination info.
type SearchResult struct {
    Files      []FileMetadata `json:"files"`
    Total      int            `json:"total"`
    Offset     int            `json:"offset"`
    Limit      int            `json:"limit"`
    HasMore    bool           `json:"has_more"`
}

// Search finds files matching the query criteria.
func (idx *MetadataIndex) Search(query SearchQuery) *SearchResult {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    // Collect all matching files
    matches := make([]FileMetadata, 0)
    for _, meta := range idx.data {
        if idx.matchesQuery(meta, query) {
            matches = append(matches, meta)
        }
    }

    // Sort results
    idx.sortFiles(matches, query.SortBy, query.SortOrder)

    // Apply pagination
    total := len(matches)
    start := query.Offset
    if start < 0 {
        start = 0
    }
    if start > total {
        start = total
    }

    end := total
    if query.Limit > 0 {
        end = start + query.Limit
        if end > total {
            end = total
        }
    }

    paginatedFiles := matches[start:end]
    hasMore := end < total

    return &SearchResult{
        Files:   paginatedFiles,
        Total:   total,
        Offset:  start,
        Limit:   query.Limit,
        HasMore: hasMore,
    }
}

// matchesQuery checks if a file matches all query criteria.
func (idx *MetadataIndex) matchesQuery(meta FileMetadata, query SearchQuery) bool {
    // Name pattern match (case-insensitive substring)
    if query.Name != "" {
        lowerName := strings.ToLower(meta.OriginalName)
        lowerQuery := strings.ToLower(query.Name)
        if !strings.Contains(lowerName, lowerQuery) {
            return false
        }
    }

    // Extension match
    if query.Extension != "" {
        ext := strings.ToLower(filepath.Ext(meta.OriginalName))
        queryExt := strings.ToLower(query.Extension)
        if !strings.HasPrefix(queryExt, ".") {
            queryExt = "." + queryExt
        }
        if ext != queryExt {
            return false
        }
    }

    // Category match (case-insensitive substring)
    if query.Category != "" {
        lowerCategory := strings.ToLower(meta.Category)
        lowerQuery := strings.ToLower(query.Category)
        if !strings.Contains(lowerCategory, lowerQuery) {
            return false
        }
    }

    // MIME type match (prefix match for flexibility)
    if query.MimeType != "" {
        lowerMime := strings.ToLower(meta.MimeType)
        lowerQuery := strings.ToLower(query.MimeType)
        if !strings.HasPrefix(lowerMime, lowerQuery) {
            return false
        }
    }

    // Hash match (exact)
    if query.Hash != "" && meta.Hash != query.Hash {
        return false
    }

    // Size range
    if query.MinSize > 0 && meta.Size < query.MinSize {
        return false
    }
    if query.MaxSize > 0 && meta.Size > query.MaxSize {
        return false
    }

    // Date range
    if !query.DateFrom.IsZero() && meta.UploadedAt.Before(query.DateFrom) {
        return false
    }
    if !query.DateTo.IsZero() && meta.UploadedAt.After(query.DateTo) {
        return false
    }

    return true
}

// sortFiles sorts the file list by the specified field and order.
func (idx *MetadataIndex) sortFiles(files []FileMetadata, sortBy, sortOrder string) {
    if len(files) == 0 {
        return
    }

    desc := sortOrder == "desc"

    switch sortBy {
    case "name":
        sort.Slice(files, func(i, j int) bool {
            if desc {
                return files[i].OriginalName > files[j].OriginalName
            }
            return files[i].OriginalName < files[j].OriginalName
        })
    case "size":
        sort.Slice(files, func(i, j int) bool {
            if desc {
                return files[i].Size > files[j].Size
            }
            return files[i].Size < files[j].Size
        })
    case "date":
        sort.Slice(files, func(i, j int) bool {
            if desc {
                return files[i].UploadedAt.After(files[j].UploadedAt)
            }
            return files[i].UploadedAt.Before(files[j].UploadedAt)
        })
    default:
        // Default sort by upload date (newest first)
        sort.Slice(files, func(i, j int) bool {
            return files[i].UploadedAt.After(files[j].UploadedAt)
        })
    }
}
