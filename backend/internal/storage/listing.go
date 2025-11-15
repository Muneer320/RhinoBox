package storage

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ListOptions contains parameters for listing files.
type ListOptions struct {
	Page      int           // Page number (1-indexed)
	Limit     int           // Items per page
	SortBy    string        // Sort field: name, uploaded_at, size, category, mime_type
	Order     string        // Sort order: asc, desc
	Category  string        // Filter by category (partial match)
	Type      string        // Filter by type (MIME type or category pattern)
	MimeType  string        // Filter by exact MIME type
	Extension string        // Filter by file extension
	DateFrom  time.Time     // Filter by upload date (from)
	DateTo    time.Time     // Filter by upload date (to)
	Name      string        // Filter by name (partial match)
}

// ListResult contains paginated file listing results.
type ListResult struct {
	Files     []FileMetadata `json:"files"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo contains pagination metadata.
type PaginationInfo struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ListFiles returns a paginated, filtered, and sorted list of files.
func (m *Manager) ListFiles(options ListOptions) (*ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Default values
	if options.Page < 1 {
		options.Page = 1
	}
	if options.Limit < 1 {
		options.Limit = 50
	}
	if options.Limit > 1000 {
		options.Limit = 1000
	}
	if options.SortBy == "" {
		options.SortBy = "uploaded_at"
	}
	if options.Order == "" {
		options.Order = "desc"
	}

	// Convert all metadata to slice for filtering and sorting
	allFiles := make([]FileMetadata, 0, len(m.index.data))
	for _, meta := range m.index.data {
		allFiles = append(allFiles, meta)
	}

	// Apply filters
	filtered := make([]FileMetadata, 0)
	for _, meta := range allFiles {
		if matchesListFilters(meta, options) {
			filtered = append(filtered, meta)
		}
	}

	// Sort
	sortFiles(filtered, options.SortBy, options.Order)

	// Paginate
	total := len(filtered)
	totalPages := (total + options.Limit - 1) / options.Limit
	start := (options.Page - 1) * options.Limit
	end := start + options.Limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paginated []FileMetadata
	if start < end {
		paginated = filtered[start:end]
	} else {
		paginated = []FileMetadata{}
	}

	return &ListResult{
		Files: paginated,
		Pagination: PaginationInfo{
			Page:       options.Page,
			Limit:      options.Limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    options.Page < totalPages,
			HasPrev:    options.Page > 1,
		},
	}, nil
}

// matchesListFilters checks if a file matches the list filters.
func matchesListFilters(meta FileMetadata, options ListOptions) bool {
	// Category filter
	if options.Category != "" {
		categoryLower := strings.ToLower(meta.Category)
		searchLower := strings.ToLower(options.Category)
		if !strings.Contains(categoryLower, searchLower) {
			return false
		}
	}

	// Type filter (MIME type or category pattern)
	if options.Type != "" {
		typeLower := strings.ToLower(options.Type)
		mimeLower := strings.ToLower(meta.MimeType)
		categoryLower := strings.ToLower(meta.Category)

		matchesMime := strings.Contains(mimeLower, typeLower)
		matchesCategory := strings.Contains(categoryLower, typeLower)

		// Check common patterns
		matchesPattern := false
		if strings.HasPrefix(typeLower, "image/") && strings.HasPrefix(mimeLower, "image/") {
			matchesPattern = true
		} else if strings.HasPrefix(typeLower, "video/") && strings.HasPrefix(mimeLower, "video/") {
			matchesPattern = true
		} else if strings.HasPrefix(typeLower, "audio/") && strings.HasPrefix(mimeLower, "audio/") {
			matchesPattern = true
		} else if typeLower == "image" && strings.HasPrefix(mimeLower, "image/") {
			matchesPattern = true
		} else if typeLower == "video" && strings.HasPrefix(mimeLower, "video/") {
			matchesPattern = true
		} else if typeLower == "audio" && strings.HasPrefix(mimeLower, "audio/") {
			matchesPattern = true
		}

		if !matchesMime && !matchesCategory && !matchesPattern {
			return false
		}
	}

	// MIME type filter
	if options.MimeType != "" {
		if !strings.EqualFold(meta.MimeType, options.MimeType) {
			return false
		}
	}

	// Extension filter
	if options.Extension != "" {
		ext := strings.ToLower(strings.TrimPrefix(options.Extension, "."))
		fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(meta.OriginalName), "."))
		if fileExt != ext {
			return false
		}
	}

	// Date range filters
	if !options.DateFrom.IsZero() {
		if meta.UploadedAt.Before(options.DateFrom) {
			return false
		}
	}
	if !options.DateTo.IsZero() {
		if meta.UploadedAt.After(options.DateTo) {
			return false
		}
	}

	// Name filter
	if options.Name != "" {
		nameLower := strings.ToLower(meta.OriginalName)
		searchLower := strings.ToLower(options.Name)
		if !strings.Contains(nameLower, searchLower) {
			return false
		}
	}

	return true
}

// sortFiles sorts files by the specified field and order.
func sortFiles(files []FileMetadata, sortBy, order string) {
	sort.Slice(files, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(files[i].OriginalName) < strings.ToLower(files[j].OriginalName)
		case "uploaded_at":
			less = files[i].UploadedAt.Before(files[j].UploadedAt)
		case "size":
			less = files[i].Size < files[j].Size
		case "category":
			less = files[i].Category < files[j].Category
		case "mime_type":
			less = files[i].MimeType < files[j].MimeType
		default:
			// Default to uploaded_at
			less = files[i].UploadedAt.Before(files[j].UploadedAt)
		}

		if order == "desc" {
			return !less
		}
		return less
	})
}

// DirectoryEntry represents a directory or file in the browse view.
type DirectoryEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"` // "file" or "directory"
	Size      int64     `json:"size,omitempty"`
	FileCount int       `json:"file_count,omitempty"` // For directories
	Modified  time.Time `json:"modified,omitempty"`
}

