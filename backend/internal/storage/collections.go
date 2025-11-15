package storage

import (
	"fmt"
	"strings"
)

// CollectionInfo represents metadata about a collection type.
type CollectionInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
}

// CollectionStats represents statistics for a collection.
type CollectionStats struct {
	Type        string `json:"type"`
	FileCount   int    `json:"file_count"`
	StorageUsed int64  `json:"storage_used"`
	StorageUsedFormatted string `json:"storage_used_formatted"`
}

// Predefined collection types with metadata.
var collectionTypes = []CollectionInfo{
	{Type: "images", Name: "Images", Description: "Photos, graphics, and visual media files."},
	{Type: "videos", Name: "Videos", Description: "Video recordings, clips, and multimedia content."},
	{Type: "audio", Name: "Audio", Description: "Music files, recordings, and sound clips."},
	{Type: "documents", Name: "Documents", Description: "Text documents, PDFs, and written content."},
	{Type: "spreadsheets", Name: "Spreadsheets", Description: "Data tables, calculations, and structured data."},
	{Type: "presentations", Name: "Presentations", Description: "Slides, decks, and presentation materials."},
	{Type: "archives", Name: "Archives", Description: "Compressed files, ZIPs, and archived content."},
	{Type: "other", Name: "Other", Description: "Miscellaneous files and unclassified content."},
}

// GetCollections returns all available collection types with metadata.
func (m *Manager) GetCollections() []CollectionInfo {
	return collectionTypes
}

// GetCollectionStats returns statistics for a specific collection type.
func (m *Manager) GetCollectionStats(collectionType string) (*CollectionStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fileCount int
	var storageUsed int64

	// Normalize collection type for matching
	normalizedType := strings.ToLower(strings.TrimSpace(collectionType))

	// Iterate through all metadata entries
	for _, meta := range m.index.data {
		// Extract the top-level category from the category path
		// Category format is like "images/jpg" or "documents/pdf"
		categoryParts := strings.Split(meta.Category, "/")
		if len(categoryParts) > 0 {
			topLevelCategory := strings.ToLower(categoryParts[0])
			
			// Map some categories to collection types
			if matchesCollectionType(topLevelCategory, normalizedType) {
				fileCount++
				storageUsed += meta.Size
			}
		}
	}

	return &CollectionStats{
		Type:                collectionType,
		FileCount:           fileCount,
		StorageUsed:         storageUsed,
		StorageUsedFormatted: formatBytes(storageUsed),
	}, nil
}

// matchesCollectionType checks if a category matches a collection type.
func matchesCollectionType(category, collectionType string) bool {
	// Direct match
	if category == collectionType {
		return true
	}

	// Handle special mappings
	switch collectionType {
	case "images":
		return category == "images"
	case "videos":
		return category == "videos"
	case "audio":
		return category == "audio"
	case "documents":
		return category == "documents"
	case "spreadsheets":
		return category == "spreadsheets"
	case "presentations":
		return category == "presentations"
	case "archives":
		return category == "archives"
	case "other":
		// Other includes anything not in the main categories
		mainCategories := []string{"images", "videos", "audio", "documents", "spreadsheets", "presentations", "archives"}
		for _, mainCat := range mainCategories {
			if category == mainCat {
				return false
			}
		}
		return true
	}

	return false
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

