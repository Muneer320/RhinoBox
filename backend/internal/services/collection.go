package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Muneer320/RhinoBox/internal/cache"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

const (
	// Cache key prefix for collection statistics
	cacheKeyPrefix = "collection:stats:"
	// Cache key for all collections
	cacheKeyAll = "collections:all"
)

// CollectionService provides collection discovery and statistics aggregation.
type CollectionService struct {
	storage    *storage.Manager
	cache     *cache.Cache
	logger    *slog.Logger
	cacheTTL  time.Duration
	statsMu   sync.RWMutex
	statsCache map[string]*CachedStats
}

// CachedStats holds cached collection statistics with expiration.
type CachedStats struct {
	Stats      CollectionStats
	ExpiresAt  time.Time
}

// CollectionStats represents statistics for a collection.
type CollectionStats struct {
	Type         string `json:"type"`
	FileCount    int    `json:"file_count"`
	StorageUsed  int64  `json:"storage_used"`
	LastUpdated  string `json:"last_updated"`
}

// CollectionDTO represents a collection for frontend responses.
type CollectionDTO struct {
	Type        string          `json:"type"`
	DisplayName string          `json:"display_name"`
	Stats       CollectionStats `json:"stats"`
}

// CollectionsResponse represents the response for listing all collections.
type CollectionsResponse struct {
	Collections []CollectionDTO `json:"collections"`
	Total       int             `json:"total"`
	GeneratedAt string          `json:"generated_at"`
}

// CollectionStatsResponse represents the response for collection statistics.
type CollectionStatsResponse struct {
	Type        string          `json:"type"`
	DisplayName string          `json:"display_name"`
	Stats       CollectionStats `json:"stats"`
}

// CollectionType represents a collection type with its display name.
type CollectionType struct {
	Type        string
	DisplayName string
}

// Valid collection types based on storage layout.
var collectionTypes = []CollectionType{
	{"images", "Images"},
	{"videos", "Videos"},
	{"audio", "Audio"},
	{"documents", "Documents"},
	{"spreadsheets", "Spreadsheets"},
	{"presentations", "Presentations"},
	{"archives", "Archives"},
	{"code", "Code"},
	{"other", "Other"},
	{"json", "JSON Documents"},
}

// NewCollectionService creates a new collection service.
func NewCollectionService(storage *storage.Manager, cache *cache.Cache, logger *slog.Logger) *CollectionService {
	return &CollectionService{
		storage:    storage,
		cache:      cache,
		logger:     logger,
		cacheTTL:   5 * time.Minute,
		statsCache: make(map[string]*CachedStats),
	}
}

// DiscoverCollections discovers all collection types from storage metadata.
func (s *CollectionService) DiscoverCollections() ([]CollectionDTO, error) {
	collections := make([]CollectionDTO, 0, len(collectionTypes))
	
	// Get all metadata from storage
	allMetadata := s.getAllMetadata()
	
	// Group by collection type
	typeMap := make(map[string][]storage.FileMetadata)
	for _, meta := range allMetadata {
		collectionType := s.extractCollectionType(meta)
		typeMap[collectionType] = append(typeMap[collectionType], meta)
	}
	
	// Build collection DTOs
	for _, ct := range collectionTypes {
		metadata := typeMap[ct.Type]
		stats := s.calculateStats(ct.Type, metadata)
		
		collections = append(collections, CollectionDTO{
			Type:        ct.Type,
			DisplayName: ct.DisplayName,
			Stats:       stats,
		})
	}
	
	return collections, nil
}

// GetCollectionStats returns statistics for a specific collection type.
func (s *CollectionService) GetCollectionStats(collectionType string) (*CollectionStatsResponse, error) {
	// Check cache first
	if cached := s.getCachedStats(collectionType); cached != nil {
		displayName := s.getDisplayName(collectionType)
		return &CollectionStatsResponse{
			Type:        collectionType,
			DisplayName: displayName,
			Stats:       *cached,
		}, nil
	}
	
	// Validate collection type
	if !s.isValidCollectionType(collectionType) {
		return nil, fmt.Errorf("invalid collection type: %s", collectionType)
	}
	
	// Get all metadata
	allMetadata := s.getAllMetadata()
	
	// Filter by collection type
	filtered := make([]storage.FileMetadata, 0)
	for _, meta := range allMetadata {
		if s.extractCollectionType(meta) == collectionType {
			filtered = append(filtered, meta)
		}
	}
	
	// Calculate statistics
	stats := s.calculateStats(collectionType, filtered)
	
	// Cache the result
	s.setCachedStats(collectionType, stats)
	
	displayName := s.getDisplayName(collectionType)
	return &CollectionStatsResponse{
		Type:        collectionType,
		DisplayName: displayName,
		Stats:       stats,
	}, nil
}

