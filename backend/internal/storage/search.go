package storage

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SearchFilters contains all possible search filter criteria.
type SearchFilters struct {
	Name          string    // Partial match on original filename (case-insensitive)
	Extension     string    // Exact match on file extension (case-insensitive, with or without dot)
	Type          string    // Match on MIME type or category (supports partial match)
	DateFrom      time.Time // Files uploaded on or after this date
	DateTo        time.Time // Files uploaded on or before this date
	Category      string    // Match on category path (supports partial match)
	MimeType      string    // Exact match on MIME type
	ContentSearch string    // Search inside text files (case-insensitive, only for text MIME types)
}

// SearchFiles performs a filtered search across all file metadata.
// All filters are combined with AND logic (all must match).
// Empty/zero values are ignored (not applied as filters).
func (m *Manager) SearchFiles(filters SearchFilters) []FileMetadata {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]FileMetadata, 0)

	for _, meta := range m.index.data {
		if matchesFilters(meta, filters) {
			results = append(results, meta)
		}
	}

	return results
}

// matchesFilters checks if a file metadata matches all specified filters.
func matchesFilters(meta FileMetadata, filters SearchFilters) bool {
	// Filter by name (partial match, case-insensitive)
	if filters.Name != "" {
		nameLower := strings.ToLower(meta.OriginalName)
		searchLower := strings.ToLower(filters.Name)
		if !strings.Contains(nameLower, searchLower) {
			return false
		}
	}

	// Filter by extension (case-insensitive)
	if filters.Extension != "" {
		ext := strings.ToLower(strings.TrimPrefix(filters.Extension, "."))
		fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(meta.OriginalName), "."))
		if fileExt != ext {
			return false
		}
	}

	// Filter by type (MIME type or category, supports partial match)
	if filters.Type != "" {
		typeLower := strings.ToLower(filters.Type)
		mimeLower := strings.ToLower(meta.MimeType)
		categoryLower := strings.ToLower(meta.Category)
		
		// Check if type matches MIME type or category
		matchesMime := strings.Contains(mimeLower, typeLower)
		matchesCategory := strings.Contains(categoryLower, typeLower)
		
		// Also check for common type patterns
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

	// Filter by category (partial match, case-insensitive)
	if filters.Category != "" {
		categoryLower := strings.ToLower(meta.Category)
		searchLower := strings.ToLower(filters.Category)
		if !strings.Contains(categoryLower, searchLower) {
			return false
		}
	}

	// Filter by exact MIME type (case-insensitive)
	if filters.MimeType != "" {
		if !strings.EqualFold(meta.MimeType, filters.MimeType) {
			return false
		}
	}

	// Filter by date range
	if !filters.DateFrom.IsZero() {
		if meta.UploadedAt.Before(filters.DateFrom) {
			return false
		}
	}
	if !filters.DateTo.IsZero() {
		// DateTo is already normalized by the API layer (end of day for YYYY-MM-DD, exact for RFC3339)
		if meta.UploadedAt.After(filters.DateTo) {
			return false
		}
	}

	return true
}

// isTextMimeType checks if the MIME type is searchable text.
func isTextMimeType(mimeType string) bool {
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/x-sh",
		"application/x-python",
	}
	
	lowerMime := strings.ToLower(mimeType)
	for _, prefix := range textTypes {
		if strings.HasPrefix(lowerMime, prefix) {
			return true
		}
	}
	return false
}

// SearchFilesWithContent performs search including content search for text files.
func (m *Manager) SearchFilesWithContent(filters SearchFilters) []FileMetadata {
	// First get results matching metadata filters
	metadataResults := m.SearchFiles(filters)
	
	// If no content search, return metadata results
	if filters.ContentSearch == "" {
		return metadataResults
	}
	
	// Filter results by content search
	contentResults := make([]FileMetadata, 0)
	searchLower := strings.ToLower(filters.ContentSearch)
	
	for _, meta := range metadataResults {
		// Only search content in text files
		if !isTextMimeType(meta.MimeType) {
			continue
		}
		
		// Build full path
		fullPath := filepath.Join(m.root, meta.StoredPath)
		
		// Search file content
		if searchFileContent(fullPath, searchLower) {
			contentResults = append(contentResults, meta)
		}
	}
	
	return contentResults
}

// searchFileContent searches for a string in a file (case-insensitive).
// Returns true if the search string is found.
func searchFileContent(filePath, searchLower string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()
	
	// Limit file size for content search (max 10 MB)
	const maxSize = 10 * 1024 * 1024
	fileInfo, err := file.Stat()
	if err != nil || fileInfo.Size() > maxSize {
		return false
	}
	
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line size
	
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, searchLower) {
			return true
		}
	}
	
	return false
}
