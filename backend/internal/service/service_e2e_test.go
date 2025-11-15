package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestEndToEnd_FileOperations tests the complete flow of file operations through the service layer.
func TestEndToEnd_FileOperations(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Step 1: Store a file
	t.Log("Step 1: Storing file")
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("end-to-end test content")),
		Filename: "e2e_test.txt",
		MimeType: "text/plain",
		Size:     23,
		Metadata: map[string]string{
			"test":     "true",
			"category": "e2e",
		},
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}
	if stored.Hash == "" {
		t.Fatal("stored file hash is empty")
	}
	t.Logf("Stored file with hash: %s", stored.Hash)

	// Step 2: Retrieve file metadata
	t.Log("Step 2: Retrieving file metadata")
	metadata, err := service.GetFileMetadata(stored.Hash)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}
	if metadata.OriginalName != "e2e_test.txt" {
		t.Errorf("original name mismatch: got %s, want e2e_test.txt", metadata.OriginalName)
	}
	if metadata.Metadata["test"] != "true" {
		t.Errorf("metadata mismatch: got %s, want true", metadata.Metadata["test"])
	}

	// Step 3: Update metadata
	t.Log("Step 3: Updating file metadata")
	updateReq := MetadataUpdateRequest{
		Hash:     stored.Hash,
		Action:   "merge",
		Metadata: map[string]string{"updated": "yes"},
	}
	updateResult, err := service.UpdateFileMetadata(updateReq)
	if err != nil {
		t.Fatalf("failed to update metadata: %v", err)
	}
	if updateResult.NewMetadata["updated"] != "yes" {
		t.Errorf("updated metadata not found: %v", updateResult.NewMetadata)
	}

	// Step 4: Rename file
	t.Log("Step 4: Renaming file")
	renameReq := FileRenameRequest{
		Hash:             stored.Hash,
		NewName:          "renamed_e2e_test.txt",
		UpdateStoredFile: false,
	}
	renameResult, err := service.RenameFile(renameReq)
	if err != nil {
		t.Fatalf("failed to rename file: %v", err)
	}
	if renameResult.NewName != "renamed_e2e_test.txt" {
		t.Errorf("rename failed: got %s, want renamed_e2e_test.txt", renameResult.NewName)
	}

	// Step 5: Search for file
	t.Log("Step 5: Searching for file")
	searchReq := FileSearchRequest{Query: "renamed"}
	searchResult, err := service.SearchFiles(searchReq)
	if err != nil {
		t.Fatalf("failed to search files: %v", err)
	}
	if searchResult.Count == 0 {
		t.Error("search returned no results")
	}
	found := false
	for _, result := range searchResult.Results {
		if result.Hash == stored.Hash {
			found = true
			break
		}
	}
	if !found {
		t.Error("searched file not found in results")
	}

	// Step 6: Retrieve file by hash
	t.Log("Step 6: Retrieving file by hash")
	fileResult, err := service.GetFileByHash(stored.Hash)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}
	defer fileResult.Reader.Close()

	// Read file content
	content := make([]byte, fileResult.Size)
	_, err = fileResult.Reader.Read(content)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "end-to-end test content" {
		t.Errorf("file content mismatch: got %s, want end-to-end test content", string(content))
	}

	t.Log("End-to-end test completed successfully")
}

