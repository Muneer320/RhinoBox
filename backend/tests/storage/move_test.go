package storage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestMoveFileBasic(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store a test file
	content := []byte("test image content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "photo.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	originalPath := result.Metadata.StoredPath
	originalCategory := result.Metadata.Category

	// Move the file to a new category
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FileHash:    result.Metadata.Hash,
		NewCategory: "images/png/archive",
		Reason:      "test move",
	})
	if err != nil {
		t.Fatalf("move file: %v", err)
	}

	// Verify the move
	if moveResult.OldPath != originalPath {
		t.Errorf("expected old path %s, got %s", originalPath, moveResult.OldPath)
	}
	if moveResult.OldCategory != originalCategory {
		t.Errorf("expected old category %s, got %s", originalCategory, moveResult.OldCategory)
	}
	if moveResult.NewCategory != "images/png/archive" {
		t.Errorf("expected new category images/png/archive, got %s", moveResult.NewCategory)
	}

	// Verify the file exists at new location
	newAbsPath := filepath.Join(tmpDir, filepath.FromSlash(moveResult.NewPath))
	if _, err := os.Stat(newAbsPath); err != nil {
		t.Errorf("new file not found: %v", err)
	}

	// Verify the file doesn't exist at old location
	oldAbsPath := filepath.Join(tmpDir, filepath.FromSlash(originalPath))
	if _, err := os.Stat(oldAbsPath); !os.IsNotExist(err) {
		t.Errorf("old file still exists")
	}

	// Verify metadata was updated
	if moveResult.Metadata.StoredPath != moveResult.NewPath {
		t.Errorf("metadata path not updated")
	}
	if moveResult.Metadata.Category != "images/png/archive" {
		t.Errorf("metadata category not updated")
	}
	if moveResult.Metadata.Metadata["move_reason"] != "test move" {
		t.Errorf("move reason not recorded in metadata")
	}
}

func TestMoveFileByPath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store a test file
	content := []byte("test document")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "doc.pdf",
		MimeType: "application/pdf",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	// Move the file using path instead of hash
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FilePath:    result.Metadata.StoredPath,
		NewCategory: "documents/pdf/reports",
		Reason:      "reorganization",
	})
	if err != nil {
		t.Fatalf("move file by path: %v", err)
	}

	if moveResult.NewCategory != "documents/pdf/reports" {
		t.Errorf("expected new category documents/pdf/reports, got %s", moveResult.NewCategory)
	}

	// Verify file exists at new location
	newAbsPath := filepath.Join(tmpDir, filepath.FromSlash(moveResult.NewPath))
	if _, err := os.Stat(newAbsPath); err != nil {
		t.Errorf("moved file not found: %v", err)
	}
}

func TestMoveFileConflictResolution(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store two files with same original name but different content
	// They will get different stored names due to hash prefixes
	content1 := []byte("first file")
	result1, err := mgr.StoreFile(storage.StoreRequest{
		Reader:       bytes.NewReader(content1),
		Filename:     "test.jpg",
		MimeType:     "image/jpeg",
		Size:         int64(len(content1)),
		CategoryHint: "photos",
	})
	if err != nil {
		t.Fatalf("store file 1: %v", err)
	}

	content2 := []byte("second file with different content")
	result2, err := mgr.StoreFile(storage.StoreRequest{
		Reader:       bytes.NewReader(content2),
		Filename:     "test.jpg",
		MimeType:     "image/jpeg",
		Size:         int64(len(content2)),
		CategoryHint: "vacation",
	})
	if err != nil {
		t.Fatalf("store file 2: %v", err)
	}

	// First, manually rename file1 to have the exact name we want for testing conflict
	file1Path := filepath.Join(tmpDir, filepath.FromSlash(result1.Metadata.StoredPath))
	file1Dir := filepath.Dir(file1Path)
	conflictName := filepath.Base(filepath.Join(tmpDir, filepath.FromSlash(result2.Metadata.StoredPath)))
	conflictPath := filepath.Join(file1Dir, conflictName)
	
	// Create the conflict manually by copying file2's name to file1's location
	if err := os.Rename(file1Path, conflictPath); err != nil {
		t.Fatalf("setup conflict: %v", err)
	}

	// Now move file 2 to the same category as file 1
	// This should trigger conflict resolution
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FileHash:    result2.Metadata.Hash,
		NewCategory: result1.Metadata.Category,
		Reason:      "consolidation",
	})
	if err != nil {
		t.Fatalf("move file with conflict: %v", err)
	}

	// Verify the file was renamed
	if !moveResult.Renamed {
		t.Error("expected file to be renamed due to conflict")
	}

	// Verify both files exist
	file2Path := filepath.Join(tmpDir, filepath.FromSlash(moveResult.NewPath))
	
	if _, err := os.Stat(conflictPath); err != nil {
		t.Errorf("original file not found: %v", err)
	}
	if _, err := os.Stat(file2Path); err != nil {
		t.Errorf("moved file not found: %v", err)
	}

	// Verify they have different names
	if filepath.Base(conflictPath) == filepath.Base(file2Path) {
		t.Error("files should have different names after conflict resolution")
	}
}

func TestMoveFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Try to move a non-existent file
	_, err = mgr.MoveFile(storage.MoveRequest{
		FileHash:    "nonexistent",
		NewCategory: "images/png",
		Reason:      "test",
	})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}
}

func TestMoveFileInvalidCategory(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store a test file
	content := []byte("test")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	// Try to move with empty category
	_, err = mgr.MoveFile(storage.MoveRequest{
		FileHash:    result.Metadata.Hash,
		NewCategory: "",
		Reason:      "test",
	})
	if err == nil {
		t.Error("expected error for empty category")
	}
}

func TestBatchMoveFiles(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store multiple test files
	files := make([]storage.FileMetadata, 3)
	for i := 0; i < 3; i++ {
		content := []byte("file content " + string(rune('A'+i)))
		result, err := mgr.StoreFile(storage.StoreRequest{
			Reader:   bytes.NewReader(content),
			Filename: "file" + string(rune('A'+i)) + ".jpg",
			MimeType: "image/jpeg",
			Size:     int64(len(content)),
		})
		if err != nil {
			t.Fatalf("store file %d: %v", i, err)
		}
		files[i] = result.Metadata
	}

	// Batch move all files to a new category
	batchReq := storage.BatchMoveRequest{
		Files: []storage.MoveRequest{
			{FileHash: files[0].Hash, NewCategory: "images/jpg/archive", Reason: "cleanup"},
			{FileHash: files[1].Hash, NewCategory: "images/jpg/archive", Reason: "cleanup"},
			{FileHash: files[2].Hash, NewCategory: "images/jpg/archive", Reason: "cleanup"},
		},
	}

	result, err := mgr.BatchMoveFiles(batchReq)
	if err != nil {
		t.Fatalf("batch move files: %v", err)
	}

	// Verify results
	if result.Success != 3 {
		t.Errorf("expected 3 successful moves, got %d", result.Success)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed moves, got %d", result.Failed)
	}
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(result.Results))
	}

	// Verify all files are in new location
	for _, r := range result.Results {
		if r.NewCategory != "images/jpg/archive" {
			t.Errorf("expected category images/jpg/archive, got %s", r.NewCategory)
		}
		newPath := filepath.Join(tmpDir, filepath.FromSlash(r.NewPath))
		if _, err := os.Stat(newPath); err != nil {
			t.Errorf("moved file not found: %v", err)
		}
	}
}

func TestBatchMoveFilesRollback(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store two test files
	content1 := []byte("file 1")
	result1, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content1),
		Filename: "file1.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content1)),
	})
	if err != nil {
		t.Fatalf("store file 1: %v", err)
	}

	content2 := []byte("file 2")
	result2, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content2),
		Filename: "file2.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content2)),
	})
	if err != nil {
		t.Fatalf("store file 2: %v", err)
	}

	// Save original paths
	originalPath1 := result1.Metadata.StoredPath
	originalPath2 := result2.Metadata.StoredPath

	// Attempt batch move with one invalid file
	batchReq := storage.BatchMoveRequest{
		Files: []storage.MoveRequest{
			{FileHash: result1.Metadata.Hash, NewCategory: "images/jpg/moved", Reason: "test"},
			{FileHash: "nonexistent", NewCategory: "images/jpg/moved", Reason: "test"},
		},
	}

	_, err = mgr.BatchMoveFiles(batchReq)
	if err == nil {
		t.Error("expected error for batch move with invalid file")
	}

	// Verify rollback: first file should still be at original location
	originalAbs1 := filepath.Join(tmpDir, filepath.FromSlash(originalPath1))
	if _, err := os.Stat(originalAbs1); err != nil {
		t.Error("first file should have been rolled back to original location")
	}

	// Verify second file is still at original location
	originalAbs2 := filepath.Join(tmpDir, filepath.FromSlash(originalPath2))
	if _, err := os.Stat(originalAbs2); err != nil {
		t.Error("second file should remain at original location")
	}
}

func TestMoveFilePreservesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store a file with custom metadata
	content := []byte("test with metadata")
	originalMeta := map[string]string{
		"comment": "original comment",
		"tags":    "important,archived",
	}
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "important.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content)),
		Metadata: originalMeta,
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	originalHash := result.Metadata.Hash
	originalUploadedAt := result.Metadata.UploadedAt
	originalName := result.Metadata.OriginalName
	originalSize := result.Metadata.Size

	// Move the file
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FileHash:    result.Metadata.Hash,
		NewCategory: "images/jpg/critical",
		Reason:      "prioritization",
	})
	if err != nil {
		t.Fatalf("move file: %v", err)
	}

	// Verify preserved metadata
	if moveResult.Metadata.Hash != originalHash {
		t.Errorf("hash changed after move: %s -> %s", originalHash, moveResult.Metadata.Hash)
	}
	if moveResult.Metadata.OriginalName != originalName {
		t.Errorf("original name changed: %s -> %s", originalName, moveResult.Metadata.OriginalName)
	}
	if moveResult.Metadata.Size != originalSize {
		t.Errorf("size changed: %d -> %d", originalSize, moveResult.Metadata.Size)
	}
	if !moveResult.Metadata.UploadedAt.Equal(originalUploadedAt) {
		t.Errorf("uploaded_at changed")
	}
	if moveResult.Metadata.Metadata["comment"] != "original comment" {
		t.Errorf("original metadata lost")
	}
	if moveResult.Metadata.Metadata["tags"] != "important,archived" {
		t.Errorf("original metadata lost")
	}

	// Verify move-specific metadata was added
	if moveResult.Metadata.Metadata["move_reason"] != "prioritization" {
		t.Errorf("move reason not recorded")
	}
	if moveResult.Metadata.Metadata["moved_at"] == "" {
		t.Errorf("moved_at not recorded")
	}
	if moveResult.Metadata.Metadata["moved_from"] != result.Metadata.StoredPath {
		t.Errorf("moved_from not recorded correctly")
	}
}

func TestMoveFileToDifferentMediaTypes(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store an image
	content := []byte("image data")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "photo.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	// Move to a completely different category structure
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FileHash:    result.Metadata.Hash,
		NewCategory: "documents/pdf/scanned",
		Reason:      "recategorization",
	})
	if err != nil {
		t.Fatalf("move file: %v", err)
	}

	if moveResult.NewCategory != "documents/pdf/scanned" {
		t.Errorf("expected new category documents/pdf/scanned, got %s", moveResult.NewCategory)
	}

	// Verify file exists at new location
	newPath := filepath.Join(tmpDir, filepath.FromSlash(moveResult.NewPath))
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(data) != "image data" {
		t.Error("file content changed after move")
	}
}

func TestMoveFileCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Store a file
	content := []byte("test")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("store file: %v", err)
	}

	// Move to a new deeply nested category that doesn't exist
	moveResult, err := mgr.MoveFile(storage.MoveRequest{
		FileHash:    result.Metadata.Hash,
		NewCategory: "images/jpg/2025/november/week3/vacation/beach",
		Reason:      "organization",
	})
	if err != nil {
		t.Fatalf("move file: %v", err)
	}

	// Verify all directories were created
	newPath := filepath.Join(tmpDir, filepath.FromSlash(moveResult.NewPath))
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("file not found at new location: %v", err)
	}

	dir := filepath.Dir(newPath)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}
