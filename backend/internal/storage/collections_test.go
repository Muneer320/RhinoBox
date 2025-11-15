package storage

import (
	"bytes"
	"testing"
)

func TestGetCollections(t *testing.T) {
	manager := newTestManager(t)
	defer cleanupManager(t, manager)

	collections := manager.GetCollections()

	if len(collections) == 0 {
		t.Fatalf("expected at least one collection, got %d", len(collections))
	}

	// Verify expected collection types
	expectedTypes := map[string]bool{
		"images":       false,
		"videos":       false,
		"audio":        false,
		"documents":    false,
		"spreadsheets": false,
		"presentations": false,
		"archives":     false,
		"other":        false,
	}

	for _, collection := range collections {
		if _, exists := expectedTypes[collection.Type]; exists {
			expectedTypes[collection.Type] = true
		}
		if collection.Name == "" {
			t.Errorf("collection %s has empty name", collection.Type)
		}
		if collection.Description == "" {
			t.Errorf("collection %s has empty description", collection.Type)
		}
	}

	for typeName, found := range expectedTypes {
		if !found {
			t.Errorf("expected collection type %s not found", typeName)
		}
	}
}

func TestGetCollectionStatsEmpty(t *testing.T) {
	manager := newTestManager(t)
	defer cleanupManager(t, manager)

	stats, err := manager.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Type != "images" {
		t.Errorf("expected type 'images', got %s", stats.Type)
	}
	if stats.FileCount != 0 {
		t.Errorf("expected 0 files, got %d", stats.FileCount)
	}
	if stats.StorageUsed != 0 {
		t.Errorf("expected 0 storage, got %d", stats.StorageUsed)
	}
	if stats.StorageUsedFormatted != "0 B" {
		t.Errorf("expected '0 B', got %s", stats.StorageUsedFormatted)
	}
}

func TestGetCollectionStatsWithFiles(t *testing.T) {
	manager := newTestManager(t)
	defer cleanupManager(t, manager)

	// Store a test image file
	imageData := []byte("fake image data")
	req := StoreRequest{
		Reader:   bytes.NewReader(imageData),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(imageData)),
	}

	result, err := manager.StoreFile(req)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	if result == nil {
		t.Fatalf("expected store result, got nil")
	}

	// Get stats for images collection
	stats, err := manager.GetCollectionStats("images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.FileCount < 1 {
		t.Errorf("expected at least 1 file, got %d", stats.FileCount)
	}
	if stats.StorageUsed <= 0 {
		t.Errorf("expected storage > 0, got %d", stats.StorageUsed)
	}
	if stats.StorageUsedFormatted == "" {
		t.Errorf("expected formatted storage, got empty string")
	}
}

func TestGetCollectionStatsMultipleTypes(t *testing.T) {
	manager := newTestManager(t)
	defer cleanupManager(t, manager)

	// Store files of different types
	files := []struct {
		filename string
		mimeType string
		data     []byte
		expectedCollection string
	}{
		{"test.jpg", "image/jpeg", []byte("image data"), "images"},
		{"test.mp4", "video/mp4", []byte("video data"), "videos"},
		{"test.mp3", "audio/mpeg", []byte("audio data"), "audio"},
	}

	for _, file := range files {
		req := StoreRequest{
			Reader:   bytes.NewReader(file.data),
			Filename: file.filename,
			MimeType: file.mimeType,
			Size:     int64(len(file.data)),
		}

		_, err := manager.StoreFile(req)
		if err != nil {
			t.Fatalf("failed to store %s: %v", file.filename, err)
		}
	}

	// Check stats for each collection
	for _, file := range files {
		stats, err := manager.GetCollectionStats(file.expectedCollection)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", file.expectedCollection, err)
		}

		if stats.FileCount < 1 {
			t.Errorf("expected at least 1 file for %s, got %d", file.expectedCollection, stats.FileCount)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, test := range tests {
		result := formatBytes(test.bytes)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	return manager
}

func cleanupManager(t *testing.T, manager *Manager) {
	t.Helper()
	// Cleanup is handled by t.TempDir()
}

