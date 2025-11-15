package services

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/cache"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// setupTestStorage creates a temporary storage manager for testing.
func setupTestStorage(t *testing.T) (*storage.Manager, string) {
	tmpDir := t.TempDir()
	store, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}
	return store, tmpDir
}

// setupTestCache creates a temporary cache for testing.
func setupTestCache(t *testing.T) *cache.Cache {
	tmpDir := t.TempDir()
	cacheConfig := cache.DefaultConfig()
	cacheConfig.L3Path = filepath.Join(tmpDir, "cache")
	cacheInstance, err := cache.New(cacheConfig)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	return cacheInstance
}

// setupTestService creates a collection service for testing.
func setupTestService(t *testing.T) (*CollectionService, *storage.Manager) {
	store, _ := setupTestStorage(t)
	cacheInstance := setupTestCache(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewCollectionService(store, cacheInstance, logger)
	return service, store
}

// addTestFile adds a test file to storage and returns its metadata.
func addTestFile(t *testing.T, store *storage.Manager, filename, mimeType, category string, size int64) storage.FileMetadata {
	// Create a temporary file with content
	tmpFile := filepath.Join(t.TempDir(), filename)
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Read the file and store it
	file, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	result, err := store.StoreFile(storage.StoreRequest{
		Reader:       file,
		Filename:     filename,
		MimeType:     mimeType,
		Size:         fileInfo.Size(),
		Metadata:     map[string]string{},
		CategoryHint: category,
	})
	if err != nil {
		t.Fatalf("failed to store test file: %v", err)
	}

	return result.Metadata
}

func TestNewCollectionService(t *testing.T) {
	service, _ := setupTestService(t)
	if service == nil {
		t.Fatal("expected service to be created")
	}
	if service.storage == nil {
		t.Error("expected storage to be set")
	}
	if service.cache == nil {
		t.Error("expected cache to be set")
	}
}

func TestDiscoverCollections_EmptyStorage(t *testing.T) {
	service, _ := setupTestService(t)

	collections, err := service.DiscoverCollections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(collections) == 0 {
		t.Error("expected at least some collection types to be returned")
	}

	// Check that all collection types are present
	typeMap := make(map[string]bool)
	for _, c := range collections {
		typeMap[c.Type] = true
	}

	for _, ct := range collectionTypes {
		if !typeMap[ct.Type] {
			t.Errorf("expected collection type %s to be present", ct.Type)
		}
	}
}

func TestDiscoverCollections_WithFiles(t *testing.T) {
	service, store := setupTestService(t)

	// Add test files of different types
	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)
	addTestFile(t, store, "test.mp4", "video/mp4", "videos", 2048)
	addTestFile(t, store, "test.mp3", "audio/mpeg", "audio", 512)
	addTestFile(t, store, "test.pdf", "application/pdf", "documents", 1536)

	collections, err := service.DiscoverCollections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the collections we added files to
	imagesFound := false
	videosFound := false
	audioFound := false
	documentsFound := false

	for _, c := range collections {
		switch c.Type {
		case "images":
			imagesFound = true
			if c.Stats.FileCount != 1 {
				t.Errorf("expected 1 image file, got %d", c.Stats.FileCount)
			}
			if c.Stats.StorageUsed != 1024 {
				t.Errorf("expected 1024 bytes for images, got %d", c.Stats.StorageUsed)
			}
		case "videos":
			videosFound = true
			if c.Stats.FileCount != 1 {
				t.Errorf("expected 1 video file, got %d", c.Stats.FileCount)
			}
			if c.Stats.StorageUsed != 2048 {
				t.Errorf("expected 2048 bytes for videos, got %d", c.Stats.StorageUsed)
			}
		case "audio":
			audioFound = true
			if c.Stats.FileCount != 1 {
				t.Errorf("expected 1 audio file, got %d", c.Stats.FileCount)
			}
			if c.Stats.StorageUsed != 512 {
				t.Errorf("expected 512 bytes for audio, got %d", c.Stats.StorageUsed)
			}
		case "documents":
			documentsFound = true
			if c.Stats.FileCount != 1 {
				t.Errorf("expected 1 document file, got %d", c.Stats.FileCount)
			}
			if c.Stats.StorageUsed != 1536 {
				t.Errorf("expected 1536 bytes for documents, got %d", c.Stats.StorageUsed)
			}
		}
	}

	if !imagesFound {
		t.Error("expected images collection to be found")
	}
	if !videosFound {
		t.Error("expected videos collection to be found")
	}
	if !audioFound {
		t.Error("expected audio collection to be found")
	}
	if !documentsFound {
		t.Error("expected documents collection to be found")
	}
}

