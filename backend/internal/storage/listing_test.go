package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListFiles_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add some test files
	testFiles := []struct {
		name     string
		mimeType string
		category string
		size     int64
	}{
		{"test1.pdf", "application/pdf", "documents/pdf", 1024},
		{"test2.jpg", "image/jpeg", "images/jpg", 2048},
		{"test3.mp4", "video/mp4", "videos/mp4", 4096},
		{"test4.png", "image/png", "images/png", 512},
		{"test5.doc", "application/msword", "documents/doc", 256},
	}

	for _, tf := range testFiles {
		meta := FileMetadata{
			Hash:         "hash_" + tf.name,
			OriginalName: tf.name,
			StoredPath:   "storage/" + tf.category + "/" + tf.name,
			Category:     tf.category,
			MimeType:     tf.mimeType,
			Size:         tf.size,
			UploadedAt:   time.Now().UTC(),
		}
		if err := manager.index.Add(meta); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	// Test basic listing
	result, err := manager.ListFiles(ListOptions{})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result.Pagination.Total != len(testFiles) {
		t.Errorf("expected %d files, got %d", len(testFiles), result.Pagination.Total)
	}

	if len(result.Files) != len(testFiles) {
		t.Errorf("expected %d files in result, got %d", len(testFiles), len(result.Files))
	}
}

func TestListFiles_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add 10 test files
	for i := 0; i < 10; i++ {
		meta := FileMetadata{
			Hash:         "hash_" + string(rune('0'+i)),
			OriginalName: "file" + string(rune('0'+i)) + ".txt",
			StoredPath:   "storage/test/file" + string(rune('0'+i)) + ".txt",
			Category:     "test",
			MimeType:     "text/plain",
			Size:         100,
			UploadedAt:   time.Now().UTC(),
		}
		if err := manager.index.Add(meta); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	// Test pagination: page 1, limit 3
	result, err := manager.ListFiles(ListOptions{Page: 1, Limit: 3})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(result.Files) != 3 {
		t.Errorf("expected 3 files on page 1, got %d", len(result.Files))
	}

	if result.Pagination.TotalPages != 4 {
		t.Errorf("expected 4 total pages, got %d", result.Pagination.TotalPages)
	}

	if !result.Pagination.HasNext {
		t.Error("expected has_next to be true")
	}

	if result.Pagination.HasPrev {
		t.Error("expected has_prev to be false on first page")
	}

	// Test page 2
	result2, err := manager.ListFiles(ListOptions{Page: 2, Limit: 3})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(result2.Files) != 3 {
		t.Errorf("expected 3 files on page 2, got %d", len(result2.Files))
	}

	if !result2.Pagination.HasPrev {
		t.Error("expected has_prev to be true on page 2")
	}
}