// BrowseResult contains directory browsing results.
type BrowseResult struct {
	Path        string          `json:"path"`
	Entries     []DirectoryEntry `json:"entries"`
	ParentPath  string          `json:"parent_path,omitempty"`
	Breadcrumbs []Breadcrumb    `json:"breadcrumbs"`
}

// Breadcrumb represents a navigation breadcrumb.
type Breadcrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BrowseDirectory lists the contents of a directory path.
func (m *Manager) BrowseDirectory(path string) (*BrowseResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Normalize path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "storage"
	}

	// Security: validate path
	if err := validatePath(path); err != nil {
		return nil, err
	}

	// Build full path
	fullPath := filepath.Join(m.root, path)

	// Check if path exists
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if !info.IsDir() {
		return nil, errors.New("path is not a directory")
	}

	// Read directory contents
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	result := &BrowseResult{
		Path:        path,
		Entries:     make([]DirectoryEntry, 0),
		Breadcrumbs: buildBreadcrumbs(path),
	}

	// Set parent path
	if path != "storage" {
		parent := filepath.Dir(path)
		if parent == "." {
			parent = "storage"
		}
		result.ParentPath = parent
	}

	// Process entries
	for _, entry := range entries {
		// Skip hidden files and temp directory
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		dirEntry := DirectoryEntry{
			Name:     entry.Name(),
			Path:     filepath.ToSlash(filepath.Join(path, entry.Name())),
			Modified: entryInfo.ModTime(),
		}

		if entry.IsDir() {
			dirEntry.Type = "directory"
			// Count files in directory (approximate from metadata)
			dirEntry.FileCount = m.countFilesInCategory(dirEntry.Path)
		} else {
			dirEntry.Type = "file"
			dirEntry.Size = entryInfo.Size()
		}

		result.Entries = append(result.Entries, dirEntry)
	}

	// Sort: directories first, then by name
	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].Type != result.Entries[j].Type {
			return result.Entries[i].Type == "directory"
		}
		return strings.ToLower(result.Entries[i].Name) < strings.ToLower(result.Entries[j].Name)
	})

	return result, nil
}

// buildBreadcrumbs creates breadcrumb navigation.
func buildBreadcrumbs(path string) []Breadcrumb {
	parts := strings.Split(strings.TrimPrefix(path, "storage/"), "/")
	breadcrumbs := []Breadcrumb{
		{Name: "storage", Path: "storage"},
	}

	currentPath := "storage"
	for _, part := range parts {
		if part == "" {
			continue
		}
		currentPath = filepath.ToSlash(filepath.Join(currentPath, part))
		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Name: part,
			Path: currentPath,
		})
	}

	return breadcrumbs
}