func TestGetCollectionStats_ValidType(t *testing.T) {
	service, store := setupTestService(t)

	// Add test files
	addTestFile(t, store, "test1.jpg", "image/jpeg", "images", 1024)
	addTestFile(t, store, "test2.jpg", "image/jpeg", "images", 2048)

	stats, err := service.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Type != "images" {
		t.Errorf("expected type 'images', got '%s'", stats.Type)
	}
	if stats.DisplayName != "Images" {
		t.Errorf("expected display name 'Images', got '%s'", stats.DisplayName)
	}
	if stats.Stats.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", stats.Stats.FileCount)
	}
	if stats.Stats.StorageUsed != 3072 {
		t.Errorf("expected 3072 bytes, got %d", stats.Stats.StorageUsed)
	}
}

func TestGetCollectionStats_InvalidType(t *testing.T) {
	service, _ := setupTestService(t)

	_, err := service.GetCollectionStats("invalid_type")
	if err == nil {
		t.Error("expected error for invalid collection type")
	}
	if !contains(err.Error(), "invalid collection type") {
		t.Errorf("expected error message about invalid type, got: %v", err)
	}
}

func TestGetCollectionStats_Caching(t *testing.T) {
	service, store := setupTestService(t)

	// Add test file
	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)

	// First call - should calculate
	stats1, err := service.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call - should use cache
	stats2, err := service.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Results should be the same
	if stats1.Stats.FileCount != stats2.Stats.FileCount {
		t.Error("cached stats should match original stats")
	}
	if stats1.Stats.StorageUsed != stats2.Stats.StorageUsed {
		t.Error("cached stats should match original stats")
	}
}

func TestGetAllCollections(t *testing.T) {
	service, store := setupTestService(t)

	// Add test files
	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)
	addTestFile(t, store, "test.mp4", "video/mp4", "videos", 2048)

	response, err := service.GetAllCollections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response == nil {
		t.Fatal("expected response to be non-nil")
	}
	if response.Total == 0 {
		t.Error("expected at least one collection")
	}
	if len(response.Collections) != response.Total {
		t.Errorf("expected collections length to match total, got %d != %d", len(response.Collections), response.Total)
	}
	if response.GeneratedAt == "" {
		t.Error("expected generated_at to be set")
	}
}

func TestGetAllCollections_Caching(t *testing.T) {
	service, store := setupTestService(t)

	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)

	// First call
	response1, err := service.GetAllCollections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call - should use cache
	response2, err := service.GetAllCollections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check cache key
	cacheKey := cacheKeyAll
	cached, ok := service.cache.Get(cacheKey)
	if !ok {
		t.Error("expected response to be cached")
	}

	var cachedResponse CollectionsResponse
	if err := json.Unmarshal(cached, &cachedResponse); err != nil {
		t.Fatalf("failed to unmarshal cached response: %v", err)
	}

	if cachedResponse.Total != response1.Total {
		t.Error("cached response should match original")
	}
	if len(cachedResponse.Collections) != len(response1.Collections) {
		t.Error("cached response should match original")
	}

	// Results should be the same
	if response1.Total != response2.Total {
		t.Error("cached response should match original")
	}
}

