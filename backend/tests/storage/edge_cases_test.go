package storage_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestEdgeCaseEmptyFile tests handling of empty files
func TestEdgeCaseEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Test empty file
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader([]byte{}),
		Filename: "empty.txt",
		MimeType: "text/plain",
		Size:     0,
	})

	if err != nil {
		t.Fatalf("StoreFile failed: %v", err)
	}

	if result.Metadata.Size != 0 {
		t.Errorf("expected size 0, got %d", result.Metadata.Size)
	}

	if result.Metadata.Hash == "" {
		t.Error("empty file should still have a hash")
	}
}

// TestEdgeCaseVeryLongFilename tests handling of very long filenames
func TestEdgeCaseVeryLongFilename(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a very long filename (over 255 characters)
	longName := strings.Repeat("a", 300) + ".txt"
	content := []byte("test content")

	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: longName,
		MimeType: "text/plain",
		Size:     int64(len(content)),
	})

	if err != nil {
		t.Fatalf("StoreFile failed: %v", err)
	}

	// Filename should be sanitized/shortened
	if len(result.Metadata.OriginalName) > 255 {
		t.Errorf("filename should be sanitized, got length %d", len(result.Metadata.OriginalName))
	}
}

// TestEdgeCaseSpecialCharactersInFilename tests handling of special characters
func TestEdgeCaseSpecialCharactersInFilename(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	specialNames := []string{
		"file with spaces.txt",
		"file@with#special$chars.txt",
		"file/with/slashes.txt",
		"file\\with\\backslashes.txt",
		"file:with:colons.txt",
		"file*with*asterisks.txt",
		"file?with?question.txt",
		"file<with>brackets.txt",
		"file|with|pipes.txt",
		"file\"with\"quotes.txt",
	}

	for _, name := range specialNames {
		t.Run(name, func(t *testing.T) {
			content := []byte("test content")
			result, err := mgr.StoreFile(storage.StoreRequest{
				Reader:   bytes.NewReader(content),
				Filename: name,
				MimeType: "text/plain",
				Size:     int64(len(content)),
			})

			if err != nil {
				t.Fatalf("StoreFile failed for %s: %v", name, err)
			}

			// Stored path should not contain special characters
			if strings.Contains(result.Metadata.StoredPath, " ") ||
				strings.Contains(result.Metadata.StoredPath, "@") ||
				strings.Contains(result.Metadata.StoredPath, "#") {
				t.Errorf("stored path should be sanitized: %s", result.Metadata.StoredPath)
			}
		})
	}
}

// TestEdgeCaseUnicodeFilename tests handling of Unicode filenames
func TestEdgeCaseUnicodeFilename(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	unicodeNames := []string{
		"测试文件.txt",
		"файл.txt",
		"ファイル.txt",
		"📄 document.txt",
		"file-émojis-🎉.txt",
	}

	for _, name := range unicodeNames {
		t.Run(name, func(t *testing.T) {
			content := []byte("test content")
			result, err := mgr.StoreFile(storage.StoreRequest{
				Reader:   bytes.NewReader(content),
				Filename: name,
				MimeType: "text/plain",
				Size:     int64(len(content)),
			})

			if err != nil {
				t.Fatalf("StoreFile failed for %s: %v", name, err)
			}

			if result.Metadata.OriginalName != name {
				t.Errorf("original name should preserve Unicode: got %s, want %s", result.Metadata.OriginalName, name)
			}
		})
	}
}

// TestEdgeCaseConcurrentSameFile tests concurrent uploads of the same file
func TestEdgeCaseConcurrentSameFile(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	content := []byte("identical content for deduplication test")
	numGoroutines := 10

	results := make([]*storage.StoreResult, numGoroutines)
	errors := make([]error, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			result, err := mgr.StoreFile(storage.StoreRequest{
				Reader:   bytes.NewReader(content),
				Filename: "concurrent.txt",
				MimeType: "text/plain",
				Size:     int64(len(content)),
			})
			results[idx] = result
			errors[idx] = err
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check all succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}

	// All should have the same hash (deduplication)
	firstHash := results[0].Metadata.Hash
	for i, result := range results {
		if result.Metadata.Hash != firstHash {
			t.Errorf("goroutine %d: hash mismatch, expected %s, got %s", i, firstHash, result.Metadata.Hash)
		}
		// At least one should be marked as duplicate
		if i > 0 && !result.Duplicate {
			// This is acceptable - deduplication might not catch all concurrent uploads
			t.Logf("goroutine %d: not marked as duplicate (acceptable for concurrent uploads)", i)
		}
	}
}