func TestListFiles_Filtering(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add test files with different categories
	files := []FileMetadata{
		{Hash: "h1", OriginalName: "img1.jpg", Category: "images/jpg", MimeType: "image/jpeg", Size: 100, UploadedAt: time.Now()},
		{Hash: "h2", OriginalName: "img2.png", Category: "images/png", MimeType: "image/png", Size: 200, UploadedAt: time.Now()},
		{Hash: "h3", OriginalName: "doc1.pdf", Category: "documents/pdf", MimeType: "application/pdf", Size: 300, UploadedAt: time.Now()},
		{Hash: "h4", OriginalName: "vid1.mp4", Category: "videos/mp4", MimeType: "video/mp4", Size: 400, UploadedAt: time.Now()},
	}

	for _, f := range files {
		if err := manager.index.Add(f); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	// Test category filter
	result, err := manager.ListFiles(ListOptions{Category: "images"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result.Pagination.Total != 2 {
		t.Errorf("expected 2 image files, got %d", result.Pagination.Total)
	}

	// Test type filter
	result2, err := manager.ListFiles(ListOptions{Type: "image"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result2.Pagination.Total != 2 {
		t.Errorf("expected 2 image files by type, got %d", result2.Pagination.Total)
	}

	// Test MIME type filter
	result3, err := manager.ListFiles(ListOptions{MimeType: "application/pdf"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result3.Pagination.Total != 1 {
		t.Errorf("expected 1 PDF file, got %d", result3.Pagination.Total)
	}

	// Test extension filter
	result4, err := manager.ListFiles(ListOptions{Extension: "jpg"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result4.Pagination.Total != 1 {
		t.Errorf("expected 1 JPG file, got %d", result4.Pagination.Total)
	}

	// Test name filter
	result5, err := manager.ListFiles(ListOptions{Name: "img"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result5.Pagination.Total != 2 {
		t.Errorf("expected 2 files with 'img' in name, got %d", result5.Pagination.Total)
	}
}

func TestListFiles_Sorting(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	now := time.Now().UTC()
	files := []FileMetadata{
		{Hash: "h1", OriginalName: "zebra.txt", Category: "test", MimeType: "text/plain", Size: 300, UploadedAt: now.Add(-2 * time.Hour)},
		{Hash: "h2", OriginalName: "alpha.txt", Category: "test", MimeType: "text/plain", Size: 100, UploadedAt: now.Add(-1 * time.Hour)},
		{Hash: "h3", OriginalName: "beta.txt", Category: "test", MimeType: "text/plain", Size: 200, UploadedAt: now},
	}

	for _, f := range files {
		if err := manager.index.Add(f); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	// Test sort by name ascending
	result, err := manager.ListFiles(ListOptions{SortBy: "name", Order: "asc"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result.Files[0].OriginalName != "alpha.txt" {
		t.Errorf("expected first file to be alpha.txt, got %s", result.Files[0].OriginalName)
	}

	// Test sort by size descending
	result2, err := manager.ListFiles(ListOptions{SortBy: "size", Order: "desc"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result2.Files[0].Size != 300 {
		t.Errorf("expected first file size to be 300, got %d", result2.Files[0].Size)
	}

	// Test sort by uploaded_at descending (default)
	result3, err := manager.ListFiles(ListOptions{SortBy: "uploaded_at", Order: "desc"})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result3.Files[0].OriginalName != "beta.txt" {
		t.Errorf("expected first file to be beta.txt (most recent), got %s", result3.Files[0].OriginalName)
	}
}

func TestListFiles_DateFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	baseTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	files := []FileMetadata{
		{Hash: "h1", OriginalName: "old.txt", Category: "test", MimeType: "text/plain", Size: 100, UploadedAt: baseTime.Add(-5 * 24 * time.Hour)},
		{Hash: "h2", OriginalName: "recent.txt", Category: "test", MimeType: "text/plain", Size: 100, UploadedAt: baseTime.Add(-1 * 24 * time.Hour)},
		{Hash: "h3", OriginalName: "today.txt", Category: "test", MimeType: "text/plain", Size: 100, UploadedAt: baseTime},
	}

	for _, f := range files {
		if err := manager.index.Add(f); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	// Test date_from filter
	result, err := manager.ListFiles(ListOptions{DateFrom: baseTime.Add(-2 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result.Pagination.Total != 2 {
		t.Errorf("expected 2 files after date_from, got %d", result.Pagination.Total)
	}

	// Test date_to filter
	result2, err := manager.ListFiles(ListOptions{DateTo: baseTime.Add(-2 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result2.Pagination.Total != 1 {
		t.Errorf("expected 1 file before date_to, got %d", result2.Pagination.Total)
	}

	// Test date range
	result3, err := manager.ListFiles(ListOptions{
		DateFrom: baseTime.Add(-3 * 24 * time.Hour),
		DateTo:   baseTime.Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if result3.Pagination.Total != 1 {
		t.Errorf("expected 1 file in date range, got %d", result3.Pagination.Total)
	}
}

func TestBrowseDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create some test directories and files
	storageDir := filepath.Join(tmpDir, "storage")
	testDir := filepath.Join(storageDir, "test")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(testDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test browsing storage root
	result, err := manager.BrowseDirectory("storage")
	if err != nil {
		t.Fatalf("failed to browse directory: %v", err)
	}

	if result.Path != "storage" {
		t.Errorf("expected path to be 'storage', got %s", result.Path)
	}

	if len(result.Entries) == 0 {
		t.Error("expected at least one entry in storage directory")
	}

	// Test browsing subdirectory
	result2, err := manager.BrowseDirectory("storage/test")
	if err != nil {
		t.Fatalf("failed to browse subdirectory: %v", err)
	}

	if result2.Path != "storage/test" {
		t.Errorf("expected path to be 'storage/test', got %s", result2.Path)
	}

	if result2.ParentPath != "storage" {
		t.Errorf("expected parent path to be 'storage', got %s", result2.ParentPath)
	}

	if len(result2.Breadcrumbs) < 2 {
		t.Error("expected at least 2 breadcrumbs")
	}
}

func TestGetCategories(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add files to different categories
	files := []FileMetadata{
		{Hash: "h1", OriginalName: "f1.jpg", Category: "images/jpg", MimeType: "image/jpeg", Size: 100, UploadedAt: time.Now()},
		{Hash: "h2", OriginalName: "f2.jpg", Category: "images/jpg", MimeType: "image/jpeg", Size: 200, UploadedAt: time.Now()},
		{Hash: "h3", OriginalName: "f3.png", Category: "images/png", MimeType: "image/png", Size: 300, UploadedAt: time.Now()},
		{Hash: "h4", OriginalName: "f4.pdf", Category: "documents/pdf", MimeType: "application/pdf", Size: 400, UploadedAt: time.Now()},
	}

	for _, f := range files {
		if err := manager.index.Add(f); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	categories, err := manager.GetCategories()
	if err != nil {
		t.Fatalf("failed to get categories: %v", err)
	}

	if len(categories) != 3 {
		t.Errorf("expected 3 categories, got %d", len(categories))
	}

	// Check that categories are sorted by count (descending)
	if categories[0].Count < categories[1].Count {
		t.Error("categories should be sorted by count descending")
	}

	// Find images/jpg category
	var jpgCategory *CategoryInfo
	for i := range categories {
		if categories[i].Path == "images/jpg" {
			jpgCategory = &categories[i]
			break
		}
	}

	if jpgCategory == nil {
		t.Fatal("images/jpg category not found")
	}

	if jpgCategory.Count != 2 {
		t.Errorf("expected 2 files in images/jpg, got %d", jpgCategory.Count)
	}

	if jpgCategory.Size != 300 {
		t.Errorf("expected total size 300 in images/jpg, got %d", jpgCategory.Size)
	}
}

func TestGetStorageStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	now := time.Now().UTC()
	files := []FileMetadata{
		{Hash: "h1", OriginalName: "f1.jpg", Category: "images/jpg", MimeType: "image/jpeg", Size: 100, UploadedAt: now.Add(-10 * time.Hour)},
		{Hash: "h2", OriginalName: "f2.png", Category: "images/png", MimeType: "image/png", Size: 200, UploadedAt: now.Add(-2 * 24 * time.Hour)},
		{Hash: "h3", OriginalName: "f3.pdf", Category: "documents/pdf", MimeType: "application/pdf", Size: 300, UploadedAt: now.Add(-10 * 24 * time.Hour)},
	}

	for _, f := range files {
		if err := manager.index.Add(f); err != nil {
			t.Fatalf("failed to add metadata: %v", err)
		}
	}

	stats, err := manager.GetStorageStats()
	if err != nil {
		t.Fatalf("failed to get storage stats: %v", err)
	}

	if stats.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", stats.TotalFiles)
	}

	if stats.TotalSize != 600 {
		t.Errorf("expected total size 600, got %d", stats.TotalSize)
	}

	if stats.RecentUploads.Last24Hours != 1 {
		t.Errorf("expected 1 file in last 24 hours, got %d", stats.RecentUploads.Last24Hours)
	}

	if stats.RecentUploads.Last7Days != 2 {
		t.Errorf("expected 2 files in last 7 days, got %d", stats.RecentUploads.Last7Days)
	}

	if stats.RecentUploads.Last30Days != 3 {
		t.Errorf("expected 3 files in last 30 days, got %d", stats.RecentUploads.Last30Days)
	}

	if len(stats.FileTypes) != 3 {
		t.Errorf("expected 3 file types, got %d", len(stats.FileTypes))
	}

	if stats.FileTypes["image/jpeg"] != 1 {
		t.Errorf("expected 1 image/jpeg file, got %d", stats.FileTypes["image/jpeg"])
	}
}