func TestExtractCollectionType(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		name     string
		metadata storage.FileMetadata
		expected string
	}{
		{
			name: "image file",
			metadata: storage.FileMetadata{
				Category:   "images/jpg",
				StoredPath: "storage/images/jpg/test.jpg",
			},
			expected: "images",
		},
		{
			name: "video file",
			metadata: storage.FileMetadata{
				Category:   "videos/mp4",
				StoredPath: "storage/videos/mp4/test.mp4",
			},
			expected: "videos",
		},
		{
			name: "json file",
			metadata: storage.FileMetadata{
				Category:   "",
				StoredPath: "json/sql/namespace/batch.ndjson",
			},
			expected: "json",
		},
		{
			name: "unknown category",
			metadata: storage.FileMetadata{
				Category:   "unknown/type",
				StoredPath: "storage/unknown/type/test.file",
			},
			expected: "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.extractCollectionType(tt.metadata)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCalculateStats(t *testing.T) {
	service, _ := setupTestService(t)

	metadata := []storage.FileMetadata{
		{Size: 1024, Category: "images/jpg"},
		{Size: 2048, Category: "images/jpg"},
		{Size: 512, Category: "images/jpg"},
	}

	stats := service.calculateStats("images", metadata)

	if stats.Type != "images" {
		t.Errorf("expected type 'images', got '%s'", stats.Type)
	}
	if stats.FileCount != 3 {
		t.Errorf("expected 3 files, got %d", stats.FileCount)
	}
	if stats.StorageUsed != 3584 {
		t.Errorf("expected 3584 bytes, got %d", stats.StorageUsed)
	}
	if stats.LastUpdated == "" {
		t.Error("expected last_updated to be set")
	}
}

func TestInvalidateCache(t *testing.T) {
	service, store := setupTestService(t)

	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)

	// Get stats to populate cache
	_, err := service.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache is populated
	cached := service.getCachedStats("images")
	if cached == nil {
		t.Error("expected cache to be populated")
	}

	// Invalidate cache
	service.InvalidateCache("images")

	// Verify cache is cleared
	cached = service.getCachedStats("images")
	if cached != nil {
		t.Error("expected cache to be cleared")
	}
}

func TestInvalidateCache_All(t *testing.T) {
	service, store := setupTestService(t)

	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)
	addTestFile(t, store, "test.mp4", "video/mp4", "videos", 2048)

	// Populate cache
	_, _ = service.GetCollectionStats("images")
	_, _ = service.GetCollectionStats("videos")
	_, _ = service.GetAllCollections()

	// Invalidate all
	service.InvalidateCache("")

	// Verify all caches are cleared
	if cached := service.getCachedStats("images"); cached != nil {
		t.Error("expected images cache to be cleared")
	}
	if cached := service.getCachedStats("videos"); cached != nil {
		t.Error("expected videos cache to be cleared")
	}

	// Check that cache key is deleted
	_, ok := service.cache.Get(cacheKeyAll)
	if ok {
		t.Error("expected all collections cache to be cleared")
	}
}

func TestIsValidCollectionType(t *testing.T) {
	service, _ := setupTestService(t)

	validTypes := []string{"images", "videos", "audio", "documents", "json", "other"}
	invalidTypes := []string{"invalid", "unknown", "test"}

	for _, typ := range validTypes {
		if !service.isValidCollectionType(typ) {
			t.Errorf("expected %s to be valid", typ)
		}
	}

	for _, typ := range invalidTypes {
		if service.isValidCollectionType(typ) {
			t.Errorf("expected %s to be invalid", typ)
		}
	}
}

func TestGetDisplayName(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		input    string
		expected string
	}{
		{"images", "Images"},
		{"videos", "Videos"},
		{"audio", "Audio"},
		{"json", "JSON Documents"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := service.getDisplayName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCachedStats_Expiration(t *testing.T) {
	service, store := setupTestService(t)

	// Set a short TTL for testing
	service.cacheTTL = 100 * time.Millisecond

	addTestFile(t, store, "test.jpg", "image/jpeg", "images", 1024)

	// Get stats to populate cache
	_, err := service.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache is populated
	cached := service.getCachedStats("images")
	if cached == nil {
		t.Error("expected cache to be populated")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Verify cache is expired
	cached = service.getCachedStats("images")
	if cached != nil {
		t.Error("expected cache to be expired")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

