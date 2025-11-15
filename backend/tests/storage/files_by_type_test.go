package storage

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestGetFilesByType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store files of different types
	files := []struct {
		name     string
		content  []byte
		mimeType string
		expected string
	}{
		{"image1.jpg", []byte("image content 1"), "image/jpeg", "images"},
		{"image2.png", []byte("image content 2"), "image/png", "images"},
		{"video1.mp4", []byte("video content 1"), "video/mp4", "videos"},
		{"video2.mov", []byte("video content 2"), "video/quicktime", "videos"},
		{"audio1.mp3", []byte("audio content 1"), "audio/mpeg", "audio"},
		{"doc1.pdf", []byte("document content"), "application/pdf", "documents"},
	}

	for _, file := range files {
		_, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(file.content),
			Filename: file.name,
			MimeType: file.mimeType,
			Size:     int64(len(file.content)),
		})
		if err != nil {
			t.Fatalf("failed to store file %s: %v", file.name, err)
		}
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Test: Get images
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to get files by type: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 images, got %d", result.Total)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files in response, got %d", len(result.Files))
	}
	if result.Page != 1 {
		t.Errorf("expected page 1, got %d", result.Page)
	}
	if result.Limit != 10 {
		t.Errorf("expected limit 10, got %d", result.Limit)
	}
	if result.TotalPages != 1 {
		t.Errorf("expected 1 total page, got %d", result.TotalPages)
	}

	// Verify all returned files are images
	for _, file := range result.Files {
		categoryParts := strings.Split(file.Category, "/")
		if len(categoryParts) == 0 || categoryParts[0] != "images" {
			t.Errorf("expected image category, got %s", file.Category)
		}
	}

	// Test: Get videos
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "videos",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to get videos: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 videos, got %d", result.Total)
	}

	// Test: Get audio
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "audio",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to get audio: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("expected 1 audio file, got %d", result.Total)
	}

	// Test: Get documents
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "documents",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to get documents: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("expected 1 document, got %d", result.Total)
	}
}

func TestGetFilesByType_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store 5 images with unique content to avoid deduplication
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("image content %d", i))
		_, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(content),
			Filename: fmt.Sprintf("image_%d.jpg", i),
			MimeType: "image/jpeg",
			Size:     int64(len(content)),
		})
		if err != nil {
			t.Fatalf("failed to store file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Test: First page (limit 2)
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  1,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files on page 1, got %d", len(result.Files))
	}
	if result.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", result.TotalPages)
	}

	// Test: Second page
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  2,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if len(result.Files) != 2 {
		t.Errorf("expected 2 files on page 2, got %d", len(result.Files))
	}

	// Test: Third page (should have 1 file)
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  3,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if len(result.Files) != 1 {
		t.Errorf("expected 1 file on page 3, got %d", len(result.Files))
	}

	// Test: Page beyond available data
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  10,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if len(result.Files) != 0 {
		t.Errorf("expected 0 files on page 10, got %d", len(result.Files))
	}
}

func TestGetFilesByType_DefaultPagination(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store one file
	_, err = mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader([]byte("content")),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     7,
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Test: Default pagination (page 0 should default to 1)
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type: "images",
		Page: 0,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if result.Page != 1 {
		t.Errorf("expected page to default to 1, got %d", result.Page)
	}
	if result.Limit != 50 {
		t.Errorf("expected limit to default to 50, got %d", result.Limit)
	}

	// Test: Max limit
	result, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  1,
		Limit: 2000, // Should be capped at 1000
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if result.Limit != 1000 {
		t.Errorf("expected limit to be capped at 1000, got %d", result.Limit)
	}
}

func TestGetFilesByType_CategoryFilter(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store images with different categories
	_, err = mgr.StoreFile(storage.StoreRequest{
		Reader:       bytes.NewReader([]byte("content1")),
		Filename:     "photo1.jpg",
		MimeType:     "image/jpeg",
		Size:         8,
		CategoryHint: "vacation",
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	_, err = mgr.StoreFile(storage.StoreRequest{
		Reader:       bytes.NewReader([]byte("content2")),
		Filename:     "photo2.jpg",
		MimeType:     "image/jpeg",
		Size:         8,
		CategoryHint: "vacation",
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	_, err = mgr.StoreFile(storage.StoreRequest{
		Reader:       bytes.NewReader([]byte("content3")),
		Filename:     "photo3.jpg",
		MimeType:     "image/jpeg",
		Size:         8,
		CategoryHint: "work",
	})
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Test: Filter by category
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:     "images",
		Page:     1,
		Limit:    10,
		Category: "vacation",
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 files with vacation category, got %d", result.Total)
	}
}

func TestGetFilesByType_EmptyType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test: Empty type should return error
	_, err = mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type: "",
	})
	if err == nil {
		t.Error("expected error for empty type")
	}
}

func TestGetFilesByType_NonExistentType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test: Non-existent type should return empty result
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "nonexistent",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected 0 files for nonexistent type, got %d", result.Total)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected 0 files in response, got %d", len(result.Files))
	}
}

func TestGetFilesByType_Sorting(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Store files with unique content and delays to ensure different timestamps
	for i := 0; i < 3; i++ {
		content := []byte(fmt.Sprintf("content %d", i))
		_, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(content),
			Filename: fmt.Sprintf("image_%d.jpg", i),
			MimeType: "image/jpeg",
			Size:     int64(len(content)),
		})
		if err != nil {
			t.Fatalf("failed to store file: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Test: Files should be sorted by upload date (newest first)
	result, err := mgr.GetFilesByType(storage.GetFilesByTypeRequest{
		Type:  "images",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	if len(result.Files) < 2 {
		t.Fatalf("need at least 2 files to test sorting")
	}

	// Verify files are sorted by date (newest first)
	for i := 0; i < len(result.Files)-1; i++ {
		if result.Files[i].UploadedAt.Before(result.Files[i+1].UploadedAt) {
			t.Errorf("files not sorted correctly: file %d is older than file %d", i, i+1)
		}
	}
}