// TestEndToEnd_BatchOperations tests batch metadata updates.
func TestEndToEnd_BatchOperations(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store multiple files
	var hashes []string
	for i := 0; i < 5; i++ {
		storeReq := FileStoreRequest{
			Reader:   bytes.NewReader([]byte("batch test content " + string(rune(i)))),
			Filename: "batch_test.txt",
			MimeType: "text/plain",
			Size:     20,
		}
		stored, err := service.StoreFile(storeReq)
		if err != nil {
			t.Fatalf("failed to store file %d: %v", i, err)
		}
		hashes = append(hashes, stored.Hash)
	}

	// Batch update metadata
	updates := make([]MetadataUpdateRequest, len(hashes))
	for i, hash := range hashes {
		updates[i] = MetadataUpdateRequest{
			Hash:     hash,
			Action:   "merge",
			Metadata: map[string]string{"batch_id": "123", "index": string(rune(i))},
		}
	}

	batchReq := BatchMetadataUpdateRequest{Updates: updates}
	batchResult, err := service.BatchUpdateFileMetadata(batchReq)
	if err != nil {
		t.Fatalf("failed to batch update: %v", err)
	}

	if batchResult.Total != len(hashes) {
		t.Errorf("total mismatch: got %d, want %d", batchResult.Total, len(hashes))
	}
	if batchResult.SuccessCount != len(hashes) {
		t.Errorf("success count mismatch: got %d, want %d", batchResult.SuccessCount, len(hashes))
	}
	if batchResult.FailureCount != 0 {
		t.Errorf("failure count mismatch: got %d, want 0", batchResult.FailureCount)
	}

	// Verify updates
	for i, item := range batchResult.Results {
		if !item.Success {
			t.Errorf("item %d failed: %s", i, item.Error)
		}
		if item.NewMetadata["batch_id"] != "123" {
			t.Errorf("item %d metadata mismatch", i)
		}
	}
}

// TestEndToEnd_ErrorHandling tests error handling throughout the service layer.
func TestEndToEnd_ErrorHandling(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Test error handling for non-existent file
	_, err := service.GetFileByHash("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent hash")
	}
	if !errors.Is(err, storage.ErrFileNotFound) {
		// Error should be wrapped but still checkable
		if !strings.Contains(err.Error(), "retrieval error") {
			t.Errorf("error should be wrapped: %v", err)
		}
	}

	// Test error handling for invalid rename
	renameReq := FileRenameRequest{
		Hash:    "nonexistent",
		NewName: "test.txt",
	}
	_, err = service.RenameFile(renameReq)
	if err == nil {
		t.Error("expected error for non-existent file rename")
	}

	// Test error handling for invalid delete
	deleteReq := FileDeleteRequest{Hash: "nonexistent"}
	_, err = service.DeleteFile(deleteReq)
	if err == nil {
		t.Error("expected error for non-existent file delete")
	}
}

// TestEndToEnd_ResponseTransformation tests that responses are properly transformed.
func TestEndToEnd_ResponseTransformation(t *testing.T) {
	store, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store file
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("transformation test")),
		Filename: "transform_test.txt",
		MimeType: "text/plain",
		Size:     19,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		t.Fatalf("failed to store file: %v", err)
	}

	// Verify response structure matches DTO
	jsonData, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var response FileStoreResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify all expected fields are present
	if response.Hash == "" {
		t.Error("hash field missing")
	}
	if response.OriginalName == "" {
		t.Error("original_name field missing")
	}
	if response.StoredPath == "" {
		t.Error("stored_path field missing")
	}
	if response.UploadedAt.IsZero() {
		t.Error("uploaded_at field missing or zero")
	}
}

// BenchmarkFileService_StoreFile benchmarks file storage operations.
func BenchmarkFileService_StoreFile(b *testing.B) {
	store, tmpDir := setupTestStorage(&testing.T{})
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	content := bytes.NewReader([]byte("benchmark test content"))
	req := FileStoreRequest{
		Reader:   content,
		Filename: "benchmark.txt",
		MimeType: "text/plain",
		Size:     22,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content.Seek(0, 0)
		_, err := service.StoreFile(req)
		if err != nil {
			b.Fatalf("failed to store file: %v", err)
		}
	}
}

// BenchmarkFileService_GetFileMetadata benchmarks metadata retrieval.
func BenchmarkFileService_GetFileMetadata(b *testing.B) {
	store, tmpDir := setupTestStorage(&testing.T{})
	defer cleanupTestStorage(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	service := NewFileService(store, logger)

	// Store a file first
	storeReq := FileStoreRequest{
		Reader:   bytes.NewReader([]byte("benchmark content")),
		Filename: "benchmark.txt",
		MimeType: "text/plain",
		Size:     17,
	}
	stored, err := service.StoreFile(storeReq)
	if err != nil {
		b.Fatalf("failed to store file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetFileMetadata(stored.Hash)
		if err != nil {
			b.Fatalf("failed to get metadata: %v", err)
		}
	}
}

