package storage

import (
	"strings"
)

// GetFilesByType returns all files that match a collection type.
func (m *Manager) GetFilesByType(collectionType string) ([]FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []FileMetadata
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
				// Create a copy to avoid returning references to internal data
				metaCopy := meta
				results = append(results, metaCopy)
			}
		}
	}

	return results, nil
}