// countFilesInCategory counts files in a category path (approximate from metadata).
func (m *Manager) countFilesInCategory(categoryPath string) int {
	count := 0
	categoryLower := strings.ToLower(categoryPath)
	for _, meta := range m.index.data {
		metaCategoryLower := strings.ToLower(meta.Category)
		if strings.HasPrefix(metaCategoryLower, categoryLower) || 
		   strings.HasPrefix(categoryLower, metaCategoryLower) {
			count++
		}
	}
	return count
}

// CategoryInfo contains information about a file category.
type CategoryInfo struct {
	Path      string `json:"path"`
	Count     int    `json:"count"`
	Size      int64  `json:"size"`
	MimeTypes []string `json:"mime_types,omitempty"`
}

// GetCategories returns a list of all categories with file counts and sizes.
func (m *Manager) GetCategories() ([]CategoryInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	categoryMap := make(map[string]*CategoryInfo)

	// Aggregate by category
	for _, meta := range m.index.data {
		category := meta.Category
		if category == "" {
			category = "uncategorized"
		}

		catInfo, exists := categoryMap[category]
		if !exists {
			catInfo = &CategoryInfo{
				Path:      category,
				MimeTypes: make([]string, 0),
			}
			categoryMap[category] = catInfo
		}

		catInfo.Count++
		catInfo.Size += meta.Size

		// Track unique MIME types
		mimeExists := false
		for _, mime := range catInfo.MimeTypes {
			if mime == meta.MimeType {
				mimeExists = true
				break
			}
		}
		if !mimeExists {
			catInfo.MimeTypes = append(catInfo.MimeTypes, meta.MimeType)
		}
	}

	// Convert to slice and sort
	categories := make([]CategoryInfo, 0, len(categoryMap))
	for _, catInfo := range categoryMap {
		categories = append(categories, *catInfo)
	}

	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Count != categories[j].Count {
			return categories[i].Count > categories[j].Count
		}
		return categories[i].Path < categories[j].Path
	})

	return categories, nil
}

// StorageStats contains aggregated storage statistics.
type StorageStats struct {
	TotalFiles    int                    `json:"total_files"`
	TotalSize     int64                  `json:"total_size"`
	Categories    map[string]CategoryInfo `json:"categories"`
	FileTypes     map[string]int          `json:"file_types"`     // MIME type -> count
	FileSizes     map[string]int64       `json:"file_sizes"`     // MIME type -> total size
	RecentUploads RecentUploadStats      `json:"recent_uploads"`
}

// RecentUploadStats contains statistics about recent uploads.
type RecentUploadStats struct {
	Last24Hours int `json:"last_24_hours"`
	Last7Days   int `json:"last_7_days"`
	Last30Days  int `json:"last_30_days"`
}

// GetStorageStats returns comprehensive storage statistics.
func (m *Manager) GetStorageStats() (*StorageStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := &StorageStats{
		Categories: make(map[string]CategoryInfo),
		FileTypes:  make(map[string]int),
		FileSizes:  make(map[string]int64),
	}

	now := time.Now().UTC()
	last24h := now.Add(-24 * time.Hour)
	last7d := now.Add(-7 * 24 * time.Hour)
	last30d := now.Add(-30 * 24 * time.Hour)

	// Process all files
	for _, meta := range m.index.data {
		stats.TotalFiles++
		stats.TotalSize += meta.Size

		// Track by MIME type
		stats.FileTypes[meta.MimeType]++
		stats.FileSizes[meta.MimeType] += meta.Size

		// Track by category
		category := meta.Category
		if category == "" {
			category = "uncategorized"
		}
		catInfo, exists := stats.Categories[category]
		if !exists {
			catInfo = CategoryInfo{
				Path: category,
			}
		}
		catInfo.Count++
		catInfo.Size += meta.Size
		stats.Categories[category] = catInfo

		// Track recent uploads
		if meta.UploadedAt.After(last24h) {
			stats.RecentUploads.Last24Hours++
		}
		if meta.UploadedAt.After(last7d) {
			stats.RecentUploads.Last7Days++
		}
		if meta.UploadedAt.After(last30d) {
			stats.RecentUploads.Last30Days++
		}
	}

	return stats, nil
}
