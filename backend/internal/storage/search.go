package storage

import (
	"path/filepath"
	"strings"
	"time"
)

// SearchFilters contains all possible search filter criteria.
type SearchFilters struct {
	Name       string    // Partial match on original filename (case-insensitive)
	Extension  string    // Exact match on file extension (case-insensitive, with or without dot)
	Type       string    // Match on MIME type or category (supports partial match)
	DateFrom   time.Time // Files uploaded on or after this date
	DateTo     time.Time // Files uploaded on or before this date
	Category   string    // Match on category path (supports partial match)
	MimeType   string    // Exact match on MIME type
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