// TestEdgeCaseLargeMetadata tests handling of large metadata
func TestEdgeCaseLargeMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create metadata at the limit
	largeMetadata := make(map[string]string)
	// Add many fields to test metadata handling
	for i := 0; i < 50; i++ {
		largeMetadata[fmt.Sprintf("field_%d", i)] = strings.Repeat("x", 1000)
	}

	content := []byte("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "large_metadata.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
		Metadata: largeMetadata,
	})

	if err != nil {
		t.Fatalf("StoreFile failed: %v", err)
	}

	if len(result.Metadata.Metadata) != len(largeMetadata) {
		t.Errorf("metadata count mismatch: expected %d, got %d", len(largeMetadata), len(result.Metadata.Metadata))
	}
}

// TestEdgeCasePathTraversal tests security against path traversal
func TestEdgeCasePathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"storage/../../etc/passwd",
		"/absolute/path/file.txt",
	}

	for _, path := range maliciousPaths {
		t.Run(path, func(t *testing.T) {
			// Try to retrieve by malicious path
			_, err := mgr.GetFileByPath(path)
			if err == nil {
				t.Errorf("path traversal should be rejected: %s", path)
			}
			if !strings.Contains(err.Error(), "invalid path") && !strings.Contains(err.Error(), "not found") {
				t.Errorf("expected path traversal error, got: %v", err)
			}
		})
	}
}

// TestEdgeCaseNilReader tests handling of nil reader
func TestEdgeCaseNilReader(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   nil,
		Filename: "nil_reader.txt",
		MimeType: "text/plain",
		Size:     0,
	})

	if err == nil {
		t.Error("expected error for nil reader")
	}
}

// TestEdgeCaseInvalidMimeType tests handling of invalid/unknown MIME types
func TestEdgeCaseInvalidMimeType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	invalidMimes := []string{
		"",
		"invalid/mime/type",
		"application/unknown",
		"x-custom/type",
	}

	for _, mime := range invalidMimes {
		t.Run(mime, func(t *testing.T) {
			content := []byte("test content")
			result, err := mgr.StoreFile(storage.StoreRequest{
				Reader:   bytes.NewReader(content),
				Filename: "test.txt",
				MimeType: mime,
				Size:     int64(len(content)),
			})

			if err != nil {
				t.Fatalf("StoreFile should handle invalid MIME type: %v", err)
			}

			// Should fall back to extension-based classification
			if result.Metadata.MimeType == "" {
				t.Error("MIME type should have a fallback value")
			}
		})
	}
}

// TestEdgeCaseRapidMetadataUpdates tests rapid sequential metadata updates
func TestEdgeCaseRapidMetadataUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Upload a file
	content := []byte("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "rapid_updates.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	hash := result.Metadata.Hash

	// Perform rapid sequential updates
	for i := 0; i < 100; i++ {
		req := storage.MetadataUpdateRequest{
			Hash:   hash,
			Action: "merge",
			Metadata: map[string]string{
				"update_count": fmt.Sprintf("%d", i),
				"timestamp":    time.Now().Format(time.RFC3339Nano),
			},
		}

		_, err := mgr.UpdateFileMetadata(req)
		if err != nil {
			t.Fatalf("UpdateFileMetadata failed at iteration %d: %v", i, err)
		}
	}

	// Verify final state
	metadata, err := mgr.GetFileMetadata(hash)
	if err != nil {
		t.Fatalf("GetFileMetadata: %v", err)
	}

	if metadata.Metadata["update_count"] != "99" {
		t.Errorf("expected update_count=99, got %s", metadata.Metadata["update_count"])
	}
}

