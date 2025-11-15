package storage

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStatisticsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	stats, err := manager.GetStatistics()
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	if stats.TotalFiles != 0 {
		t.Errorf("expected TotalFiles=0, got %d", stats.TotalFiles)
	}
	if stats.StorageUsed != 0 {
		t.Errorf("expected StorageUsed=0, got %d", stats.StorageUsed)
	}
	if stats.CollectionCount != 0 {
		t.Errorf("expected CollectionCount=0, got %d", stats.CollectionCount)
	}
	if stats.StorageUsedFormatted != "0 B" {
		t.Errorf("expected StorageUsedFormatted='0 B', got %s", stats.StorageUsedFormatted)
	}
}

func TestGetStatisticsWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Add some test files to metadata index
	testFiles := []FileMetadata{
		{
			Hash:         "hash1",
			OriginalName: "test1.jpg",
			StoredPath:   "storage/images/jpg/test1.jpg",
			Category:     "images/jpg",
			MimeType:     "image/jpeg",
			Size:         1024,
			UploadedAt:   time.Now().UTC(),
			Metadata:     nil,
		},
		{
			Hash:         "hash2",
			OriginalName: "test2.png",
			StoredPath:   "storage/images/png/test2.png",
			Category:     "images/png",
			MimeType:     "image/png",
			Size:         2048,
			UploadedAt:   time.Now().UTC(),
			Metadata:     nil,
		},
		{
			Hash:         "hash3",
			OriginalName: "doc.pdf",
			StoredPath:   "storage/documents/pdf/doc.pdf",
			Category:     "documents/pdf",
			MimeType:     "application/pdf",
			Size:         4096,
			UploadedAt:   time.Now().UTC(),
			Metadata:     nil,
		},
	}

	// Add files to index
	for _, file := range testFiles {
		if err := manager.index.Add(file); err != nil {
			t.Fatalf("add file to index: %v", err)
		}
	}

	stats, err := manager.GetStatistics()
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	if stats.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", stats.TotalFiles)
	}

	expectedStorage := int64(1024 + 2048 + 4096)
	if stats.StorageUsed != expectedStorage {
		t.Errorf("expected StorageUsed=%d, got %d", expectedStorage, stats.StorageUsed)
	}

	// Should have 2 collections: images and documents
	if stats.CollectionCount != 2 {
		t.Errorf("expected CollectionCount=2, got %d", stats.CollectionCount)
	}

	// Verify collection details
	if imagesCount, ok := stats.Collections["images"]; !ok || imagesCount != 2 {
		t.Errorf("expected images collection with 2 files, got %v", stats.Collections["images"])
	}
	if docsCount, ok := stats.Collections["documents"]; !ok || docsCount != 1 {
		t.Errorf("expected documents collection with 1 file, got %v", stats.Collections["documents"])
	}

	// Verify storage formatting
	if stats.StorageUsedFormatted == "" {
		t.Errorf("expected StorageUsedFormatted to be non-empty")
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

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestGetAllMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "metadata", "files.json")
	index, err := NewMetadataIndex(indexPath)
	if err != nil {
		t.Fatalf("new metadata index: %v", err)
	}

	// Initially should be empty
	allMeta := index.GetAllMetadata()
	if len(allMeta) != 0 {
		t.Errorf("expected empty metadata, got %d entries", len(allMeta))
	}

	// Add some metadata
	testMeta := FileMetadata{
		Hash:         "test_hash",
		OriginalName: "test.jpg",
		StoredPath:   "storage/images/jpg/test.jpg",
		Category:     "images/jpg",
		MimeType:     "image/jpeg",
		Size:         1024,
		UploadedAt:   time.Now().UTC(),
		Metadata:     nil,
	}

	if err := index.Add(testMeta); err != nil {
		t.Fatalf("add metadata: %v", err)
	}

	// Get all metadata
	allMeta = index.GetAllMetadata()
	if len(allMeta) != 1 {
		t.Errorf("expected 1 metadata entry, got %d", len(allMeta))
	}

	if allMeta[0].Hash != testMeta.Hash {
		t.Errorf("expected hash %s, got %s", testMeta.Hash, allMeta[0].Hash)
	}
}

func TestGetStatisticsIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Create actual files and store them
	testContent1 := []byte("test image content 1")
	testContent2 := []byte("test image content 2 longer")
	testContent3 := []byte("test document content")

	// Store files
	req1 := StoreRequest{
		Reader:       bytes.NewReader(testContent1),
		Filename:     "test1.jpg",
		MimeType:     "image/jpeg",
		Size:         int64(len(testContent1)),
		Metadata:     nil,
		CategoryHint: "images",
	}

	req2 := StoreRequest{
		Reader:       bytes.NewReader(testContent2),
		Filename:     "test2.png",
		MimeType:     "image/png",
		Size:         int64(len(testContent2)),
		Metadata:     nil,
		CategoryHint: "images",
	}

	req3 := StoreRequest{
		Reader:       bytes.NewReader(testContent3),
		Filename:     "doc.pdf",
		MimeType:     "application/pdf",
		Size:         int64(len(testContent3)),
		Metadata:     nil,
		CategoryHint: "documents",
	}

	_, err = manager.StoreFile(req1)
	if err != nil {
		t.Fatalf("store file 1: %v", err)
	}

	_, err = manager.StoreFile(req2)
	if err != nil {
		t.Fatalf("store file 2: %v", err)
	}

	_, err = manager.StoreFile(req3)
	if err != nil {
		t.Fatalf("store file 3: %v", err)
	}

	// Get statistics
	stats, err := manager.GetStatistics()
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	if stats.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", stats.TotalFiles)
	}

	expectedStorage := int64(len(testContent1) + len(testContent2) + len(testContent3))
	if stats.StorageUsed != expectedStorage {
		t.Errorf("expected StorageUsed=%d, got %d", expectedStorage, stats.StorageUsed)
	}

	if stats.CollectionCount < 1 {
		t.Errorf("expected at least 1 collection, got %d", stats.CollectionCount)
	}
}

