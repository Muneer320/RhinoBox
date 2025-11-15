package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CollectionMetadata represents metadata for a collection type.
type CollectionMetadata struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	FileCount   int    `json:"file_count"`
	TotalSize   int64  `json:"total_size"`
	FormattedSize string `json:"formatted_size"`
}

// CollectionDefinitions maps collection types to their metadata.
var collectionDefinitions = map[string]struct {
	Name        string
	Description string
	Icon        string
}{
	"images": {
		Name:        "Images",
		Description: "Photos, graphics, and visual media files.",
		Icon:        "https://images.unsplash.com/photo-1516035069371-29a1b244cc32?auto=format&fit=crop&w=600&q=80",
	},
	"videos": {
		Name:        "Videos",
		Description: "Video recordings, clips, and multimedia content.",
		Icon:        "https://images.unsplash.com/photo-1533750516457-a7f992034fec?auto=format&fit=crop&w=600&q=80",
	},
	"audio": {
		Name:        "Audio",
		Description: "Music files, recordings, and sound clips.",
		Icon:        "https://images.unsplash.com/photo-1493225457124-a3eb161ffa5f?auto=format&fit=crop&w=600&q=80",
	},
	"documents": {
		Name:        "Documents",
		Description: "Text documents, PDFs, and written content.",
		Icon:        "https://images.unsplash.com/photo-1455390582262-044cdead277a?auto=format&fit=crop&w=600&q=80",
	},
	"spreadsheets": {
		Name:        "Spreadsheets",
		Description: "Data tables, calculations, and structured data.",
		Icon:        "https://images.unsplash.com/photo-1551288049-bebda4e38f71?auto=format&fit=crop&w=600&q=80",
	},
	"presentations": {
		Name:        "Presentations",
		Description: "Slides, decks, and presentation materials.",
		Icon:        "https://images.unsplash.com/photo-1554224155-6726b3ff858f?auto=format&fit=crop&w=600&q=80",
	},
	"archives": {
		Name:        "Archives",
		Description: "Compressed files, ZIPs, and archived content.",
		Icon:        "https://images.unsplash.com/photo-1586281380349-632531db7ed4?auto=format&fit=crop&w=600&q=80",
	},
	"code": {
		Name:        "Code",
		Description: "Source code files and programming scripts.",
		Icon:        "https://images.unsplash.com/photo-1555066931-4365d14bab8c?auto=format&fit=crop&w=600&q=80",
	},
	"other": {
		Name:        "Other",
		Description: "Miscellaneous files and unclassified content.",
		Icon:        "https://images.unsplash.com/photo-1558494949-ef010cbdcc31?auto=format&fit=crop&w=600&q=80",
	},
}

// GetCollections scans storage and returns metadata for all available collection types.
func (m *Manager) GetCollections() ([]CollectionMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	collections := make([]CollectionMetadata, 0, len(collectionDefinitions))
	
	// Scan each collection type
	for collectionType, def := range collectionDefinitions {
		collectionPath := filepath.Join(m.storageRoot, collectionType)
		
		// Count files and calculate total size
		fileCount, totalSize, err := m.scanCollection(collectionPath)
		if err != nil {
			// If directory doesn't exist, skip it (collection has no files)
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("scan collection %s: %w", collectionType, err)
		}

		// Only include collections that have files
		if fileCount > 0 {
			collections = append(collections, CollectionMetadata{
				Type:          collectionType,
				Name:          def.Name,
				Description:   def.Description,
				Icon:          def.Icon,
				FileCount:     fileCount,
				TotalSize:     totalSize,
				FormattedSize: formatSize(totalSize),
			})
		}
	}

	// Also check metadata index for collections that might not have files on disk yet
	// but have metadata entries
	indexedCollections := make(map[string]bool)
	for _, meta := range m.index.data {
		// Extract collection type from stored path (e.g., "media/images/jpg/..." -> "images")
		// StoredPath uses forward slashes as separators
		parts := strings.Split(meta.StoredPath, "/")
		if len(parts) > 1 && parts[0] == "media" {
			collectionType := parts[1]
			if _, exists := collectionDefinitions[collectionType]; exists {
				indexedCollections[collectionType] = true
			}
		}
	}

	// Add collections that exist in index but not yet scanned
	for collectionType := range indexedCollections {
		found := false
		for _, coll := range collections {
			if coll.Type == collectionType {
				found = true
				break
			}
		}
		if !found {
			def := collectionDefinitions[collectionType]
			// Count files from metadata index
			fileCount := m.countFilesInCollection(collectionType)
			if fileCount > 0 {
				totalSize := m.calculateCollectionSize(collectionType)
				collections = append(collections, CollectionMetadata{
					Type:          collectionType,
					Name:          def.Name,
					Description:   def.Description,
					Icon:          def.Icon,
					FileCount:     fileCount,
					TotalSize:     totalSize,
					FormattedSize: formatSize(totalSize),
				})
			}
		}
	}

	return collections, nil
}

// scanCollection recursively scans a collection directory and counts files and total size.
func (m *Manager) scanCollection(path string) (fileCount int, totalSize int64, err error) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			fileCount++
			totalSize += info.Size()
		}()

		return nil
	})

	wg.Wait()
	return fileCount, totalSize, err
}

// countFilesInCollection counts files in a collection from the metadata index.
func (m *Manager) countFilesInCollection(collectionType string) int {
	count := 0
	for _, meta := range m.index.data {
		parts := strings.Split(meta.StoredPath, "/")
		if len(parts) > 1 && parts[0] == "media" && parts[1] == collectionType {
			count++
		}
	}
	return count
}

// calculateCollectionSize calculates total size of files in a collection from metadata index.
func (m *Manager) calculateCollectionSize(collectionType string) int64 {
	var totalSize int64
	for _, meta := range m.index.data {
		parts := strings.Split(meta.StoredPath, "/")
		if len(parts) > 1 && parts[0] == "media" && parts[1] == collectionType {
			totalSize += meta.Size
		}
	}
	return totalSize
}

// formatSize formats bytes into human-readable format.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