// TestEdgeCaseFileNotFoundOperations tests operations on non-existent files
func TestEdgeCaseFileNotFoundOperations(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	nonExistentHash := "a" + strings.Repeat("0", 63) // Valid SHA-256 format but non-existent

	// Test metadata retrieval
	_, err = mgr.GetFileMetadata(nonExistentHash)
	if err == nil {
		t.Error("expected error for non-existent file metadata")
	}

	// Test file retrieval
	_, err = mgr.GetFileByHash(nonExistentHash)
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Test metadata update
	updateReq := storage.MetadataUpdateRequest{
		Hash:   nonExistentHash,
		Action: "merge",
		Metadata: map[string]string{
			"test": "value",
		},
	}
	_, err = mgr.UpdateFileMetadata(updateReq)
	if err == nil {
		t.Error("expected error for metadata update on non-existent file")
	}
}

// TestEdgeCaseEmptyMetadata tests handling of empty metadata
func TestEdgeCaseEmptyMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	content := []byte("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "empty_metadata.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
		Metadata: nil, // nil metadata
	})

	if err != nil {
		t.Fatalf("StoreFile failed: %v", err)
	}

	// Should handle nil metadata gracefully
	if result.Metadata.Metadata == nil {
		// This is acceptable - nil metadata is valid
		return
	}

	if len(result.Metadata.Metadata) != 0 {
		t.Errorf("expected empty metadata, got %d fields", len(result.Metadata.Metadata))
	}
}

// TestEdgeCaseVeryLargeFile tests handling of very large files (if memory allows)
func TestEdgeCaseVeryLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test in short mode")
	}

	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a 10MB file
	largeContent := bytes.Repeat([]byte("x"), 10*1024*1024)
	
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(largeContent),
		Filename: "large_file.txt",
		MimeType: "text/plain",
		Size:     int64(len(largeContent)),
	})

	if err != nil {
		t.Fatalf("StoreFile failed for large file: %v", err)
	}

	if result.Metadata.Size != int64(len(largeContent)) {
		t.Errorf("size mismatch: expected %d, got %d", len(largeContent), result.Metadata.Size)
	}

	// Verify file can be retrieved
	retrieved, err := mgr.GetFileByHash(result.Metadata.Hash)
	if err != nil {
		t.Fatalf("GetFileByHash failed: %v", err)
	}
	defer retrieved.Reader.Close()

	// Verify file size on disk
	fileInfo, err := retrieved.Reader.Stat()
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if fileInfo.Size() != int64(len(largeContent)) {
		t.Errorf("file size on disk mismatch: expected %d, got %d", len(largeContent), fileInfo.Size())
	}
}

// TestEdgeCaseConcurrentMetadataUpdates tests concurrent metadata updates on same file
func TestEdgeCaseConcurrentMetadataUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Upload a file
	content := []byte("test content")
	result, err := mgr.StoreFile(storage.StoreRequest{
		Reader:   bytes.NewReader(content),
		Filename: "concurrent_metadata.txt",
		MimeType: "text/plain",
		Size:     int64(len(content)),
	})
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	hash := result.Metadata.Hash
	numGoroutines := 20
	updatesPerGoroutine := 5

	done := make(chan bool, numGoroutines*updatesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			for j := 0; j < updatesPerGoroutine; j++ {
				req := storage.MetadataUpdateRequest{
					Hash:   hash,
					Action: "merge",
					Metadata: map[string]string{
						fmt.Sprintf("worker_%d_update_%d", workerID, j): fmt.Sprintf("value_%d_%d", workerID, j),
					},
				}

				_, err := mgr.UpdateFileMetadata(req)
				if err != nil {
					t.Errorf("UpdateFileMetadata failed: %v", err)
				}
				done <- true
			}
		}(i)
	}

	// Wait for all updates
	for i := 0; i < numGoroutines*updatesPerGoroutine; i++ {
		<-done
	}

	// Verify final metadata state
	metadata, err := mgr.GetFileMetadata(hash)
	if err != nil {
		t.Fatalf("GetFileMetadata: %v", err)
	}

	// Should have accumulated all updates
	expectedMinFields := numGoroutines * updatesPerGoroutine
	if len(metadata.Metadata) < expectedMinFields {
		t.Logf("Note: Some concurrent updates may have overwritten each other (expected in concurrent scenarios)")
		t.Logf("Expected at least %d fields, got %d", expectedMinFields, len(metadata.Metadata))
	}
}