// GetAllCollections returns all collections with their statistics.
func (s *CollectionService) GetAllCollections() (*CollectionsResponse, error) {
	// Try cache first
	cacheKey := "collections:all"
	if cached, ok := s.cache.Get(cacheKey); ok {
		var response CollectionsResponse
		if err := json.Unmarshal(cached, &response); err == nil {
			return &response, nil
		}
	}
	
	collections, err := s.DiscoverCollections()
	if err != nil {
		return nil, fmt.Errorf("discover collections: %w", err)
	}
	
	response := &CollectionsResponse{
		Collections: collections,
		Total:       len(collections),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	
	// Cache the response
	if data, err := json.Marshal(response); err == nil {
		_ = s.cache.Set(cacheKey, data)
	}
	
	return response, nil
}

// extractCollectionType extracts the collection type from file metadata.
func (s *CollectionService) extractCollectionType(meta storage.FileMetadata) string {
	// Check if it's a JSON collection
	if strings.HasPrefix(meta.StoredPath, "json/") {
		return "json"
	}
	
	// Extract from category (format: "category/subcategory" or just "category")
	category := meta.Category
	if category == "" {
		return "other"
	}
	
	// Split category by "/" and take the first part
	parts := strings.Split(category, "/")
	if len(parts) > 0 {
		firstPart := parts[0]
		// Validate it's a known collection type
		if s.isValidCollectionType(firstPart) {
			return firstPart
		}
	}
	
	return "other"
}

// calculateStats calculates statistics for a collection.
func (s *CollectionService) calculateStats(collectionType string, metadata []storage.FileMetadata) CollectionStats {
	var totalSize int64
	fileCount := len(metadata)
	
	for _, meta := range metadata {
		totalSize += meta.Size
	}
	
	return CollectionStats{
		Type:        collectionType,
		FileCount:   fileCount,
		StorageUsed: totalSize,
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}
}

// getAllMetadata retrieves all file metadata from storage.
func (s *CollectionService) getAllMetadata() []storage.FileMetadata {
	// Access the storage manager's metadata index
	// We need to add a method to get all metadata
	// For now, we'll use reflection or add a method to storage.Manager
	
	// Since we can't directly access the private index, we'll need to add a method
	// to storage.Manager to get all metadata. For now, let's create a workaround.
	
	// Actually, we should add a GetAllMetadata method to storage.Manager
	// But for now, let's use a different approach - we can iterate through
	// the storage directory structure
	
	// For MVP, let's add a method to storage to get all metadata
	return s.storage.GetAllMetadata()
}

// isValidCollectionType checks if a collection type is valid.
func (s *CollectionService) isValidCollectionType(collectionType string) bool {
	for _, ct := range collectionTypes {
		if ct.Type == collectionType {
			return true
		}
	}
	return false
}

// getDisplayName returns the display name for a collection type.
func (s *CollectionService) getDisplayName(collectionType string) string {
	for _, ct := range collectionTypes {
		if ct.Type == collectionType {
			return ct.DisplayName
		}
	}
	// Capitalize first letter
	if len(collectionType) == 0 {
		return collectionType
	}
	runes := []rune(collectionType)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// getCachedStats retrieves cached statistics if not expired.
func (s *CollectionService) getCachedStats(collectionType string) *CollectionStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	
	cached, ok := s.statsCache[collectionType]
	if !ok {
		return nil
	}
	
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}
	
	return &cached.Stats
}

// setCachedStats stores statistics in cache.
func (s *CollectionService) setCachedStats(collectionType string, stats CollectionStats) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	
	s.statsCache[collectionType] = &CachedStats{
		Stats:     stats,
		ExpiresAt: time.Now().Add(s.cacheTTL),
	}
}

// InvalidateCache invalidates the cache for a collection type or all collections.
func (s *CollectionService) InvalidateCache(collectionType string) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	
	if collectionType == "" {
		// Invalidate all
		s.statsCache = make(map[string]*CachedStats)
		_ = s.cache.Delete("collections:all")
	} else {
		delete(s.statsCache, collectionType)
	}
}

